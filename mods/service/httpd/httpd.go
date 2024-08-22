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
	"github.com/machbase/neo-client/machrpc"
	"github.com/machbase/neo-server/api"
	"github.com/machbase/neo-server/api/bridge"
	"github.com/machbase/neo-server/api/mgmt"
	"github.com/machbase/neo-server/api/schedule"
	"github.com/machbase/neo-server/mods/logging"
	"github.com/machbase/neo-server/mods/model"
	"github.com/machbase/neo-server/mods/pkgs"
	"github.com/machbase/neo-server/mods/service/backupd"
	"github.com/machbase/neo-server/mods/service/internal/ginutil"
	"github.com/machbase/neo-server/mods/service/internal/netutil"
	"github.com/machbase/neo-server/mods/service/security"
	"github.com/machbase/neo-server/mods/tql"
	"github.com/machbase/neo-server/mods/util/ssfs"
	"github.com/pkg/errors"
)

type Service interface {
	Start() error
	Stop()
}

// Factory
func New(db api.Database, options ...Option) (Service, error) {
	s := &httpd{
		log:      logging.GetLog("httpd"),
		db:       db,
		jwtCache: security.NewJwtCache(),
		handlers: []*HandlerConfig{
			{Prefix: "/db", Handler: "machbase"},
			{Prefix: "/lakes", Handler: "lakes"},
			{Prefix: "/metrics", Handler: "influx"},
			{Prefix: "/web", Handler: "web"},
		},
		neoShellAccount: make(map[string]string),
	}
	for _, opt := range options {
		opt(s)
	}
	return s, nil
}

type httpd struct {
	log   logging.Log
	db    api.Database
	alive bool

	listenAddresses []string
	enableTokenAuth bool
	handlers        []*HandlerConfig
	disableWeb      bool

	serverInfoFunc     func() (*machrpc.ServerInfo, error)
	serverSessionsFunc func(statz, session bool) (*machrpc.Statz, []*machrpc.Session, error)
	mqttInfoFunc       func() map[string]any
	mqttWsHandler      func(*gin.Context)

	httpServer        *http.Server
	listeners         []net.Listener
	jwtCache          security.JwtCache
	authServer        security.AuthServer
	backupService     backupd.Service
	mgmtImpl          mgmt.ManagementServer
	schedMgmtImpl     schedule.ManagementServer
	bridgeMgmtImpl    bridge.ManagementServer
	bridgeRuntimeImpl bridge.RuntimeServer
	pkgMgr            *pkgs.PkgManager

	neoShellAddress string
	neoShellAccount map[string]string

	tqlLoader tql.Loader
	serverFs  *ssfs.SSFS

	licenseFilePath        string
	debugMode              bool
	webShellProvider       model.ShellProvider
	experimentModeProvider func() bool
	uiContentFs            http.FileSystem

	memoryFs *MemoryFS

	statzAllowed []string
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

	if svr.memoryFs != nil {
		svr.memoryFs.Stop()
	}
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
			if svr.enableTokenAuth && svr.authServer != nil {
				group.Use(svr.handleAuthToken)
			}
			group.POST("/:oper", svr.handleLineProtocol)
			svr.log.Infof("HTTP path %s for the line protocol", prefix)
		case HandlerWeb: // web ui
			if svr.disableWeb {
				continue
			}
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
			group.GET("/api/console/:console_id/data", svr.handleConsoleData)
			if svr.mqttWsHandler != nil {
				group.GET("/api/mqtt", svr.mqttWsHandler)
				svr.log.Infof("MQTT websocket handler enabled")
			}
			if svr.tqlLoader != nil {
				svr.memoryFs = &MemoryFS{Prefix: "/web/api/tql-assets/"}
				go svr.memoryFs.Start()
				svr.tqlLoader.SetVolatileAssetsProvider(svr.memoryFs)
				group.GET("/api/tql-assets/*path", gin.WrapH(http.FileServer(svr.memoryFs)))
			}
			if svr.pkgMgr != nil {
				svr.pkgMgr.HttpAppRouter(group, svr.handleTagQL)
			}
			group.Use(svr.handleJwtToken)
			if svr.pkgMgr != nil {
				svr.pkgMgr.HttpPkgRouter(group.Group("/api/pkgs"))
			}
			group.POST("/api/term/:term_id/windowsize", svr.handleTermWindowSize)
			group.GET("/api/tql/*path", svr.handleTagQL)
			group.POST("/api/tql/*path", svr.handleTagQL)
			group.POST("/api/tql", svr.handlePostTagQL)
			group.POST("/api/md", svr.handleMarkdown)
			group.Any("/machbase", svr.handleQuery) // TODO depcreated, use /web/api/query
			group.Any("/api/query", svr.handleQuery)
			group.GET("/api/check", svr.handleCheck)
			group.POST("/api/relogin", svr.handleReLogin)
			group.POST("/api/logout", svr.handleLogout)
			group.GET("/api/shell/:id", svr.handleGetShell)
			group.GET("/api/shell/:id/copy", svr.handleGetShellCopy)
			group.POST("/api/shell/:id", svr.handlePostShell)
			group.DELETE("/api/shell/:id", svr.handleDeleteShell)
			group.GET("/api/keys", svr.handleKeys)
			group.POST("/api/keys", svr.handleKeysGen)
			group.DELETE("/api/keys/:id", svr.handleKeysDel)
			group.GET("/api/timers/:name", svr.handleTimer)
			group.GET("/api/timers", svr.handleTimers)
			group.POST("/api/timers", svr.handleTimersAdd)
			group.POST("/api/timers/:name/state", svr.handleTimersState)
			group.PUT("/api/timers/:name", svr.handleTimersUpdate)
			group.DELETE("/api/timers/:name", svr.handleTimersDel)
			group.GET("/api/bridges", svr.handleBridges)
			group.POST("/api/bridges", svr.handleBridgesAdd)
			group.POST("/api/bridges/:name/state", svr.handleBridgeState)
			group.DELETE("/api/bridges/:name", svr.handleBridgesDel)
			group.GET("/api/subscribers/:name", svr.handleSubscriber)
			group.GET("/api/subscribers", svr.handleSubscribers)
			group.POST("/api/subscribers", svr.handleSubscribersAdd)
			group.POST("/api/subscribers/:name/state", svr.handleSubscribersState)
			group.DELETE("/api/subscribers/:name", svr.handleSubscribersDel)
			group.GET("/api/sshkeys", svr.handleSshKeys)
			group.POST("/api/sshkeys", svr.handleSshKeysAdd)
			group.DELETE("/api/sshkeys/:fingerprint", svr.handleSshKeysDel)
			group.GET("/api/tables", svr.handleTables)
			group.GET("/api/tables/:table/tags", svr.handleTags)
			group.GET("/api/tables/:table/tags/:tag/stat", svr.handleTagStat)
			group.Any("/api/files/*path", svr.handleFiles)
			group.GET("/api/refs/*path", svr.handleRefs)
			group.GET("/api/license", svr.handleGetLicense)
			group.POST("/api/license", svr.handleInstallLicense)
			if svr.backupService != nil {
				backupdGroup := group.Group("/api/backup")
				svr.backupService.HttpRouter(backupdGroup)
			}
			svr.log.Infof("HTTP path %s for the web ui", prefix)
		case HandlerLake:
			group.GET("/tags", svr.handleLakeGetTagList)
			group.GET("/values/:type", svr.handleLakeGetValues)
			group.POST("/values", svr.handleLakePostValues)
			group.POST("/values/:type", svr.handleLakePostValues)
			group.POST("/inter/execquery", svr.handleLakeExecQuery)
			svr.log.Infof("HTTP path %s for lake api", prefix)
		case HandlerMachbase: // "machbase"
			if svr.enableTokenAuth && svr.authServer != nil {
				group.Use(svr.handleAuthToken)
			}
			group.GET("/query", svr.handleQuery)
			group.POST("/query", svr.handleQuery)
			group.POST("/write", svr.handleWrite)
			group.POST("/write/:table", svr.handleWrite)
			group.GET("/tql/*path", svr.handleTagQL)
			group.POST("/tql/*path", svr.handleTagQL)
			group.POST("/tql", svr.handlePostTagQL)
			group.GET("/statz", svr.handleStatz)
			svr.log.Infof("HTTP path %s for machbase api", prefix)
		}
	}

	r.NoRoute(gin.WrapH(http.FileServer(AssetsDir())))
	return r
}

// for the internal processor
func (svr *httpd) getTrustConnection(ctx *gin.Context) (api.Conn, error) {
	// TODO handle API Token
	return svr.db.Connect(ctx, api.WithTrustUser("sys"))
}

// for the api called from web-client that authorized by JWT
func (svr *httpd) getUserConnection(ctx *gin.Context) (api.Conn, error) {
	claim, _ := svr.getJwtClaim(ctx)
	if claim != nil {
		return svr.db.Connect(ctx, api.WithTrustUser(claim.Subject))
	} else {
		return nil, errors.New("unathorized db request")
	}
}

func (svr *httpd) handleJwtToken(ctx *gin.Context) {
	auth, exist := ctx.Request.Header["Authorization"]
	if !exist {
		ctx.AsciiJSON(http.StatusUnauthorized, map[string]any{"success": false, "reason": "missing authorization header"})
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
				ctx.AsciiJSON(http.StatusUnauthorized, map[string]any{"success": false, "reason": err.Error()})
				ctx.Abort()
				return
			}
		}
		if claim == nil {
			continue
		}
		found = true
		break
	}
	if found {
		ctx.Set("jwt-claim", claim)
	} else {
		ctx.AsciiJSON(http.StatusUnauthorized, map[string]any{"success": false, "reason": "user not found or wrong password"})
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
		tok := ctx.Query("token")
		if tok != "" {
			result, err := svr.authServer.ValidateClientToken(tok)
			if err == nil && result {
				return
			}
		}
		ctx.JSON(http.StatusUnauthorized, map[string]any{"success": false, "reason": "missing authorization token"})
		ctx.Abort()
		return
	}
	found := false
	for _, h := range auth {
		if !strings.HasPrefix(strings.ToUpper(h), "BEARER ") {
			continue
		}
		tok := h[7:]
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
		AllowMethods:    []string{http.MethodGet, http.MethodHead, http.MethodOptions},
		AllowHeaders:    []string{"Origin", "Accept", "Content-Type"},
		ExposeHeaders:   []string{"Content-Length"},
		MaxAge:          12 * time.Hour,
	})
	return corsHandler
}

func (svr *httpd) allowStatz(remote string) bool {
	if remote == "127.0.0.1" {
		return true
	}
	for _, p := range svr.statzAllowed {
		if p == remote {
			return true
		}
	}
	return false
}

func (svr *httpd) handleStatz(ctx *gin.Context) {
	remote := ctx.RemoteIP()
	if !svr.allowStatz(remote) {
		ctx.String(http.StatusForbidden, "")
		return
	}
	if svr.serverInfoFunc == nil {
		ctx.String(http.StatusServiceUnavailable, "")
		return
	}
	stat, err := svr.serverInfoFunc()
	if err != nil {
		ctx.String(http.StatusInternalServerError, err.Error())
		return
	}

	ret := map[string]any{}
	ret["neo"] = map[string]any{
		"mem":         stat.Runtime.Mem,
		"volatile_fs": svr.memoryFs.Statz(),
	}

	if svr.serverSessionsFunc != nil {
		statz, _, _ := svr.serverSessionsFunc(true, false)
		ret["sess"] = map[string]any{
			"conns":          statz.ConnsInUse,
			"conns_used":     statz.Conns,
			"stmts":          statz.StmtsInUse,
			"stmts_used":     statz.Stmts,
			"appenders":      statz.AppendersInUse,
			"appenders_used": statz.Appenders,
			"raw_conns":      statz.RawConns,
		}
	}
	if svr.mqttInfoFunc != nil {
		if statz := svr.mqttInfoFunc(); statz != nil {
			ret["mqtt"] = statz
		}
	}

	ctx.JSON(http.StatusOK, ret)
}
