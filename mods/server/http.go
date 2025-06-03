package server

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	httpPprof "net/http/pprof"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v4"
	"github.com/gorilla/websocket"
	"github.com/machbase/neo-server/v8/api"
	"github.com/machbase/neo-server/v8/api/bridge"
	"github.com/machbase/neo-server/v8/api/mgmt"
	"github.com/machbase/neo-server/v8/api/schedule"
	"github.com/machbase/neo-server/v8/mods"
	"github.com/machbase/neo-server/v8/mods/eventbus"
	"github.com/machbase/neo-server/v8/mods/jsh"
	jshHttp "github.com/machbase/neo-server/v8/mods/jsh/http"
	"github.com/machbase/neo-server/v8/mods/logging"
	"github.com/machbase/neo-server/v8/mods/model"
	"github.com/machbase/neo-server/v8/mods/pkgs"
	"github.com/machbase/neo-server/v8/mods/tql"
	"github.com/machbase/neo-server/v8/mods/util"
	"github.com/machbase/neo-server/v8/mods/util/mdconv"
	"github.com/machbase/neo-server/v8/mods/util/restclient"
	"github.com/machbase/neo-server/v8/mods/util/ssfs"
	cmap "github.com/orcaman/concurrent-map/v2"
	"golang.org/x/crypto/ssh"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

// Factory
func NewHttp(db api.Database, options ...HttpOption) (*httpd, error) {
	s := &httpd{
		log:      logging.GetLog("httpd"),
		db:       db,
		jwtCache: NewJwtCache(),
		handlers: []*HandlerConfig{
			{Prefix: "/db", Handler: "machbase"},
			{Prefix: "/lakes", Handler: "lakes"},
			{Prefix: "/metrics", Handler: "influx"},
			{Prefix: "/web", Handler: "web"},
		},
		neoShellAccount: make(map[string]string),
		pathMap:         map[string]string{},
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
	mqttWsHandler   func(*gin.Context)

	httpServer        *http.Server
	listeners         []net.Listener
	jwtCache          JwtCache
	authServer        AuthServer
	bakd              *backupd
	mgmtImpl          mgmt.ManagementServer
	schedMgmtImpl     schedule.ManagementServer
	bridgeMgmtImpl    bridge.ManagementServer
	bridgeRuntimeImpl bridge.RuntimeServer
	pkgMgr            *pkgs.PkgManager

	neoShellAddress string
	neoShellAccount map[string]string

	tqlLoader tql.Loader
	serverFs  *ssfs.SSFS

	eulaPassed             bool
	eulaFilePath           string
	licenseFilePath        string
	licenseStatusLastTime  time.Time
	licenseStatus          string
	debugMode              bool
	debugLogFilterLatency  time.Duration
	readBufSize            int
	writeBufSize           int
	linger                 int
	keepAlive              int
	webShellProvider       model.ShellProvider
	experimentModeProvider func() bool
	uiContentFs            http.FileSystem

	memoryFs *MemoryFS
	pathMap  map[string]string

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

	var connContext func(context.Context, net.Conn) context.Context
	if runtime.GOOS != "windows" {
		connContext = func(ctx context.Context, c net.Conn) context.Context {
			if tcpCon, ok := c.(*net.TCPConn); ok && tcpCon != nil {
				tcpCon.SetNoDelay(true)
				if svr.keepAlive > 0 {
					tcpCon.SetKeepAlive(true)
					tcpCon.SetKeepAlivePeriod(time.Duration(svr.keepAlive) * time.Second)
				}
				if svr.linger >= 0 {
					tcpCon.SetLinger(svr.linger)
				}
				if svr.readBufSize > 0 {
					tcpCon.SetReadBuffer(svr.readBufSize)
				}
				if svr.writeBufSize > 0 {
					tcpCon.SetWriteBuffer(svr.writeBufSize)
				}
			}
			return ctx
		}
	}
	svr.httpServer = &http.Server{
		ConnContext: connContext,
	}
	router := svr.Router()
	svr.httpServer.Handler = router
	jshHttp.SetDefaultRouter(router)

	for _, listen := range svr.listenAddresses {
		lsnr, err := util.MakeListener(listen)
		if err != nil {
			return fmt.Errorf("cannot start with failed listener, %s", err.Error())
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
	svr.log.Infof("gracefully stopping server")
	ctx, cancelFunc := context.WithTimeout(context.Background(), 3*time.Second)
	svr.httpServer.Shutdown(ctx)
	cancelFunc()
	svr.httpServer.Close()

	if svr.memoryFs != nil {
		svr.memoryFs.Stop()
	}
}

func (svr *httpd) AdvertiseAddress() string {
	for _, addr := range svr.listeners {
		if strAddr := addr.Addr().String(); strAddr == "" {
			continue
		} else {
			return "http://" + strings.TrimPrefix(strAddr, "tcp://")
		}
	}
	return ""
}

// DebugMode returns the current debug mode and the log filter latency
func (svr *httpd) DebugMode() (bool, time.Duration) {
	return svr.debugMode, svr.debugLogFilterLatency
}

// SetDebugMode sets the debug mode and the log filter latency
func (svr *httpd) SetDebugMode(debug bool, filterLatency time.Duration) {
	svr.debugMode = debug
	if filterLatency >= 0 {
		svr.debugLogFilterLatency = filterLatency
	}
}

func (svr *httpd) Router() *gin.Engine {
	r := gin.New()
	r.Use(RecoveryWithLogging(svr.log))
	r.Use(HttpLogger("http-log", &svr.debugMode, &svr.debugLogFilterLatency))
	r.Use(svr.corsHandler())
	r.Use(MetricsInterceptor())

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
			contentBase := "/ui/"
			group.GET("/", func(ctx *gin.Context) {
				ctx.Redirect(http.StatusFound, path.Join(prefix, contentBase))
			})
			if svr.uiContentFs != nil {
				group.StaticFS(contentBase, svr.uiContentFs)
			} else {
				group.StaticFS(contentBase, GetAssets(contentBase))
			}
			group.Any("/api/license/eula", svr.handleEula)
			group.POST("/api/login", svr.handleLogin)
			group.GET("/api/term/:term_id/data", svr.handleTermData)
			group.GET("/api/console/:console_id/data", svr.handleConsoleData)
			if svr.mqttWsHandler != nil {
				group.GET("/api/mqtt", svr.mqttWsHandler)
				svr.log.Infof("MQTT websocket handler enabled")
			}
			if svr.tqlLoader != nil {
				svr.memoryFs = NewMemoryFS("/web/api/tql-assets/")
				go svr.memoryFs.Start()
				svr.tqlLoader.SetVolatileAssetsProvider(svr.memoryFs)
				group.GET("/api/tql-assets/*path", gin.WrapH(http.FileServer(svr.memoryFs)))
			}
			if svr.pkgMgr != nil {
				svr.pkgMgr.HttpAppRouter(group, svr.handleTqlFile)
			}
			group.GET("/api/tql-exec", svr.handleTqlQueryExec)
			group.Use(svr.handleJwtToken)
			if svr.pkgMgr != nil {
				svr.pkgMgr.HttpPkgRouter(group.Group("/api/pkgs"))
			}
			group.POST("/api/term/:term_id/windowsize", svr.handleTermWindowSize)
			group.GET("/api/tql/*path", svr.handleTqlFile)
			group.POST("/api/tql/*path", svr.handleTqlFile)
			group.GET("/api/tql", svr.handleTqlQuery)
			group.POST("/api/tql", svr.handleTqlQuery)
			group.POST("/api/md", svr.handleMarkdown)
			group.Any("/machbase", func(c *gin.Context) {
				svr.log.Debugf("/web/api/machbase is deprecated, use /web/api/query")
				svr.handleQuery(c)
			})
			group.Any("/api/query", svr.handleQuery)
			group.GET("/api/check", svr.handleCheck)
			group.POST("/api/splitter/sql", svr.handleSplitSQL)
			group.POST("/api/splitter/http", svr.handleSplitHTTP)
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
			if svr.bakd != nil {
				backupdGroup := group.Group("/api/backup")
				svr.bakd.HttpRouter(backupdGroup)
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
			group.GET("/query/file/:table/:column/:id", svr.handleFileQuery)
			group.GET("/watch/:table", svr.handleWatchQuery)
			group.GET("/tql/*path", svr.handleTqlFile)
			group.POST("/tql/*path", svr.handleTqlFile)
			group.GET("/tql", svr.handleTqlQuery)
			group.POST("/tql", svr.handleTqlQuery)
			svr.log.Infof("HTTP path %s for machbase api", prefix)
		}
	}
	// debug group
	debugGroup := r.Group("/debug")
	debugGroup.Use(svr.allowDebug)
	debugGroup.Any("/pprof/*path", gin.WrapF(httpPprof.Index))
	debugGroup.GET("/statz", svr.handleStatz)

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
		return nil, errors.New("unauthorized db request")
	}
}

func (svr *httpd) handleJwtToken(ctx *gin.Context) {
	auth, exist := ctx.Request.Header["Authorization"]
	if !exist {
		if ctx.Request.RemoteAddr == "@" {
			// MEMO: why the remoteAddr is "@" on Windows?
			ctx.Request.RemoteAddr = ""
		}
		if ctx.Request.RemoteAddr == "" {
			// this request from localhost via unix socket.
			// allow it without jwt token
			return
		}
		ctx.AsciiJSON(http.StatusUnauthorized, map[string]any{"success": false, "reason": "missing authorization header"})
		ctx.Abort()
		return
	}
	var claim Claim
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

func (svr *httpd) getJwtClaim(ctx *gin.Context) (Claim, bool) {
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

func (svr *httpd) issueAccessToken(loginName string) (accessToken string, refreshToken string, refreshTokenId string, err error) {
	claim := NewClaim(loginName)
	accessToken, err = SignTokenWithClaim(claim)
	if err != nil {
		err = fmt.Errorf("signing at error, %s", err.Error())
		return
	}

	refreshClaim := NewClaimForRefresh(claim)
	refreshToken, err = SignTokenWithClaim(refreshClaim)
	if err != nil {
		err = fmt.Errorf("signing rt error, %s", err.Error())
		return
	}
	refreshTokenId = refreshClaim.ID
	return
}

func (svr *httpd) verifyAccessToken(token string) (Claim, error) {
	claim := NewClaimEmpty()
	ok, err := VerifyTokenWithClaim(token, claim)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, nil
	}
	return claim, nil
}

func IsErrTokenExpired(err error) bool {
	if jwtErr, ok := err.(*jwt.ValidationError); ok && jwtErr.Is(jwt.ErrTokenExpired) {
		return true
	}
	return false
}

type LoginReq struct {
	LoginName string `json:"loginName"`
	Password  string `json:"password"`
}

type LoginRsp struct {
	Success      bool        `json:"success"`
	AccessToken  string      `json:"accessToken"`
	RefreshToken string      `json:"refreshToken"`
	Reason       string      `json:"reason"`
	Elapse       string      `json:"elapse"`
	ServerInfo   *ServerInfo `json:"server,omitempty"`
}

type LoginCheckRsp struct {
	Success        bool                     `json:"success"`
	Reason         string                   `json:"reason"`
	Elapse         string                   `json:"elapse"`
	ServerInfo     *ServerInfo              `json:"server,omitempty"`
	ExperimentMode bool                     `json:"experimentMode"`
	EulaRequired   bool                     `json:"eulaRequired,omitempty"`
	LicenseStatus  string                   `json:"licenseStatus,omitempty"`
	Shells         []*model.ShellDefinition `json:"shells,omitempty"`
}

type ServerInfo struct {
	Version string `json:"version,omitempty"`
}

type WebReferenceGroup struct {
	Label string          `json:"label"`
	Items []ReferenceItem `json:"items"`
}

type ReferenceItem struct {
	Type   string `json:"type"`
	Title  string `json:"title"`
	Addr   string `json:"address"`
	Target string `json:"target,omitempty"`
}

func (svr *httpd) handleLogin(ctx *gin.Context) {
	var req = &LoginReq{}
	var rsp = &LoginRsp{
		Success: false,
		Reason:  "not specified",
	}

	tick := time.Now()

	err := ctx.Bind(req)
	if err != nil {
		rsp.Reason = err.Error()
		rsp.Elapse = time.Since(tick).String()
		ctx.JSON(http.StatusBadRequest, rsp)
		return
	}

	if len(req.LoginName) == 0 {
		rsp.Reason = "missing required loginName field"
		rsp.Elapse = time.Since(tick).String()
		ctx.JSON(http.StatusBadRequest, rsp)
		return
	}

	passed, reason, err := svr.db.UserAuth(ctx, req.LoginName, req.Password)
	if err != nil {
		svr.log.Warnf("database auth failed %s", err.Error())
		rsp.Reason = "database error for user authentication"
		rsp.Elapse = time.Since(tick).String()
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	}

	if !passed {
		svr.log.Tracef("'%s' login fail password mis-matched", req.LoginName)
		rsp.Reason = reason
		rsp.Elapse = time.Since(tick).String()
		ctx.JSON(http.StatusNotFound, rsp)
		return
	}

	accessToken, refreshToken, refreshTokenId, err := svr.issueAccessToken(req.LoginName)
	svr.log.Tracef("'%s' login success %s", req.LoginName, refreshTokenId)
	if err != nil {
		rsp.Reason = err.Error()
		rsp.Elapse = time.Since(tick).String()
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	}

	// cache username and password for web-terminal uses
	svr.neoShellAccount[strings.ToLower(req.LoginName)] = req.Password

	// store refresh token
	svr.jwtCache.SetRefreshToken(refreshTokenId, refreshToken)

	rsp.Success = true
	rsp.Reason = "success"
	rsp.AccessToken = accessToken
	rsp.RefreshToken = refreshToken
	rsp.ServerInfo = svr.getServerInfo()
	rsp.Elapse = time.Since(tick).String()

	ctx.JSON(http.StatusOK, rsp)
}

type ReLoginReq struct {
	RefreshToken string `json:"refreshToken"`
}

type ReLoginRsp LoginRsp

func (svr *httpd) handleReLogin(ctx *gin.Context) {
	var req ReLoginReq
	var rsp = &ReLoginRsp{
		Success: false,
		Reason:  "not specified",
	}

	tick := time.Now()

	err := ctx.Bind(&req)
	if err != nil {
		rsp.Reason = err.Error()
		rsp.Elapse = time.Since(tick).String()
		ctx.JSON(http.StatusBadRequest, rsp)
		return
	}

	// Convert refresh token to refreshClaim and verify it.
	refreshClaim := NewClaimEmpty()
	verified, err := VerifyTokenWithClaim(req.RefreshToken, refreshClaim)
	if err != nil {
		rsp.Reason = err.Error()
		rsp.Elapse = time.Since(tick).String()
		ctx.JSON(http.StatusUnauthorized, rsp)
		return
	}
	if !verified {
		rsp.Reason = "not verified refresh token"
		rsp.Elapse = time.Since(tick).String()
		ctx.JSON(http.StatusUnauthorized, rsp)
		return
	}

	svr.log.Tracef("'%s' relogin", refreshClaim.Subject)

	// Comparing with stored refresh token.
	// load refresh token from cached table by claim.ID
	storedToken, ok := svr.jwtCache.GetRefreshToken(refreshClaim.ID)
	if !ok {
		rsp.Reason = "refresh token not found"
		rsp.Elapse = time.Since(tick).String()
		ctx.JSON(http.StatusUnauthorized, rsp)
		return
	}
	if req.RefreshToken != storedToken {
		rsp.Reason = "invalid refresh token"
		rsp.Elapse = time.Since(tick).String()
		ctx.JSON(http.StatusUnauthorized, rsp)
		return
	}

	// Re-issue access token when stored refresh token is matched with requested refresh token

	/// Note:
	///   In the process of renewing a new accessToken with refreshToken,
	///   refreshToken itself has two options to renew or not to renew.
	///     1) If you renew it like here, the user does not have to log in with ID/PW again even if they continue to use the system.
	///     2) If you do not renew it, you have to log in with ID/PW every time the refreshToken expires.
	accessToken, refreshToken, refreshTokenId, err := svr.issueAccessToken(refreshClaim.Subject)
	if err != nil {
		rsp.Reason = err.Error()
		rsp.Elapse = time.Since(tick).String()
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	}

	// store re-issued refresh token
	svr.jwtCache.SetRefreshToken(refreshTokenId, refreshToken)

	rsp.Success, rsp.Reason = true, "success"
	rsp.AccessToken = accessToken
	rsp.RefreshToken = refreshToken
	rsp.ServerInfo = svr.getServerInfo()
	rsp.Elapse = time.Since(tick).String()

	ctx.JSON(http.StatusOK, rsp)
}

type LogoutReq struct {
	RefreshToken string `json:"refreshToken"`
}

type LogoutRsp struct {
	Success bool   `json:"success"`
	Reason  string `json:"reason"`
	Elapse  string `json:"elapse"`
}

func (svr *httpd) handleLogout(ctx *gin.Context) {
	tick := time.Now()

	var req = &LogoutReq{}
	var rsp = &LogoutRsp{
		Success: false,
		Reason:  "not specified",
	}
	err := ctx.Bind(req)
	if err != nil {
		rsp.Reason = err.Error()
		rsp.Elapse = time.Since(tick).String()
		ctx.JSON(http.StatusBadRequest, rsp)
		return
	}

	refreshClaim := NewClaimEmpty()
	_, err = VerifyTokenWithClaim(req.RefreshToken, refreshClaim)
	if err == nil && len(refreshClaim.ID) > 0 {
		// delete stored refresh token by claim.ID
		svr.jwtCache.RemoveRefreshToken(refreshClaim.ID)
	}

	svr.log.Tracef("logout '%s' success rt.id:'%s'", refreshClaim.Subject, refreshClaim.ID)

	rsp.Success, rsp.Reason = true, "success"
	rsp.Elapse = time.Since(tick).String()
	ctx.JSON(http.StatusOK, rsp)
}

func (svr *httpd) handleCheck(ctx *gin.Context) {
	tick := time.Now()
	claim, claimExists := svr.getJwtClaim(ctx)
	if !claimExists {
		ctx.JSON(http.StatusUnauthorized, "")
	}
	if claim == nil || claim.Valid() != nil {
		ctx.JSON(http.StatusUnauthorized, "")
	}

	if !svr.eulaPassed {
		if _, err := os.Stat(svr.eulaFilePath); err == nil {
			if content, err := os.ReadFile(svr.eulaFilePath); err == nil {
				h := sha1.New()
				h.Write(content)
				installedEulaHash := h.Sum(nil)
				h = sha1.New()
				h.Write([]byte(eulaTxt))
				currentEulaHash := h.Sum(nil)
				svr.eulaPassed = bytes.Equal(installedEulaHash, currentEulaHash)
			}
		}
	}

	if svr.licenseStatusLastTime.IsZero() || time.Since(svr.licenseStatusLastTime) > 30*time.Minute {
		svr.licenseStatusLastTime = time.Now()
		svr.licenseStatus = "Unknown"
		if conn, err := svr.getUserConnection(ctx); err == nil {
			if nfo, err := api.GetLicenseInfo(ctx, conn); err == nil {
				svr.licenseStatus = nfo.LicenseStatus
			}
			conn.Close()
		}
	}

	rsp := &LoginCheckRsp{
		Success:       true,
		EulaRequired:  !svr.eulaPassed,
		LicenseStatus: svr.licenseStatus,
		Reason:        "success",
	}
	rsp.ServerInfo = svr.getServerInfo()
	if svr.experimentModeProvider != nil {
		rsp.ExperimentMode = svr.experimentModeProvider()
	}
	if svr.webShellProvider != nil {
		rsp.Shells = svr.webShellProvider.GetAllShells(true)
	}
	rsp.Elapse = time.Since(tick).String()

	ctx.JSON(http.StatusOK, rsp)
}

func (svr *httpd) handleEula(ctx *gin.Context) {
	switch ctx.Request.Method {
	case http.MethodGet:
		ctx.String(http.StatusOK, eulaTxt)
	case http.MethodPost:
		if err := os.WriteFile(svr.eulaFilePath, []byte(eulaTxt), 0644); err != nil {
			ctx.JSON(http.StatusInternalServerError, map[string]any{"success": false, "reason": err.Error()})
		} else {
			ctx.JSON(http.StatusOK, map[string]any{"success": true, "reason": "success"})
		}
	case http.MethodDelete:
		if err := os.Remove(svr.eulaFilePath); err != nil {
			ctx.JSON(http.StatusInternalServerError, map[string]any{"success": false, "reason": err.Error()})
		} else {
			ctx.JSON(http.StatusOK, map[string]any{"success": true, "reason": "success"})
		}
	default:
		ctx.String(http.StatusMethodNotAllowed, "")
	}
}

func (svr *httpd) getServerInfo() *ServerInfo {
	return &ServerInfo{
		Version: mods.DisplayVersion(),
	}
}

func (svr *httpd) allowDebug(ctx *gin.Context) {
	remote := ctx.RemoteIP()
	pass := false
	if remote == "" || remote == "127.0.0.1" {
		pass = true
	}
	for _, p := range svr.statzAllowed {
		if p == remote {
			pass = true
			break
		}
	}
	if !pass {
		ctx.String(http.StatusForbidden, "")
		ctx.Abort()
		return
	}
	ctx.Next()
}

func (svr *httpd) handleStatz(ctx *gin.Context) {
	ret := map[string]any{}
	includes := ctx.QueryArray("keys")
	format := ctx.Query("format")
	interval := ctx.Query("interval")
	if interval == "" {
		interval = "1m"
	}
	dur, err := time.ParseDuration(interval)
	if err != nil {
		dur = time.Minute
	}

	stat := api.QueryStatzRows(dur, 1, func(key string) (bool, int) {
		return strings.HasPrefix(key, "machbase:") ||
			strings.HasPrefix(key, "go:") ||
			slices.Contains(includes, key), 0
	})
	if stat.Err != nil {
		ctx.String(http.StatusInternalServerError, stat.Err.Error())
		return
	}
	for idx, col := range stat.Cols {
		value := stat.Rows[0].Values[idx]
		valueType := stat.ValueTypes[idx]
		if format == "html" {
			if value == nil {
				ret[col.Name] = "null"
				continue
			}
			printer := message.NewPrinter(language.English)
			switch col.DataType {
			case api.DataTypeInt64:
				ret[col.Name] = printer.Sprintf("%d", value)
			case api.DataTypeFloat64:
				if valueType == "dur" {
					switch val := value.(type) {
					case float64:
						ret[col.Name] = printer.Sprintf("%s", time.Duration(val))
					case int64:
						ret[col.Name] = printer.Sprintf("%s", time.Duration(val))
					default:
						ret[col.Name] = printer.Sprintf("%v", value)
					}
				} else if valueType == "i" {
					switch val := value.(type) {
					case float64:
						ret[col.Name] = printer.Sprintf("%d", int64(val))
					case int64:
						ret[col.Name] = printer.Sprintf("%d", val)
					default:
						ret[col.Name] = printer.Sprintf("%v", value)
					}
				} else {
					ret[col.Name] = printer.Sprintf("%f", value)
				}
			case api.DataTypeString:
				ret[col.Name] = value
			default:
				ret[col.Name] = printer.Sprintf("%v", value)
			}
		} else {
			ret[col.Name] = value
		}
	}
	if format == "html" {
		tpl := template.New("statz").Funcs(template.FuncMap{
			"isMap": func(v any) bool {
				switch v.(type) {
				case map[string]any, map[string]float64, map[string]string, map[string]int64:
					return true
				default:
					return false
				}
			},
		})
		tpl = template.Must(tpl.Parse(tmplStatz))
		if err := tpl.ExecuteTemplate(ctx.Writer, "statz", ret); err != nil {
			ctx.String(http.StatusInternalServerError, err.Error())
		}
	} else {
		ctx.JSON(http.StatusOK, ret)
	}
}

var tmplStatz = `
{{- define "statz" }}
<style>
  table {
    border-collapse: collapse;
  }
  tr:nth-child(even) {
    background-color: #f2f2f2;
  }
  td {
    border: 1px solid #ddd;
    padding: 8px;
  }
</style>
<table>
{{- range $key, $value := . }}
<tr>
  <td>{{ $key }}</td>
  <td>{{ $value }}</td>
</tr>
{{- end }}
</table>
{{ end }}`

type LicenseResponse struct {
	Success bool             `json:"success"`
	Reason  string           `json:"reason"`
	Elapse  string           `json:"elapse"`
	Data    *api.LicenseInfo `json:"data,omitempty"`
}

func (svr *httpd) handleGetLicense(ctx *gin.Context) {
	rsp := &LicenseResponse{Success: false, Reason: "unspecified"}
	tick := time.Now()

	conn, err := svr.getUserConnection(ctx)
	if err != nil {
		rsp.Reason = err.Error()
		ctx.JSON(http.StatusUnauthorized, rsp)
		return
	}
	defer conn.Close()

	nfo, err := api.GetLicenseInfo(ctx, conn)
	if err != nil {
		rsp.Reason = err.Error()
		rsp.Elapse = time.Since(tick).String()
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	}
	rsp.Success, rsp.Reason = true, "success"
	rsp.Data = nfo
	rsp.Elapse = time.Since(tick).String()
	ctx.JSON(http.StatusOK, rsp)
}

func (svr *httpd) handleInstallLicense(ctx *gin.Context) {
	rsp := &LicenseResponse{Success: false, Reason: "unspecified"}
	tick := time.Now()

	file, fileHeader, err := ctx.Request.FormFile("license.dat")
	if err != nil {
		rsp.Reason = err.Error()
		rsp.Elapse = time.Since(tick).String()
		ctx.JSON(http.StatusBadRequest, rsp)
		return
	}
	defer file.Close()

	if fileHeader.Size > 4096 {
		// too big as a license file, user might send wrong file.
		rsp.Reason = "Too large file as a license file."
		rsp.Elapse = time.Since(tick).String()
		ctx.JSON(http.StatusBadRequest, rsp)
		return
	}
	content, err := io.ReadAll(file)
	if err != nil {
		rsp.Reason = err.Error()
		rsp.Elapse = time.Since(tick).String()
		ctx.JSON(http.StatusBadRequest, rsp)
		return
	}

	conn, err := svr.getUserConnection(ctx)
	if err != nil {
		rsp.Reason = err.Error()
		ctx.JSON(http.StatusUnauthorized, rsp)
		return
	}
	defer conn.Close()

	nfo, err := api.InstallLicenseData(ctx, conn, svr.licenseFilePath, content)
	if err != nil {
		fmt.Println("ERR", err.Error())
		rsp.Reason = err.Error()
		rsp.Elapse = time.Since(tick).String()
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	}
	rsp.Success, rsp.Reason = true, "Successfully registered."
	rsp.Data = nfo
	rsp.Elapse = time.Since(tick).String()
	ctx.JSON(http.StatusOK, rsp)
}

var (
	mdFileRootRegexp = regexp.MustCompile(`{{\s*file_root\s*}}`)
	mdFilePathRegexp = regexp.MustCompile(`{{\s*file_path\s*}}`)
	mdFileNameRegexp = regexp.MustCompile(`{{\s*file_name\s*}}`)
	mdFileDirRegexp  = regexp.MustCompile(`{{\s*file_dir\s*}}`)
)

// POST "/md"
// POST "/md?darkMode=true"
func (svr *httpd) handleMarkdown(ctx *gin.Context) {
	src, err := io.ReadAll(ctx.Request.Body)
	if err != nil {
		ctx.String(http.StatusBadRequest, err.Error())
		return
	}
	var referer string
	if dec, err := base64.StdEncoding.DecodeString(ctx.GetHeader("X-Referer")); err != nil {
		ctx.String(http.StatusBadRequest, err.Error())
		return
	} else {
		referer = string(dec)
	}
	// referer := "http://127.0.0.1:5654/web/api/tql/sample_image.wrk" // if file has been saved
	// referer := "http://127.0.0.1:5654/web/ui" // file is not saved
	var filePath, fileName, fileDir string
	if u, err := url.Parse(referer); err == nil {
		// {{ file_path }} => /web/api/tql/path/to/file.wrk
		// {{ file_name }} => file.wrk
		// {{ file_dir }}  => /web/api/tql/path/to
		filePath = u.Path
		fileName = path.Base(filePath)
		fileDir = path.Dir(filePath)
	}
	// {{ file_root }} => /web/api/tql
	fileRoot := path.Join(strings.TrimSuffix(ctx.Request.URL.Path, "/md"), "tql")
	src = mdFileRootRegexp.ReplaceAll(src, []byte(fileRoot))
	src = mdFilePathRegexp.ReplaceAll(src, []byte(filePath))
	src = mdFileNameRegexp.ReplaceAll(src, []byte(fileName))
	src = mdFileDirRegexp.ReplaceAll(src, []byte(fileDir))
	src = replaceHttpClient(src, true)

	ctx.Writer.Header().Set("Content-Type", "application/xhtml+xml")
	conv := mdconv.New(mdconv.WithDarkMode(strBool(ctx.Query("darkMode"), false)))
	ctx.Writer.Write([]byte("<div>"))
	err = conv.Convert(src, ctx.Writer)
	if err != nil {
		ctx.String(http.StatusInternalServerError, fmt.Sprintf(`<p>%s</p>`, err.Error()))
	}
	ctx.Writer.Write([]byte("</div>"))
}

func replaceHttpClient(src []byte, preserveSrc bool) []byte {
	if !bytes.Contains(src, []byte("```http")) {
		return src
	}
	offset := 0
	for {
		fenceBegin := bytes.Index(src[offset:], []byte("```http"))
		if fenceBegin == -1 {
			break
		}
		fenceBegin = offset + fenceBegin
		contentBegin := fenceBegin + 7 // length of "```http"
		contentEnd := contentBegin

		if end := bytes.Index(src[contentBegin:], []byte("```")); end == -1 {
			break
		} else {
			contentEnd = contentBegin + end
		}
		fenceEnd := contentEnd + 3 // length of "```"
		offset = fenceEnd

		content := src[contentBegin:contentEnd]
		restCli, err := restclient.Parse(string(content))
		if err != nil {
			return bytes.Join([][]byte{
				src[0:fenceEnd],
				[]byte("\n" + err.Error()),
				src[fenceEnd:]},
				[]byte("\n"))
		}
		restRsp := restCli.Do()
		resultString := restRsp.String()

		newSrc := [][]byte{}
		if preserveSrc { // preserve original source code
			newSrc = append(newSrc, src[0:fenceEnd])
		} else { // replace original source code with the result
			newSrc = append(newSrc, src[0:fenceBegin])
		}
		newSrc = append(newSrc,
			[]byte("```http"),
			[]byte(resultString),
			[]byte("```"),
			src[fenceEnd:],
		)
		src = bytes.Join(newSrc, []byte("\n"))
		offset += 10 + len(resultString) + 4 // 10 = len("```http") + len("```")
	}
	return src
}

func (svr *httpd) handleConsoleData(ctx *gin.Context) {
	consoleId := ctx.Param("console_id")
	if len(consoleId) == 0 {
		ctx.String(http.StatusBadRequest, "invalid consoleId")
		return
	}
	// current websocket spec requires pass the token through handshake process
	token := ctx.Query("token")
	claim, err := svr.verifyAccessToken(token)
	if err != nil {
		ctx.String(http.StatusUnauthorized, "unauthorized access")
		return
	}
	conn, err := upgrader.Upgrade(ctx.Writer, ctx.Request, nil)
	if err != nil {
		svr.log.Errorf("console ws upgrade fail %s", err.Error())
		ctx.String(http.StatusBadRequest, err.Error())
		return
	}

	cons := NewWebConsole(claim.Subject, consoleId, conn)
	go cons.readerLoop()
	go cons.flushLoop()
}

type WebConsole struct {
	log       logging.Log
	username  string
	consoleId string
	topic     string
	conn      *websocket.Conn
	connMutex sync.Mutex
	closeOnce sync.Once
	closed    bool

	messages      []*eventbus.Event
	lastFlushTime time.Time
	flushPeriod   time.Duration
}

func NewWebConsole(username string, consoleId string, conn *websocket.Conn) *WebConsole {
	ret := &WebConsole{
		log:           logging.GetLog(fmt.Sprintf("console-%s-%s", username, consoleId)),
		topic:         fmt.Sprintf("console:%s:%s", username, consoleId),
		username:      username,
		consoleId:     consoleId,
		conn:          conn,
		lastFlushTime: time.Now(),
		flushPeriod:   300 * time.Millisecond,
	}
	eventbus.Default.SubscribeAsync(ret.topic, ret.sendMessage, true)
	return ret
}

func (cons *WebConsole) Close() {
	cons.closeOnce.Do(func() {
		cons.closed = true
		eventbus.Default.Unsubscribe(cons.topic, cons.sendMessage)
		if cons.conn != nil {
			cons.conn.Close()
		}
	})
}

func (cons *WebConsole) readerLoop() {
	defer func() {
		cons.Close()
		if e := recover(); e != nil {
			cons.log.Error("panic recover %s", e)
		}
	}()

	cons.log.Trace("websocket: established")
	for {
		evt := &eventbus.Event{}
		err := cons.conn.ReadJSON(evt)
		if err != nil {
			if we, ok := err.(*websocket.CloseError); ok {
				cons.log.Trace(we.Error())
			} else if !errors.Is(err, io.EOF) {
				cons.log.Warn("ERR", err.Error())
			}
			cons.connMutex.Lock()
			cons.conn.WriteControl(websocket.CloseMessage, []byte{}, time.Now().Add(200*time.Millisecond))
			cons.connMutex.Unlock()
			return
		}
		switch evt.Type {
		case eventbus.EVT_PING:
			rsp := eventbus.NewPing(evt.Ping.Tick)
			cons.connMutex.Lock()
			cons.conn.WriteJSON(rsp)
			cons.connMutex.Unlock()
		}
	}
}

func (cons *WebConsole) flushLoop() {
	ticker := time.NewTicker(cons.flushPeriod)
	for range ticker.C {
		if cons.closed {
			break
		}
		cons.sendMessage(nil)
	}
	ticker.Stop()
}

func (cons *WebConsole) sendMessage(evt *eventbus.Event) {
	shouldAppend := true
	forceFlush := false

	cons.connMutex.Lock()
	defer cons.connMutex.Unlock()

	if evt != nil && evt.Type == eventbus.EVT_LOG &&
		len(cons.messages) > 0 &&
		cons.messages[len(cons.messages)-1].Type == eventbus.EVT_LOG {

		lastLog := cons.messages[len(cons.messages)-1].Log
		if lastLog.Message == evt.Log.Message {
			if lastLog.Repeat == 0 {
				lastLog.Repeat = 1
			}
			lastLog.Repeat += 1
			shouldAppend = false
		}
	} else if evt != nil && evt.Type != eventbus.EVT_LOG {
		forceFlush = true
	}

	if evt != nil && shouldAppend {
		cons.messages = append(cons.messages, evt)
	}

	if !forceFlush && time.Since(cons.lastFlushTime) < cons.flushPeriod {
		// do not flush for now
		return
	}

	for _, msg := range cons.messages {
		err := cons.conn.WriteJSON(msg)
		if err != nil {
			cons.log.Warn("ERR", err.Error())
			cons.Close()
			break
		}
	}
	cons.lastFlushTime = time.Now()
	cons.messages = cons.messages[0:0]
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func (svr *httpd) handleTermData(ctx *gin.Context) {
	termId := ctx.Param("term_id")
	if len(termId) == 0 {
		ctx.String(http.StatusBadRequest, "invalid termId")
		return
	}
	// user able to decide shell other than "machbase-neo shell"
	userShell := ctx.Query("shell")

	// current websocket spec requires pass the token through handshake process
	token := ctx.Query("token")
	claim, err := svr.verifyAccessToken(token)
	if err != nil {
		ctx.String(http.StatusUnauthorized, "unauthorized access")
		return
	}
	termLoginName := claim.Subject
	termPassword := svr.neoShellAccount[strings.ToLower(termLoginName)]
	if len(termPassword) == 0 {
		termPassword = "manager"
	}
	termAddress := svr.neoShellAddress
	if len(termAddress) == 0 {
		termAddress = "127.0.0.1:5652"
	}
	termIdleTimeout := time.Duration(0) // 0 - no timeout

	termKey := fmt.Sprintf("%s-%s", termLoginName, termId)

	conn, err := upgrader.Upgrade(ctx.Writer, ctx.Request, nil)
	if err != nil {
		svr.log.Errorf("term ws upgrade fail %s", err.Error())
		ctx.String(http.StatusBadRequest, err.Error())
		return
	}
	defer conn.Close()

	if userShell == model.SHELLID_JSH {
		// TODO: client should send X-Console-Id,
		// but this handler is for the web socket
		// and web socket can not assign http header of the request.
		consoleInfo := parseConsoleId(ctx)
		wsRw := &WsReadWriter{Conn: conn}
		terminals.Register(termKey, (*WebTerm)(nil)) // TODO set windows size for JSH
		defer terminals.Unregister(termKey)
		j := jsh.NewJsh(
			ctx,
			jsh.WithNativeModules(jsh.NativeModuleNames()...),
			jsh.WithParent(nil),
			jsh.WithReader(wsRw),
			jsh.WithWriter(wsRw),
			jsh.WithEcho(true),
			jsh.WithNewLineCRLF(true),
			jsh.WithUserName(termLoginName),
			jsh.WithConsoleId(consoleInfo.consoleId),
		)
		err = j.Exec([]string{"@"})
		if err != nil {
			for _, err := range j.Errors() {
				svr.log.Warnf("term jsh %s", jsh.ErrorToString(err))
				// Check if the connection is hijacked by attempting a zero-byte write.
				_, err := ctx.Writer.Write(nil)
				if !errors.Is(err, http.ErrHijacked) {
					ctx.String(http.StatusInternalServerError, jsh.ErrorToString(err))
				}
			}
		}
		return
	}

	_, _, err = net.SplitHostPort(termAddress)
	if err != nil {
		svr.log.Warnf("term invalid address %s", err.Error())
		ctx.String(http.StatusInternalServerError, err.Error())
		return
	}

	term, err := NewWebTerm(termAddress, userShell, termLoginName, termPassword)
	if err != nil {
		svr.log.Warnf("term conn %s", err.Error())
		ctx.String(http.StatusBadGateway, err.Error())
		return
	}

	svr.log.Debugf("term %s register %s", termKey, termAddress)
	terminals.Register(termKey, term)

	defer func() {
		svr.log.Debugf("term %s unregister %s", termKey, termAddress)
		terminals.Unregister(termKey)
		if term != nil {
			term.Close()
		}
	}()

	onceCloseMessage := sync.Once{}

	go func() {
		defer func() {
			if e := recover(); e != nil {
				svr.log.Errorf("term %s recover %s", termKey, e)
			}
		}()
		b := [termBuffSize]byte{}
		for {
			n, err := term.Stdout.Read(b[:])
			if err != nil {
				if !errors.Is(err, io.EOF) {
					conn.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("\r\nError: %s\r\n", err.Error())))
					conn.WriteControl(websocket.CloseMessage, []byte{}, time.Now().Add(200*time.Millisecond))
					svr.log.Errorf("term %s error %s", termKey, err.Error())
				} else {
					onceCloseMessage.Do(func() {
						conn.WriteMessage(websocket.TextMessage, []byte("\r\nclosed.\r\n"))
						conn.WriteControl(websocket.CloseMessage, []byte{}, time.Now().Add(200*time.Millisecond))
					})
				}
				return
			}
			if n == 0 {
				continue
			}
			conn.WriteMessage(websocket.BinaryMessage, b[:n])
		}
	}()

	go func() {
		defer func() {
			if e := recover(); e != nil {
				svr.log.Errorf("term %s recover %s", termKey, e)
			}
		}()
		b := [termBuffSize]byte{}
		for {
			n, err := term.Stderr.Read(b[:])
			if err != nil {
				if !errors.Is(err, io.EOF) {
					conn.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("\r\nError: %s\r\n", err.Error())))
					conn.WriteControl(websocket.CloseMessage, []byte{}, time.Now().Add(200*time.Millisecond))
					svr.log.Errorf("term %s error %s", termKey, err.Error())
				} else {
					onceCloseMessage.Do(func() {
						conn.WriteMessage(websocket.TextMessage|websocket.CloseMessage, []byte("\r\nclosed.\r\n"))
						conn.WriteControl(websocket.CloseMessage, []byte{}, time.Now().Add(200*time.Millisecond))
					})
				}
				return
			}
			if n == 0 {
				continue
			}
			conn.WriteMessage(websocket.BinaryMessage, b[:n])
		}
	}()

	ticker := time.NewTicker(30 * time.Second)
	tickerStop := make(chan bool, 1)
	defer func() {
		ticker.Stop()
		tickerStop <- true
		close(tickerStop)
	}()

	go func() {
		for {
			select {
			case <-ticker.C:
				term.session.SendRequest("no-op", false, []byte{})
			case <-tickerStop:
				return
			}
		}
	}()

	for {
		if termIdleTimeout > 0 {
			conn.SetReadDeadline(time.Now().Add(termIdleTimeout))
		}
		_, message, err := conn.ReadMessage()
		if err != nil {
			if closeErr, ok := err.(*websocket.CloseError); ok {
				svr.log.Debugf("term %s closed by websocket %d %s", termKey, closeErr.Code, closeErr.Text)
			} else if !errors.Is(err, io.EOF) {
				svr.log.Errorf("term %s error %T %s", termKey, err, err.Error())
			}
			conn.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("\r\nconnection closed. %s\r\n", err.Error())))
			conn.WriteControl(websocket.CloseMessage, []byte{}, time.Now().Add(200*time.Millisecond))
			return
		}
		_, err = term.Stdin.Write(message)
		if err != nil {
			if !errors.Is(err, io.EOF) {
				conn.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("\r\nError: %s\r\n", err.Error())))
				conn.WriteControl(websocket.CloseMessage, []byte{}, time.Now().Add(200*time.Millisecond))
				svr.log.Errorf("%s term error %T %s", termKey, err, err.Error())
			}
			return
		}
	}
}

type setTerminalSizeRequest struct {
	Rows int `query:"rows" form:"rows" json:"rows"`
	Cols int `query:"cols" form:"cols" json:"cols"`
}

func (svr *httpd) handleTermWindowSize(ctx *gin.Context) {
	termId := ctx.Param("term_id")

	claimAny, claimExists := ctx.Get("jwt-claim")
	if !claimExists {
		ctx.String(http.StatusUnauthorized, "unauthorized access")
		return
	}
	claim := claimAny.(Claim)
	termLoginName := claim.Subject

	req := &setTerminalSizeRequest{}
	if err := ctx.Bind(req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"success": false, "reason": err.Error()})
		return
	}
	if req.Rows == 0 || req.Cols == 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"success": false, "reason": "rows or cols can't be zero"})
		return
	}
	termKey := fmt.Sprintf("%s-%s", termLoginName, termId)
	if term, ok := terminals.Find(termKey); !ok {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"reason":  fmt.Sprintf("term '%s' not found", termKey)})
		return
	} else if term != nil { // If the websocket is JSH, *WebTerm is nil
		err := term.SetWindowSize(req.Rows, req.Cols)
		if err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"success": false, "reason": err.Error()})
			return
		}
	}
	ctx.JSON(http.StatusOK, gin.H{"success": true, "reason": "success"})
}

const termBuffSize = 4096

var terminals = &Terminals{
	list: cmap.New[*WebTerm](),
}

type Terminals struct {
	list cmap.ConcurrentMap[string, *WebTerm]
}

func (terms *Terminals) Register(termKey string, term *WebTerm) {
	terms.list.Set(termKey, term)
}

func (terms *Terminals) Unregister(termKey string) {
	terms.list.Remove(termKey)
}

func (terms *Terminals) Find(termKey string) (*WebTerm, bool) {
	if term, ok := terms.list.Get(termKey); ok {
		return term, true
	}
	return nil, false
}

type WebTerm struct {
	Type   string
	Rows   int
	Cols   int
	Stdout io.Reader
	Stderr io.Reader
	Stdin  io.WriteCloser
	Since  time.Time

	conn    *ssh.Client
	session *ssh.Session

	userShell string
}

func NewWebTerm(hostPort string, userShell string, user string, password string) (*WebTerm, error) {
	var loginString string
	if len(userShell) > 0 {
		loginString = fmt.Sprintf("%s:%s", user, userShell)
	} else {
		loginString = user
	}
	conn, err := ssh.Dial("tcp", hostPort, &ssh.ClientConfig{
		User: loginString,
		Auth: []ssh.AuthMethod{
			ssh.Password(password),
		},
		HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error { return nil },
	})
	if err != nil {
		return nil, fmt.Errorf("NewTerm dial, %s", err.Error())
	}

	// Creating a session from the connection
	session, err := conn.NewSession()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("NewTerm new session, %s", err.Error())
	}
	term := &WebTerm{
		Type:      "xterm",
		Rows:      25,
		Cols:      80,
		Since:     time.Now(),
		conn:      conn,
		session:   session,
		userShell: userShell,
	}
	term.Stdout, err = session.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("NewTerm stdout pipe, %s", err.Error())
	}
	term.Stderr, err = session.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("NewTerm stderr pipe, %s", err.Error())
	}
	term.Stdin, err = session.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("NewTerm stdin pipe, %s", err.Error())
	}

	// request pty
	err = session.RequestPty(term.Type, term.Rows, term.Cols, ssh.TerminalModes{
		ssh.ECHO: 1, // disable echoing
	})
	if err != nil {
		term.Stdin.Close()
		session.Close()
		return nil, fmt.Errorf("NewTerm pty, %s", err.Error())
	}
	// request shell
	err = session.Shell()
	if err != nil {
		term.Stdin.Close()
		session.Close()
		conn.Close()
		return nil, fmt.Errorf("NewTerm shell, %s", err.Error())
	}

	return term, nil
}

func (term *WebTerm) SetWindowSize(rows, cols int) error {
	err := term.session.WindowChange(rows, cols)
	if err != nil {
		return fmt.Errorf("SetWindowSize, %s", err.Error())
	}
	term.Rows, term.Cols = rows, cols
	return nil
}

func (term *WebTerm) Close() {
	if term.Stdin != nil {
		term.Stdin.Close()
	}
	if term.session != nil {
		term.session.Signal(ssh.SIGKILL)
		term.session.Close()
	}
	if term.conn != nil {
		term.conn.Close()
	}
}

type SsfsResponse struct {
	Success bool        `json:"success"`
	Reason  string      `json:"reason"`
	Elapse  string      `json:"elapse"`
	Data    *ssfs.Entry `json:"data,omitempty"`
}

func isFsFile(path string) bool {
	return contentTypeOfFile(path) != ""
}

// returns supported content-type of the given file path (name),
// if the name is an unsupported file type, it returns empty string
func contentTypeOfFile(name string) string {
	ext := filepath.Ext(name)
	switch strings.ToLower(ext) {
	default:
		return ""
	case ".sql":
		return "text/plain"
	case ".tql":
		return "text/plain"
	case ".taz":
		return "application/json"
	case ".wrk":
		return "application/json"
	case ".dsh":
		return "application/json"
	// image files
	case ".apng":
		return "image/apng"
	case ".avif":
		return "image/avif"
	case ".gif":
		return "image/gif"
	case ".jpeg", ".jpg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".svg":
		return "image/svg+xml"
	case ".webp":
		return "image/webp"
	case ".bmp":
		return "image/bmp"
	case ".ico":
		return "image/x-icon"
	case ".tiff":
		return "image/tiff"
	// text files
	case ".txt":
		return "text/plain"
	case ".json":
		return "application/json"
	case ".csv":
		return "text/csv"
	case ".md", ".markdown":
		return "text/markdown"
	case ".css":
		return "text/css"
	case ".js", ".mjs":
		return "text/javascript"
	case ".htm", ".html":
		return "text/html"
	case ".py":
		return "text/x-python"
	case ".sh":
		return "text/x-shellscript"
	case ".ipynb":
		return "application/x-ipynb+json"
	}
}

func (svr *httpd) handleFiles(ctx *gin.Context) {
	rsp := &SsfsResponse{Success: false, Reason: "not specified"}
	tick := time.Now()
	path := ctx.Param("path")
	filter := ctx.Query("filter")
	recursive := strBool(ctx.Query("recursive"), false)

	switch ctx.Request.Method {
	case http.MethodGet:
		var ent *ssfs.Entry
		var err error
		if isFsFile(filter) {
			ent, err = svr.serverFs.GetGlob(path, filter)
		} else {
			ent, err = svr.serverFs.GetFilter(path, func(se *ssfs.SubEntry) bool {
				if se.IsDir {
					return true
				}
				return contentTypeOfFile(se.Name) != ""
			})
		}
		if err != nil {
			rsp.Reason = err.Error()
			rsp.Elapse = time.Since(tick).String()
			ctx.JSON(http.StatusNotFound, rsp)
			return
		}
		if ent != nil {
			if ent.IsDir {
				rsp.Success, rsp.Reason = true, "success"
				rsp.Elapse = time.Since(tick).String()
				rsp.Data = ent
				ctx.JSON(http.StatusOK, rsp)
				return
			}
			if contentType := contentTypeOfFile(ent.Name); contentType != "" {
				ctx.Data(http.StatusOK, contentType, ent.Content)
				return
			}
		}
		rsp.Reason = fmt.Sprintf("not found: %s", path)
		rsp.Elapse = time.Since(tick).String()
		ctx.JSON(http.StatusNotFound, rsp)
		return
	case http.MethodDelete:
		ent, err := svr.serverFs.Get(path)
		if err != nil {
			rsp.Reason = err.Error()
			rsp.Elapse = time.Since(tick).String()
			ctx.JSON(http.StatusNotFound, rsp)
			return
		}
		if ent.IsDir {
			if len(ent.Children) == 0 || recursive {
				if recursive {
					err = svr.serverFs.RemoveRecursive(path)
				} else {
					err = svr.serverFs.Remove(path)
				}
				if err != nil {
					rsp.Reason = err.Error()
					rsp.Elapse = time.Since(tick).String()
					ctx.JSON(http.StatusInternalServerError, rsp)
					return
				} else {
					rsp.Success, rsp.Reason = true, "success"
					rsp.Elapse = time.Since(tick).String()
					ctx.JSON(http.StatusOK, rsp)
					return
				}
			} else {
				rsp.Reason = "directory is not empty"
				rsp.Elapse = time.Since(tick).String()
				ctx.JSON(http.StatusUnprocessableEntity, rsp)
				return
			}
		} else if isFsFile(path) {
			err = svr.serverFs.Remove(path)
			if err != nil {
				rsp.Reason = err.Error()
				rsp.Elapse = time.Since(tick).String()
				ctx.JSON(http.StatusInternalServerError, rsp)
				return
			} else {
				rsp.Success, rsp.Reason = true, "success"
				rsp.Elapse = time.Since(tick).String()
				ctx.JSON(http.StatusOK, rsp)
				return
			}
		} else {
			rsp.Reason = fmt.Sprintf("not found: %s", path)
			rsp.Elapse = time.Since(tick).String()
			ctx.JSON(http.StatusNotFound, rsp)
			return
		}
	case http.MethodPost:
		if isFsFile(path) {
			content, err := io.ReadAll(ctx.Request.Body)
			if err != nil {
				rsp.Reason = err.Error()
				rsp.Elapse = time.Since(tick).String()
				ctx.JSON(http.StatusInternalServerError, rsp)
				return
			}
			err = svr.serverFs.Set(path, content)
			if err == nil {
				rsp.Success, rsp.Reason = true, "success"
				rsp.Elapse = time.Since(tick).String()
				ctx.JSON(http.StatusOK, rsp)
				return
			} else {
				rsp.Reason = err.Error()
				rsp.Elapse = time.Since(tick).String()
				ctx.JSON(http.StatusInternalServerError, rsp)
				return
			}
		} else {
			content, err := io.ReadAll(ctx.Request.Body)
			if err != nil {
				rsp.Reason = err.Error()
				rsp.Elapse = time.Since(tick).String()
				ctx.JSON(http.StatusInternalServerError, rsp)
				return
			}
			var entry *ssfs.Entry
			if len(content) > 0 && ctx.ContentType() == "application/json" {
				var topic string
				if claim, exists := svr.getJwtClaim(ctx); exists {
					consoleInfo := parseConsoleId(ctx)
					topic = fmt.Sprintf("console:%s:%s", claim.Subject, consoleInfo.consoleId)
				}
				cloneReq := &GitCloneReq{}
				err = json.Unmarshal(content, cloneReq)
				if err == nil {
					cloneReq.logTopic = topic
					switch strings.ToLower(cloneReq.Cmd) {
					default:
						entry, err = svr.serverFs.GitClone(path, cloneReq.Url, cloneReq)
					case "pull":
						entry, err = svr.serverFs.GitPull(path, cloneReq.Url, cloneReq)
					}
				}
			} else {
				entry, err = svr.serverFs.MkDir(path)
			}
			if err == nil {
				rsp.Success, rsp.Reason = true, "success"
				rsp.Elapse = time.Since(tick).String()
				rsp.Data = entry
				ctx.JSON(http.StatusOK, rsp)
				return
			} else {
				rsp.Reason = err.Error()
				rsp.Elapse = time.Since(tick).String()
				ctx.JSON(http.StatusInternalServerError, rsp)
				return
			}
		}
	case http.MethodPut:
		req := RenameReq{}
		if err := ctx.Bind(&req); err != nil {
			rsp.Reason = err.Error()
			rsp.Elapse = time.Since(tick).String()
			ctx.JSON(http.StatusBadRequest, rsp)
			return
		}
		if req.Dest == "" {
			rsp.Reason = "destination is not specified."
			rsp.Elapse = time.Since(tick).String()
			ctx.JSON(http.StatusBadRequest, rsp)
			return
		}
		if err := svr.serverFs.Rename(path, req.Dest); err != nil {
			rsp.Reason = err.Error()
			rsp.Elapse = time.Since(tick).String()
			ctx.JSON(http.StatusInternalServerError, rsp)
			return
		}
		rsp.Success, rsp.Reason = true, "success"
		rsp.Elapse = time.Since(tick).String()
		ctx.JSON(http.StatusOK, rsp)
		return
	}
}

type RenameReq struct {
	Dest string `json:"destination"`
}

type GitCloneReq struct {
	Cmd      string `json:"command"`
	Url      string `json:"url"`
	logTopic string `json:"-"`
}

func (gitClone *GitCloneReq) Write(b []byte) (int, error) {
	if gitClone.logTopic == "" {
		return os.Stdout.Write(b)
	} else {
		taskId := fmt.Sprintf("%p", gitClone)
		lines := bytes.Split(b, []byte{'\n'})
		for _, line := range lines {
			carriageReturns := bytes.Split(line, []byte{'\r'})
			for i := len(carriageReturns) - 1; i >= 0; i-- {
				line = bytes.TrimSpace(carriageReturns[i])
				if len(line) > 0 {
					break
				}
			}
			if len(line) > 0 {
				eventbus.PublishLogTask(gitClone.logTopic, "INFO", taskId, string(line))
			}
		}
		return len(b), nil
	}
}

type RefsResponse struct {
	Success bool   `json:"success"`
	Reason  string `json:"reason"`
	Elapse  string `json:"elapse"`
	Data    struct {
		Refs []*WebReferenceGroup `json:"refs,omitempty"`
	} `json:"data"`
}

func (svr *httpd) handleRefs(ctx *gin.Context) {
	rsp := &RefsResponse{Reason: "unspecified"}
	tick := time.Now()
	path := ctx.Param("path")

	if path == "/" {
		references := &WebReferenceGroup{Label: "REFERENCES"}
		references.Items = append(references.Items, ReferenceItem{Type: "url", Title: "machbase-neo docs", Addr: "https://docs.machbase.com/neo", Target: "_blank"})
		references.Items = append(references.Items, ReferenceItem{Type: "url", Title: "machbase sql reference", Addr: "https://docs.machbase.com/dbms/sql-ref/", Target: "_docs_machbase"})
		references.Items = append(references.Items, ReferenceItem{Type: "url", Title: "https://machbase.com", Addr: "https://machbase.com/", Target: "_home_machbase"})
		references.Items = append(references.Items, ReferenceItem{Type: "url", Title: "Tutorials", Addr: "https://github.com/machbase/neo-tutorials", Target: "_blank"})
		references.Items = append(references.Items, ReferenceItem{Type: "url", Title: "Demo web app", Addr: "https://github.com/machbase/neo-apps"})

		sdk := &WebReferenceGroup{Label: "SDK"}
		sdk.Items = append(sdk.Items, ReferenceItem{Type: "url", Title: "SDK Download", Addr: "https://docs.machbase.com/neo/releases/#sdk-with-classic", Target: "_home_machbase"})
		sdk.Items = append(sdk.Items, ReferenceItem{Type: "url", Title: ".NET Connector", Addr: "https://docs.machbase.com/dbms/sdk/dotnet/", Target: "_docs_machbase"})
		sdk.Items = append(sdk.Items, ReferenceItem{Type: "url", Title: "JDBC Driver", Addr: "https://docs.machbase.com/dbms/sdk/jdbc/", Target: "_docs_machbase"})
		sdk.Items = append(sdk.Items, ReferenceItem{Type: "url", Title: "ODBC", Addr: "https://docs.machbase.com/dbms/sdk/cli-odbc/", Target: "_docs_machbase"})
		sdk.Items = append(sdk.Items, ReferenceItem{Type: "url", Title: "ODBC Example", Addr: "https://docs.machbase.com/dbms/sdk/cli-odbc-example/", Target: "_docs_machbase"})

		cheatSheets := &WebReferenceGroup{Label: "CHEAT SHEETS"}
		cheatSheets.Items = append(cheatSheets.Items, ReferenceItem{Type: "wrk", Title: "markdown example", Addr: "./tutorials/sample_markdown.wrk"})
		cheatSheets.Items = append(cheatSheets.Items, ReferenceItem{Type: "wrk", Title: "mermaid example", Addr: "./tutorials/sample_mermaid.wrk"})
		cheatSheets.Items = append(cheatSheets.Items, ReferenceItem{Type: "wrk", Title: "pikchr example", Addr: "./tutorials/sample_pikchr.wrk"})

		rsp.Data.Refs = []*WebReferenceGroup{references, sdk, cheatSheets}
		rsp.Success, rsp.Reason = true, "success"
		rsp.Elapse = time.Since(tick).String()
		ctx.JSON(http.StatusOK, rsp)
	} else {
		rsp.Reason = fmt.Sprintf("'%s' not found", path)
		rsp.Elapse = time.Since(tick).String()
		ctx.JSON(http.StatusNotFound, rsp)
	}
}
