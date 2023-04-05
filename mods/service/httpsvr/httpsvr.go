package httpsvr

import (
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/machbase/neo-server/mods/logging"
	"github.com/machbase/neo-server/mods/service/httpsvr/assets"
	"github.com/machbase/neo-server/mods/service/security"
	spi "github.com/machbase/neo-spi"
)

func New(db spi.Database, conf *Config) (*Server, error) {
	return &Server{
		conf:     conf,
		log:      logging.GetLog("httpsvr"),
		db:       db,
		jwtCache: security.NewJwtCache(),
	}, nil
}

type Config struct {
	Handlers []HandlerConfig
}

type HandlerConfig struct {
	Prefix  string
	Handler string
}

type Server struct {
	conf *Config
	log  logging.Log
	db   spi.Database

	jwtCache   security.JwtCache
	authServer security.AuthServer // injection point
}

func (svr *Server) Start() error {
	return nil
}

func (svr *Server) Stop() {
}

func (svr *Server) SetAuthServer(authServer security.AuthServer) {
	svr.authServer = authServer
}

func (svr *Server) Route(r *gin.Engine) {
	r.Use(svr.corsHandler())
	for _, h := range svr.conf.Handlers {
		prefix := h.Prefix
		// remove trailing slash
		prefix = strings.TrimSuffix(prefix, "/")

		if h.Handler == "-" {
			// disabled by configuration
			continue
		}
		svr.log.Debugf("Add handler %s '%s'", h.Handler, prefix)
		group := r.Group(prefix)

		switch h.Handler {
		case "influx": // "influx line protocol"
			if svr.authServer != nil {
				group.Use(svr.handleAuthToken)
			}
			group.POST("/:oper", svr.handleLineProtocol)
			svr.log.Infof("HTTP path %s for the line protocol", prefix)
		case "web": // web ui
			contentBase := "/ui/"
			group.GET("/", func(ctx *gin.Context) {
				ctx.Redirect(http.StatusFound, path.Join(prefix, contentBase))
			})
			group.StaticFS(contentBase, GetAssets(contentBase))
			group.POST("/api/login", svr.handleLogin)
			group.Use(svr.handleJwtToken)
			group.POST("/api/relogin", svr.handleReLogin)
			group.POST("/api/logout", svr.handleLogout)
			group.Any("/machbase", svr.handleQuery)
			svr.log.Infof("HTTP path %s for the web ui", prefix)
		default: // "machbase"
			if svr.authServer != nil {
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
	// handle root /favicon.ico
	r.NoRoute(gin.WrapF(assets.Handler))
}

func (svr *Server) handleJwtToken(ctx *gin.Context) {
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
		claim, err := svr.verifyAccessToken(tok)
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
	if !found {
		ctx.JSON(http.StatusUnauthorized, map[string]any{"success": false, "reason": "user not found or wrong password"})
		ctx.Abort()
		return
	}
}

func (svr *Server) handleAuthToken(ctx *gin.Context) {
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

func (svr *Server) corsHandler() gin.HandlerFunc {
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
