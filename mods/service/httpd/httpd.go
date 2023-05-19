package httpd

import (
	"context"
	"net"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/machbase/neo-server/docs"
	"github.com/machbase/neo-server/mods/logging"
	"github.com/machbase/neo-server/mods/service/httpd/assets"
	"github.com/machbase/neo-server/mods/service/internal/ginutil"
	"github.com/machbase/neo-server/mods/service/internal/netutil"
	"github.com/machbase/neo-server/mods/service/security"
	spi "github.com/machbase/neo-spi"
	"github.com/pkg/errors"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

type Service interface {
	Start() error
	Stop()
}

type Option func(s *httpd)

// Factory
func New(db spi.Database, options ...Option) (Service, error) {
	s := &httpd{
		log:      logging.GetLog("httpd"),
		db:       db,
		jwtCache: security.NewJwtCache(),
	}
	for _, opt := range options {
		opt(s)
	}
	return s, nil
}

// ListenAddresses
func OptionListenAddress(addrs ...string) Option {
	return func(s *httpd) {
		s.listenAddresses = append(s.listenAddresses, addrs...)
	}
}

// AuthServer
func OptionAuthServer(authSvc security.AuthServer, enabled bool) Option {
	return func(s *httpd) {
		s.authServer = authSvc
		s.enableTokenAUth = enabled
		if enabled {
			s.log.Infof("HTTP token authentication enabled")
		} else {
			s.log.Infof("HTTP token authentication disabled")
		}
	}
}

// neo-shell address
func OptionNeoShellAddress(addrs ...string) Option {
	return func(s *httpd) {
		for _, addr := range addrs {
			if s.neoShellAddress == "" {
				s.neoShellAddress = strings.TrimPrefix(addr, "tcp://")
			} else if strings.HasPrefix(s.neoShellAddress, "127.0.0.1:") || strings.HasPrefix(s.neoShellAddress, "localhost:") {
				s.neoShellAddress = strings.TrimPrefix(addr, "tcp://")
			}
		}
	}
}

// Handler
func OptionHandler(prefix string, handler HandlerType) Option {
	return func(s *httpd) {
		s.handlers = append(s.handlers, &HandlerConfig{Prefix: prefix, Handler: handler})
	}
}

func OptionDebugMode() Option {
	return func(s *httpd) {
		s.debugMode = true
	}
}

func OptionReleaseMode() Option {
	return func(s *httpd) {
		s.debugMode = false
	}
}

type httpd struct {
	log   logging.Log
	db    spi.Database
	alive bool

	listenAddresses []string
	enableTokenAUth bool
	handlers        []*HandlerConfig

	httpServer *http.Server
	listeners  []net.Listener
	jwtCache   security.JwtCache
	authServer security.AuthServer

	neoShellAddress string

	debugMode bool
}

type HandlerType string

const (
	HandlerMachbase = HandlerType("machbase")
	HandlerInflux   = HandlerType("influx") // influx line protocol
	HandlerWeb      = HandlerType("web")    // web ui
	HandlerLake     = HandlerType("lakes")
	HandlerSwagger  = HandlerType("swagger")
	HandlerVoid     = HandlerType("-")
)

type HandlerConfig struct {
	Prefix  string
	Handler HandlerType
}

func (svr *httpd) Start() error {
	if svr.db == nil {
		return errors.New("no database instance")
	}

	svr.alive = true

	if svr.debugMode {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	svr.httpServer = &http.Server{}
	svr.httpServer.Handler = svr.Router()

	for _, listen := range svr.listenAddresses {
		lsnr, err := netutil.MakeListener(listen)
		if err != nil {
			return errors.Wrap(err, "cannot start with failed listener")
		}
		svr.listeners = append(svr.listeners, lsnr)
		go svr.httpServer.Serve(lsnr)
		svr.log.Infof("HTTP Listen %s", listen)
	}
	return nil
}

func (svr *httpd) Stop() {
	if svr.httpServer == nil {
		return
	}
	ctx, cancelFunc := context.WithTimeout(context.Background(), 3*time.Second)
	svr.httpServer.Shutdown(ctx)
	cancelFunc()
}

func (svr *httpd) Router() *gin.Engine {
	r := gin.New()
	r.Use(ginutil.RecoveryWithLogging(svr.log))
	r.Use(ginutil.HttpLogger("http-log"))
	r.Use(svr.corsHandler())

	enableSwagger := false
	prefixSwagger := "/swagger"

	// redirect '/' -> '/web/'
	for _, h := range svr.handlers {
		if h.Handler == HandlerWeb {
			r.GET("/", func(ctx *gin.Context) {
				ctx.Redirect(http.StatusFound, h.Prefix)
			})
			break
		}
	}

	for _, h := range svr.handlers {
		prefix := h.Prefix
		// remove trailing slash
		prefix = strings.TrimSuffix(prefix, "/")

		if h.Handler == HandlerVoid {
			// disabled by configuration
			continue
		}
		svr.log.Debugf("Add handler %s '%s'", h.Handler, prefix)
		group := r.Group(prefix)

		switch h.Handler {
		case HandlerInflux: // "influx line protocol"
			if svr.enableTokenAUth && svr.authServer != nil {
				group.Use(svr.handleAuthToken)
			}
			group.POST("/:oper", svr.handleLineProtocol)
			svr.log.Infof("HTTP path %s for the line protocol", prefix)
		case HandlerSwagger: // swagger ui
			enableSwagger = true
			prefixSwagger = prefix
		case HandlerWeb: // web ui
			contentBase := "/ui/"
			group.GET("/", func(ctx *gin.Context) {
				ctx.Redirect(http.StatusFound, path.Join(prefix, contentBase))
			})
			group.StaticFS(contentBase, GetAssets(contentBase))
			group.POST("/api/login", svr.handleLogin)
			group.Use(svr.handleJwtToken)
			group.POST("/api/relogin", svr.handleReLogin)
			group.POST("/api/logout", svr.handleLogout)
			group.GET("/api/term/:term_id/data", svr.handleTermData)
			group.POST("/api/term/:term_id/windowsize", svr.handleTermWindowSize)
			group.Any("/machbase", svr.handleQuery)
			svr.log.Infof("HTTP path %s for the web ui", prefix)
		case HandlerLake:
			group.GET("/tags", svr.handleLakeGetTags)
			group.GET("/logs", svr.handleLakeGetLogs)
			group.GET("/values", svr.handleLakeGetValues)
			group.POST("/values", svr.handleLakePostValues)
			svr.log.Infof("HTTP path %s for lake api", prefix)
		case HandlerMachbase: // "machbase"
			if svr.enableTokenAUth && svr.authServer != nil {
				group.Use(svr.handleAuthToken)
			}
			group.GET("/query", svr.handleQuery)
			group.POST("/query", svr.handleQuery)
			group.GET("/chart", svr.handleChart)
			group.POST("/chart", svr.handleChart)
			group.POST("/write", svr.handleWrite)
			group.POST("/write/:table", svr.handleWrite)
			svr.log.Infof("HTTP path %s for machbase api", prefix)
		}
	}

	if enableSwagger {
		docs.SwaggerInfo.Title = "Swagger machbase-neo HTTP API"
		docs.SwaggerInfo.Version = "1.0"
		docs.SwaggerInfo.Description = "machbase-neo http server"
		docs.SwaggerInfo.BasePath = "/"
		docs.SwaggerInfo.Host = "localhost:5654"
		docs.SwaggerInfo.Schemes = []string{"http"}
		r.GET(prefixSwagger+"/*{any}", ginSwagger.WrapHandler(swaggerFiles.Handler))
		svr.log.Infof("HTTP path %s/index.html for swagger", prefixSwagger)
	}

	// handle root /favicon.ico
	r.NoRoute(gin.WrapF(assets.Handler))
	return r
}

func (svr *httpd) handleJwtToken(ctx *gin.Context) {
	auth, exist := ctx.Request.Header["Authorization"]
	if !exist {
		ctx.JSON(http.StatusUnauthorized, map[string]any{"success": false, "reason": "missing authorization header"})
		ctx.Abort()
		return
	}
	var claim security.Claim
	var err error
	var found = false
	for _, h := range auth {
		if !strings.HasPrefix(strings.ToUpper(h), "BEARER ") {
			continue
		}
		tok := h[7:]
		claim, err = svr.verifyAccessToken(tok)
		if err != nil {
			if IsErrTokenExpired(err) && strings.HasSuffix(ctx.Request.URL.Path, "/api/relogin") {
				// jwt has been expired, but the request is for 'relogin'
				found = true
				break
			} else {
				ctx.JSON(http.StatusUnauthorized, map[string]any{"success": false, "reason": err.Error()})
				ctx.Abort()
				return
			}
		}
		if claim == nil {
			continue
		}
		if err == nil && claim != nil {
			found = true
			break
		}
	}
	if found {
		ctx.Set("jwt-claim", claim)
	} else {
		ctx.JSON(http.StatusUnauthorized, map[string]any{"success": false, "reason": "user not found or wrong password"})
		ctx.Abort()
		return
	}
}

func (svr *httpd) handleAuthToken(ctx *gin.Context) {
	if svr.authServer == nil {
		ctx.JSON(http.StatusUnauthorized, map[string]any{"success": false, "reason": "no auth server"})
		ctx.Abort()
		return
	}
	auth, exist := ctx.Request.Header["Authorization"]
	if !exist {
		ctx.JSON(http.StatusUnauthorized, map[string]any{"success": false, "reason": "missing authorization header"})
		ctx.Abort()
		return
	}
	found := false
	for _, h := range auth {
		if !strings.HasPrefix(strings.ToUpper(h), "BEARER ") {
			continue
		}
		tok := h[7:]
		svr.log.Infof("tok ==>", tok)
		result, err := svr.authServer.ValidateClientToken(tok)
		if err != nil {
			svr.log.Errorf("client private key %s", err.Error())
		}
		if result {
			found = true
			break
		}
	}
	if !found {
		ctx.JSON(http.StatusUnauthorized, map[string]any{"success": false, "reason": "missing valid token"})
		ctx.Abort()
		return
	}
}

func (svr *httpd) corsHandler() gin.HandlerFunc {
	corsHandler := cors.New(cors.Config{
		AllowOrigins: []string{"*"},
		AllowMethods: []string{
			http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete,
			http.MethodPatch, http.MethodHead, http.MethodOptions},
		AllowHeaders: []string{
			"Origin", "Access-Control-Allow-Origin",
			"Authorization", "Access-Control-Allow-Headers",
			"Access-Control-Max-Age",
			"X-Requested-With", "Accept",
			"Content-Type", "Content-Length",
			"Use-Timezone",
		},
		ExposeHeaders: []string{
			"Cache-Control", "Content-Length", "Content-Language",
			"Content-Type", "Expires", "Last-Modified", "pragma",
		},
		AllowCredentials: true,
		AllowWebSockets:  true,
		MaxAge:           12 * time.Hour,
	})
	return corsHandler
}
