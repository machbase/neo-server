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
	"github.com/golang-jwt/jwt/v4"
	mach "github.com/machbase/neo-engine"
	"github.com/machbase/neo-server/mods/do"
	"github.com/machbase/neo-server/mods/logging"
	"github.com/machbase/neo-server/mods/model"
	"github.com/machbase/neo-server/mods/service/httpd/assets"
	"github.com/machbase/neo-server/mods/service/internal/ginutil"
	"github.com/machbase/neo-server/mods/service/internal/netutil"
	"github.com/machbase/neo-server/mods/service/security"
	"github.com/machbase/neo-server/mods/tql"
	"github.com/machbase/neo-server/mods/util/ssfs"
	spi "github.com/machbase/neo-spi"
	"github.com/pkg/errors"
)

type Service interface {
	Start() error
	Stop()
}

// Factory
func New(db spi.Database, options ...Option) (Service, error) {
	s := &httpd{
		log:      logging.GetLog("httpd"),
		db:       db,
		jwtCache: security.NewJwtCache(),

		neoShellAccount: make(map[string]string),
	}
	for _, opt := range options {
		opt(s)
	}
	return s, nil
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
	neoShellAccount map[string]string

	tqlLoader tql.Loader
	serverFs  *ssfs.SSFS

	licenseFilePath        string
	debugMode              bool
	webShellProvider       model.ShellProvider
	experimentModeProvider func() bool
	uiContentFs            http.FileSystem

	lake LakeAppender // ?
}

type HandlerType string

const (
	HandlerMachbase = HandlerType("machbase")
	HandlerInflux   = HandlerType("influx") // influx line protocol
	HandlerWeb      = HandlerType("web")    // web ui
	HandlerLake     = HandlerType("lakes")
	HandlerVoid     = HandlerType("-")
)

type HandlerConfig struct {
	Prefix  string
	Handler HandlerType
}

// ?
type LakeAppender struct {
	appender spi.Appender
	conn     spi.Conn
	ctx      *gin.Context
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

	//?
	svr.LoadAppender()

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

// ?
func (svr *httpd) LoadAppender() {
	var err error
	svr.lake.ctx = &gin.Context{}
	svr.lake.conn, err = svr.getTrustConnection(svr.lake.ctx)
	if err != nil {
		svr.log.Error("connection failed.")
		return
	}
	//Stop 에서 close?

	exist, err := do.ExistsTable(svr.lake.ctx, svr.lake.conn, "TAG")
	if err != nil {
		svr.log.Error("exist table error: ", err)
		return
	}
	if !exist {
		svr.log.Error("not exist 'TAG' table")
		return
	}

	svr.lake.appender, err = svr.lake.conn.Appender(svr.lake.ctx, "TAG")
	if err != nil {
		svr.log.Error("appender error: ", err)
		return
	}
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
	if svr.debugMode {
		r.Use(ginutil.HttpLogger("http-log"))
	}
	r.Use(svr.corsHandler())

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
		case HandlerWeb: // web ui
			contentBase := "/ui/"
			group.GET("/", func(ctx *gin.Context) {
				ctx.Redirect(http.StatusFound, path.Join(prefix, contentBase))
			})
			if svr.uiContentFs != nil {
				group.StaticFS(contentBase, svr.uiContentFs)
			} else {
				group.StaticFS(contentBase, GetAssets(contentBase))
			}
			group.POST("/api/login", svr.handleLogin)
			group.GET("/api/term/:term_id/data", svr.handleTermData)
			group.POST("/api/term/:term_id/windowsize", svr.handleTermWindowSize)
			group.GET("/api/console/:console_id/data", svr.handleConsoleData)
			if svr.tqlLoader != nil {
				group.GET("/api/tql/*path", svr.handleTagQL)
				group.POST("/api/tql/*path", svr.handleTagQL)
			}
			group.Use(svr.handleJwtToken)
			if svr.tqlLoader != nil {
				group.POST("/api/tql", svr.handlePostTagQL)
			}
			group.POST("/api/md", svr.handleMarkdown)
			group.Any("/machbase", svr.handleQuery)
			group.GET("/api/check", svr.handleCheck)
			group.POST("/api/relogin", svr.handleReLogin)
			group.POST("/api/logout", svr.handleLogout)
			group.GET("/api/shell/:id", svr.handleGetShell)
			group.GET("/api/shell/:id/copy", svr.handleGetShellCopy)
			group.POST("/api/shell/:id", svr.handlePostShell)
			group.DELETE("/api/shell/:id", svr.handleDeleteShell)
			group.GET("/api/chart", svr.handleChart)
			group.POST("/api/chart", svr.handleChart)
			group.GET("/api/tables", svr.handleTables)
			group.GET("/api/tables/:table/tags", svr.handleTags)
			group.GET("/api/tables/:table/tags/:tag/stat", svr.handleTagStat)
			group.Any("/api/files/*path", svr.handleFiles)
			group.GET("/api/refs/*path", svr.handleRefs)
			group.GET("/api/license", svr.handleGetLicense)
			group.POST("/api/license", svr.handleInstallLicense)
			svr.log.Infof("HTTP path %s for the web ui", prefix)
		case HandlerLake:
			group.GET("/tags", svr.handleLakeGetTagList)
			group.GET("/logs", svr.handleLakeGetLogs)
			group.GET("/values/:type", svr.handleLakeGetValues)
			group.POST("/values", svr.handleLakePostValues)
			// group.POST("/execquery",svr.handleLakeExecQuery)
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
			if svr.tqlLoader != nil {
				group.GET("/tql/*path", svr.handleTagQL)
				group.POST("/tql/*path", svr.handleTagQL)
				group.POST("/tql", svr.handlePostTagQL)
			}
			svr.log.Infof("HTTP path %s for machbase api", prefix)
		}
	}

	// handle /web/echarts/*
	r.GET("/web/echarts/*path", gin.WrapH(http.FileServer(assets.EchartsDir())))
	r.GET("/web/tutorials/*path", gin.WrapH(http.FileServer(assets.TutorialsDir())))
	// handle root /favicon.ico
	r.NoRoute(gin.WrapF(assets.Handler))
	return r
}

// for the internal processor
func (svr *httpd) getTrustConnection(ctx *gin.Context) (spi.Conn, error) {
	// TODO handle API Token
	return svr.db.Connect(ctx, mach.WithTrustUser("sys"))
}

// for the api called from web-client that authorized by JWT
func (svr *httpd) getUserConnection(ctx *gin.Context) (spi.Conn, error) {
	claim, _ := svr.getJwtClaim(ctx)
	if claim != nil {
		return svr.db.Connect(ctx, mach.WithTrustUser(claim.Subject))
	} else {
		return nil, errors.New("unathorized db request")
	}
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

func (svr *httpd) getJwtClaim(ctx *gin.Context) (security.Claim, bool) {
	obj, ok := ctx.Get("jwt-claim")
	if !ok {
		return nil, false
	}

	if token, ok := obj.(*jwt.RegisteredClaims); !ok {
		return nil, false
	} else {
		return token, ok
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
		AllowAllOrigins: true,
		//AllowOrigins:    []string{"*"},
		AllowMethods:  []string{http.MethodGet, http.MethodHead, http.MethodOptions},
		AllowHeaders:  []string{"Origin", "Accept", "Content-Type"},
		ExposeHeaders: []string{"Content-Length"},
		MaxAge:        12 * time.Hour,
	})
	return corsHandler
}
