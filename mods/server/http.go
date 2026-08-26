package server

import (
	"bytes"
	"context"
	"crypto/sha1"
	"database/sql"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	httpPprof "net/http/pprof"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v4"
	"github.com/gorilla/websocket"
	"github.com/machbase/neo-server/v8/jsh/service"
	"github.com/machbase/neo-server/v8/mods"
	"github.com/machbase/neo-server/v8/mods/eventbus"
	"github.com/machbase/neo-server/v8/mods/logging"
	"github.com/machbase/neo-server/v8/mods/model"
	"github.com/machbase/neo-server/v8/mods/scheduler"
	"github.com/machbase/neo-server/v8/mods/tql"
	"github.com/machbase/neo-server/v8/mods/util"
	"github.com/machbase/neo-server/v8/mods/util/mdconv"
	"github.com/machbase/neo-server/v8/mods/util/metric"
	"github.com/machbase/neo-server/v8/mods/util/ssfs"
	"github.com/machbase/neo-server/v8/spi"
	cmap "github.com/orcaman/concurrent-map/v2"
	"golang.org/x/crypto/ssh"
)

// Factory
func NewHttp(options ...HttpOption) (*httpd, error) {
	s := &httpd{
		log:      logging.GetLog("httpd"),
		jwtCache: NewJwtCache(),
		pathMap:  map[string]string{},
	}
	for _, opt := range options {
		opt(s)
	}
	return s, nil
}

type httpd struct {
	log   logging.Log
	alive bool

	listenAddresses []string
	enableTokenAuth bool
	mqttWsHandler   func(*gin.Context)

	httpServer *http.Server
	listeners  []net.Listener
	jwtCache   JwtCache

	authServer        *Server
	serviceController *service.Controller
	tqlLoader         tql.Loader
	serverFs          *ssfs.SSFS

	eulaPassed             bool
	eulaFilePath           string
	licenseFilePath        string
	licenseStatusLastTime  time.Time
	licenseStatus          string
	debugModeLock          sync.RWMutex
	debugMode              bool
	debugLogFilterLatency  time.Duration
	readBufSize            int
	writeBufSize           int
	linger                 int
	keepAlive              int
	experimentModeProvider func() bool
	uiContentFs            http.FileSystem

	pathMap map[string]string

	statzAllowed []string
	statzToken   string
	cypherAlg    string
	cypherKey    string
	cypherPad    string
}

type HttpOption func(s *httpd)

// ListenAddresses
func WithHttpListenAddress(addrs ...string) HttpOption {
	return func(s *httpd) {
		s.listenAddresses = append(s.listenAddresses, addrs...)
	}
}

// AuthServer
func WithHttpAuthServer(authSvc *Server, enabled bool) HttpOption {
	return func(s *httpd) {
		s.authServer = authSvc
		s.enableTokenAuth = enabled
		if authSvc != nil && authSvc.serviceController != nil {
			s.serviceController = authSvc.serviceController
		}
		if enabled {
			s.log.Infof("HTTP token authentication enabled")
		} else {
			s.log.Infof("HTTP token authentication disabled")
		}
	}
}

// neo-shell address
func WithHttpNeoShellAddress(addrs ...string) HttpOption {
	return func(s *httpd) {
		candidates := []string{}
		for _, addr := range addrs {
			if strings.HasPrefix(addr, "tcp://127.0.0.1:") || strings.HasPrefix(addr, "tcp://localhost:") {
				s.authServer.neoShellAddress = strings.TrimPrefix(addr, "tcp://")
				// if loopback is available, use it for web-terminal
				// eliminate other candiates
				candidates = candidates[:0]
				break
			} else if strings.HasPrefix(addr, "tcp://") {
				candidates = append(candidates, strings.TrimPrefix(addr, "tcp://"))
			}
		}
		if len(candidates) > 0 {
			// TODO choose one from the candidates, !EXCLUDE! virtual/tunnel ethernet addresses
			s.authServer.neoShellAddress = candidates[0]
		}
	}
}

// license file path
func WithHttpLicenseFilePath(path string) HttpOption {
	return func(s *httpd) {
		s.licenseFilePath = path
	}
}

// End User License Agreement (EULA) file path
func WithHttpEulaFilePath(path string) HttpOption {
	return func(s *httpd) {
		s.eulaFilePath = path
	}
}

func WithHttpTqlLoader(loader tql.Loader) HttpOption {
	return func(s *httpd) {
		s.tqlLoader = loader
	}
}

func WithHttpServerSideFileSystem(ssfs *ssfs.SSFS) HttpOption {
	return func(s *httpd) {
		s.serverFs = ssfs
	}
}

func WithHttpDebugMode(isDebug bool, filterLatency string) HttpOption {
	return func(s *httpd) {
		s.debugMode = isDebug
		if filterLatency != "" {
			s.debugLogFilterLatency, _ = time.ParseDuration(filterLatency)
		}
	}
}

func WithHttpKeepAlive(keepAlive int) HttpOption {
	return func(s *httpd) {
		s.keepAlive = keepAlive
	}
}

func WithHttpLinger(linger int) HttpOption {
	return func(s *httpd) {
		s.linger = linger
	}
}

func WithHttpReadBufSize(size int) HttpOption {
	return func(s *httpd) {
		s.readBufSize = size
	}
}

func WithHttpWriteBufSize(size int) HttpOption {
	return func(s *httpd) {
		s.writeBufSize = size
	}
}

func WithHttpWebDir(path string) HttpOption {
	return func(s *httpd) {
		s.uiContentFs = WrapAssets(path)
	}
}

// experiement features
func WithHttpExperimentModeProvider(provider func() bool) HttpOption {
	return func(s *httpd) {
		s.experimentModeProvider = provider
	}
}

func WithHttpStatzAllow(remotes ...string) HttpOption {
	return func(s *httpd) {
		addr := make([]string, 0, len(remotes))
		for _, remote := range remotes {
			list := strings.Split(remote, ",")
			for _, item := range list {
				if item == "" {
					continue
				}
				addr = append(addr, item)
			}
		}
		s.statzAllowed = append(s.statzAllowed, addr...)
	}
}

func WithHttpStatzToken(token string) HttpOption {
	return func(s *httpd) {
		s.statzToken = token
	}
}

func WithHttpQueryCypher(algAndKey string) HttpOption {
	alg := ""
	pad := "PCKCS7"
	key := ""
	pairs := util.ParseNameValuePairs(algAndKey)
	for _, p := range pairs {
		switch strings.ToLower(p.Name) {
		case "cypher", "cipher", "alg", "algorithm":
			alg = strings.ToUpper(p.Value)
		case "key":
			key = p.Value
		case "pad", "padding":
			pad = strings.ToUpper(p.Value)
		}
	}
	return func(s *httpd) {
		if alg == "" && key == "" {
			return
		}
		if err := util.ValidateCypherKey(alg, key); err != nil {
			s.log.Errorf("Invalid cypher settings, query cypher disabled: %v", err)
		} else {
			s.cypherAlg = alg
			s.cypherKey = key
			s.cypherPad = pad
			s.log.Infof("HTTP query cypher enabled (alg=%s,pad=%s)", s.cypherAlg, s.cypherPad)
		}
	}
}

func WithHttpMqttWsHandlerFunc(fn http.HandlerFunc) HttpOption {
	return func(s *httpd) {
		s.mqttWsHandler = gin.WrapF(fn)
	}
}

func WithHttpPathMap(name string, realPath string) HttpOption {
	return func(s *httpd) {
		s.pathMap[name] = realPath
	}
}

func (svr *httpd) Start() error {
	svr.alive = true

	if svr.debugMode {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	if svr.statzToken == "" {
		spi.SetPrometheusBearerToken(svr.statzToken)
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
	svr.debugModeLock.RLock()
	defer svr.debugModeLock.RUnlock()
	return svr.debugMode, svr.debugLogFilterLatency
}

// SetDebugMode sets the debug mode and the log filter latency
func (svr *httpd) SetDebugMode(debug bool, filterLatency time.Duration) {
	svr.debugModeLock.Lock()
	defer svr.debugModeLock.Unlock()
	svr.debugMode = debug
	if filterLatency >= 0 {
		svr.debugLogFilterLatency = filterLatency
	}
}

func (svr *httpd) Router() *gin.Engine {
	r := gin.New()
	r.Use(RecoveryWithLogging(svr.log))
	r.Use(HttpLogger("http-log", svr.DebugMode))
	r.Use(svr.corsHandler())
	r.Use(MetricsInterceptor())

	// redirect '/' -> '/web/'
	r.GET("/", func(ctx *gin.Context) {
		ctx.Redirect(http.StatusFound, "/web/")
	})
	// prefix '/metrics' for influx line protocol
	metricsGroup := r.Group("/metrics")
	if svr.enableTokenAuth && svr.authServer != nil {
		metricsGroup.Use(svr.handleAuthToken)
	}
	metricsGroup.POST("/:oper", svr.handleLineProtocol)
	svr.log.Infof("HTTP path %s for the line protocol", "/metrics")

	// prefix '/db' for machbase
	dbGroup := r.Group("/db")
	if svr.enableTokenAuth && svr.authServer != nil {
		dbGroup.Use(svr.handleAuthToken)
	}
	dbGroup.GET("/query", svr.handleQuery)
	dbGroup.POST("/query", svr.handleQuery)
	dbGroup.POST("/write", svr.handleWrite)
	dbGroup.POST("/write/:table", svr.handleWrite)
	dbGroup.GET("/query/file/:table/:column/:id", svr.handleFileQuery)
	dbGroup.GET("/watch/:table", svr.handleWatchQuery)
	dbGroup.GET("/tql/*path", svr.handleTqlFile)
	dbGroup.POST("/tql/*path", svr.handleTqlFile)
	dbGroup.GET("/tql", svr.handleTqlQuery)
	dbGroup.POST("/tql", svr.handleTqlQuery)
	svr.log.Infof("HTTP path %s for machbase api", "/db")

	// prefix '/web' for web ui
	webGroup := r.Group("/web")
	contentBase := "/ui/"
	webGroup.GET("/", func(ctx *gin.Context) {
		ctx.Redirect(http.StatusFound, path.Join("/web", contentBase))
	})
	if svr.uiContentFs != nil {
		webGroup.StaticFS(contentBase, svr.uiContentFs)
	} else {
		webGroup.StaticFS(contentBase, GetAssets(contentBase))
	}
	webGroup.Any("/api/license/eula", svr.handleEula)
	webGroup.POST("/api/login", svr.handleLogin)
	webGroup.GET("/api/term/:term_id/data", svr.handleTermData)
	webGroup.GET("/api/console/:console_id/data", svr.handleConsoleData)
	if svr.mqttWsHandler != nil {
		webGroup.GET("/api/mqtt", svr.mqttWsHandler)
		svr.log.Infof("MQTT websocket handler enabled")
	}
	if svr.tqlLoader != nil {
		webGroup.GET("/api/tql-assets/*path", gin.WrapH(http.FileServer(tql.HttpFileSystem())))
	}
	webGroup.GET("/api/tql-exec", svr.handleTqlQueryExec)
	webGroup.Any("/services/*path", svr.handleServiceProxy)
	webGroup.Use(svr.handleJwtToken)
	webGroup.POST("/api/term/:term_id/windowsize", svr.handleTermWindowSize)
	webGroup.GET("/api/tql/*path", svr.handleTqlFile)
	webGroup.POST("/api/tql/*path", svr.handleTqlFile)
	webGroup.GET("/api/tql", svr.handleTqlQuery)
	webGroup.POST("/api/tql", svr.handleTqlQuery)
	webGroup.Any("/machbase", func(c *gin.Context) {
		svr.log.Debugf("/web/api/machbase is deprecated, use /web/api/query")
		svr.handleQuery(c)
	})
	webGroup.Any("/api/query", svr.handleQuery)
	webGroup.GET("/api/check", svr.handleCheck)
	webGroup.POST("/api/rpc", svr.handleHttpRpc)
	webGroup.POST("/api/relogin", svr.handleReLogin)
	webGroup.POST("/api/logout", svr.handleLogout)
	webGroup.POST("/api/chpasswd", svr.handleChangePassword)
	webGroup.GET("/api/timers/:name", svr.handleTimer)
	webGroup.PUT("/api/timers/:name", svr.handleTimersUpdate)
	webGroup.GET("/api/subscribers/:name", svr.handleSubscriber)
	webGroup.GET("/api/tables", svr.handleTables)
	webGroup.GET("/api/tables/:table/tags", svr.handleTags)
	webGroup.GET("/api/tables/:table/tags/:tag/stat", svr.handleTagStat)
	webGroup.Any("/api/files/*path", svr.handleFiles)
	webGroup.GET("/api/refs/*path", svr.handleRefs)
	webGroup.GET("/api/license", svr.handleGetLicense)
	webGroup.POST("/api/license", svr.handleInstallLicense)
	webGroup.Any("/api/statz/config", svr.handleStatzConfig)
	if svr.authServer != nil && svr.authServer.bakd != nil {
		svr.authServer.bakd.HttpRouter(webGroup.Group("/api/backup"))
	}
	svr.log.Infof("HTTP path %s for the web ui", "/web")

	// prefix '/public' for public files
	r.Any("/public/*path", svr.handlePublic)

	// debug group
	debugGroup := r.Group("/debug")
	debugGroup.Use(svr.allowDebug)
	debugGroup.Any("/pprof/*path", gin.WrapF(httpPprof.Index))
	debugGroup.GET("/dashboard", gin.WrapF(spi.DashboardHandler()))
	debugGroup.GET("/statz", gin.WrapF(spi.HandleStatz))
	debugGroup.GET("/metrics", gin.WrapF(spi.HandlePrometheusMetrics))

	r.NoRoute(gin.WrapH(http.FileServer(AssetsDir())))
	return r
}

func (svr *httpd) getUserSqlConn(ctx *gin.Context) (*sql.Conn, error) {
	claim, _ := svr.getJwtClaim(ctx)
	if claim != nil {
		return spi.Connect(ctx, claim.Subject)
	}
	return nil, errors.New("unauthorized db request")
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

func (svr *httpd) issueAccessTokenUsername(username spi.UserName) (string, string, string, error) {
	realUser := username.Login
	if username.Proxy != "" {
		realUser = username.Proxy
	}
	return svr.issueAccessToken(realUser)
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

type ChangePasswordReq struct {
	NewPassword string `json:"newPassword"`
}

type ChangePasswordRsp struct {
	Success bool   `json:"success"`
	Reason  string `json:"reason"`
	Elapse  string `json:"elapse"`
}

func (svr *httpd) handleChangePassword(ctx *gin.Context) {
	var req = &ChangePasswordReq{}
	var rsp = &ChangePasswordRsp{
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

	if len(req.NewPassword) == 0 || strings.Contains(req.NewPassword, "'") {
		rsp.Reason = "invalid new password"
		rsp.Elapse = time.Since(tick).String()
		ctx.JSON(http.StatusBadRequest, rsp)
		return
	}

	claim, ok := svr.getJwtClaim(ctx)
	if !ok {
		rsp.Reason = "unauthorized request"
		rsp.Elapse = time.Since(tick).String()
		ctx.JSON(http.StatusUnauthorized, rsp)
		return
	}

	conn, err := spi.Connect(ctx, claim.Subject)
	if err != nil {
		rsp.Reason = err.Error()
		rsp.Elapse = time.Since(tick).String()
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	}
	defer conn.Close()

	_, err = conn.ExecContext(ctx,
		fmt.Sprintf("ALTER USER %s IDENTIFIED BY '%s'", claim.Subject, req.NewPassword))
	if err != nil {
		rsp.Reason = err.Error()
		rsp.Elapse = time.Since(tick).String()
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	}

	rsp.Success = true
	rsp.Reason = "success"
	rsp.Elapse = time.Since(tick).String()

	ctx.JSON(http.StatusOK, rsp)
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
	username, proxyed := spi.ParseUserName(req.LoginName)
	if username.Proxy != "" && !proxyed {
		rsp.Reason = "proxy login is not allowed"
		rsp.Elapse = time.Since(tick).String()
		ctx.JSON(http.StatusBadRequest, rsp)
		return
	}
	passed, reason, err := svr.authServer.ValidateUserPassword(ctx, username.Login, req.Password)
	if err != nil {
		svr.log.Warnf("database auth failed %s", err.Error())
		rsp.Reason = "database error for user authentication"
		rsp.Elapse = time.Since(tick).String()
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	}

	if !passed {
		svr.log.Tracef("'%s' login fail password mis-matched", username.Login)
		rsp.Reason = reason
		rsp.Elapse = time.Since(tick).String()
		ctx.JSON(http.StatusNotFound, rsp)
		return
	}

	accessToken, refreshToken, refreshTokenId, err := svr.issueAccessTokenUsername(username)
	if err != nil {
		rsp.Reason = err.Error()
		rsp.Elapse = time.Since(tick).String()
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	}

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

	rsp.Success, rsp.Reason = true, "success"
	rsp.Elapse = time.Since(tick).String()
	ctx.JSON(http.StatusOK, rsp)
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

//go:embed eula.txt
var eulaTxt string

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
		if conn, err := spi.Connect(ctx, "sys"); err == nil {
			if nfo, err := spi.GetLicenseInfo(ctx, conn); err == nil {
				svr.licenseStatus = nfo.LicenseStatus
			}
			defer conn.Close()
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
	if svr.authServer != nil && svr.authServer.models != nil {
		rsp.Shells = svr.authServer.models.GetAllShells(true)
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

func (svr *httpd) handleStatzConfig(ctx *gin.Context) {
	tick := time.Now()
	switch ctx.Request.Method {
	case http.MethodGet:
		ctx.JSON(http.StatusOK, map[string]any{
			"success": true,
			"reason":  "success",
			"elapse":  time.Since(tick).String(),
			"data": map[string]any{
				"out": spi.MetricsDestTable(),
			},
		})
	case http.MethodPost:
		obj := map[string]any{}
		if err := ctx.Bind(&obj); err != nil {
			ctx.JSON(http.StatusBadRequest, map[string]any{
				"success": false,
				"reason":  err.Error(),
				"elapse":  time.Since(tick).String(),
			})
		} else {
			if out, ok := obj["out"].(string); !ok {
				ctx.JSON(http.StatusBadRequest, map[string]any{
					"success": false,
					"reason":  "invalid out value",
					"elapse":  time.Since(tick).String(),
				})
			} else {
				if err := spi.SetMetricsDestTable(out); err != nil {
					ctx.JSON(http.StatusInternalServerError, map[string]any{
						"success": false,
						"reason":  err.Error(),
						"elapse":  time.Since(tick).String(),
					})
				} else {
					ctx.JSON(http.StatusOK, map[string]any{
						"success": true,
						"reason":  "success",
						"elapse":  time.Since(tick).String(),
					})
				}
			}
		}
	default:
		ctx.String(http.StatusMethodNotAllowed, "")
	}
}

type LicenseResponse struct {
	Success bool             `json:"success"`
	Reason  string           `json:"reason"`
	Elapse  string           `json:"elapse"`
	Data    *spi.LicenseInfo `json:"data,omitempty"`
}

func (svr *httpd) handleGetLicense(ctx *gin.Context) {
	rsp := &LicenseResponse{Success: false, Reason: "unspecified"}
	tick := time.Now()

	conn, err := spi.Connect(ctx, "sys")
	if err != nil {
		rsp.Reason = err.Error()
		ctx.JSON(http.StatusUnauthorized, rsp)
		return
	}
	defer conn.Close()

	nfo, err := spi.GetLicenseInfo(ctx, conn)
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

	claim, _ := svr.getJwtClaim(ctx)
	if claim == nil || claim.Valid() != nil || claim.Subject != "sys" {
		rsp.Reason = "unauthorized"
		rsp.Elapse = time.Since(tick).String()
		ctx.JSON(http.StatusUnauthorized, rsp)
		return
	}

	conn, err := spi.Connect(ctx, "sys")
	if err != nil {
		rsp.Reason = err.Error()
		ctx.JSON(http.StatusUnauthorized, rsp)
		return
	}
	defer conn.Close()

	nfo, err := spi.InstallLicenseData(ctx, conn, svr.licenseFilePath, content)
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

func (svr *httpd) handleTimer(ctx *gin.Context) {
	tick := time.Now()
	rsp := gin.H{"success": false, "reason": "not specified"}

	name := ctx.Param("name")
	getRsp, err := svr.authServer.schedSvc.GetSchedule(ctx, &scheduler.GetScheduleRequest{
		Name: name,
	})
	if err != nil {
		rsp["reason"] = err.Error()
		rsp["elapse"] = time.Since(tick).String()
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	}
	if !getRsp.Success {
		rsp["reason"] = getRsp.Reason
		rsp["elapse"] = time.Since(tick).String()
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	}

	rsp["success"] = true
	rsp["reason"] = "success"
	rsp["data"] = getRsp.Schedule
	rsp["elapse"] = time.Since(tick).String()
	ctx.JSON(http.StatusOK, rsp)
}

func (svr *httpd) handleTimersUpdate(ctx *gin.Context) {
	tick := time.Now()
	rsp := gin.H{"success": false, "reason": "not specified"}
	req := struct {
		AutoStart bool   `json:"autoStart"`
		Schedule  string `json:"schedule"`
		Path      string `json:"path"`
	}{}

	name := ctx.Param("name")
	if name == "" {
		rsp["reason"] = "no name specified"
		rsp["elapse"] = time.Since(tick).String()
		ctx.JSON(http.StatusBadRequest, rsp)
		return
	}

	err := ctx.ShouldBind(&req)
	if err != nil {
		rsp["reason"] = err.Error()
		rsp["elapse"] = time.Since(tick).String()
		ctx.JSON(http.StatusBadRequest, rsp)
		return
	}

	getRsp, err := svr.authServer.schedSvc.GetSchedule(ctx, &scheduler.GetScheduleRequest{
		Name: name,
	})
	if err != nil {
		rsp["reason"] = err.Error()
		rsp["elapse"] = time.Since(tick).String()
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	}
	if !getRsp.Success {
		rsp["reason"] = getRsp.Reason
		rsp["elapse"] = time.Since(tick).String()
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	}

	updateRsp, err := svr.authServer.schedSvc.UpdateSchedule(ctx, &scheduler.UpdateScheduleRequest{
		Name:      name,
		AutoStart: req.AutoStart,
		Schedule:  req.Schedule,
		Task:      req.Path,
	})
	if err != nil {
		rsp["reason"] = err.Error()
		rsp["elapse"] = time.Since(tick).String()
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	}
	if !updateRsp.Success {
		rsp["reason"] = updateRsp.Reason
		rsp["elapse"] = time.Since(tick).String()
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	}

	rsp["success"] = true
	rsp["reason"] = "success"
	rsp["elapse"] = time.Since(tick).String()
	ctx.JSON(http.StatusOK, rsp)
}

func (svr *httpd) handleSubscriber(ctx *gin.Context) {
	tick := time.Now()
	rsp := gin.H{"success": false, "reason": "not specified"}

	name := ctx.Param("name")
	getRsp, err := svr.authServer.schedSvc.GetSchedule(ctx, &scheduler.GetScheduleRequest{Name: name})
	if err != nil {
		rsp["reason"] = err.Error()
		rsp["elapse"] = time.Since(tick).String()
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	}
	if !getRsp.Success {
		rsp["reason"] = getRsp.Reason
		rsp["elapse"] = time.Since(tick).String()
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	}

	rsp["success"] = true
	rsp["reason"] = "success"
	rsp["data"] = getRsp.Schedule
	rsp["elapse"] = time.Since(tick).String()
	ctx.JSON(http.StatusOK, rsp)
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
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

	cons := NewWebConsole(claim.Subject, consoleId, conn, svr.serviceController)
	cons.Run()
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
	termAddress := svr.authServer.neoShellAddress
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

	_, _, err = net.SplitHostPort(termAddress)
	if err != nil {
		svr.log.Warnf("term invalid address %s", err.Error())
		ctx.String(http.StatusInternalServerError, err.Error())
		return
	}

	// user name other than sys
	if low := strings.ToLower(termLoginName); low != "sys" {
		termLoginName = "sys as " + low
	}

	term, err := NewWebTerm(termAddress, userShell, termLoginName)
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

func (svr *httpd) handleTermWindowSize(ctx *gin.Context) {
	termId := ctx.Param("term_id")

	claimAny, claimExists := ctx.Get("jwt-claim")
	if !claimExists {
		ctx.String(http.StatusUnauthorized, "unauthorized access")
		return
	}
	claim := claimAny.(Claim)
	termLoginName := claim.Subject

	req := &struct {
		Rows int `query:"rows" form:"rows" json:"rows"`
		Cols int `query:"cols" form:"cols" json:"cols"`
	}{}
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

func NewWebTerm(hostPort string, userShell string, user string) (*WebTerm, error) {
	var loginString string
	if len(userShell) > 0 {
		loginString = fmt.Sprintf("%s:%s", user, userShell)
	} else {
		loginString = user
	}
	privateKey := spi.DefaultKey()
	signer, err := ssh.NewSignerFromKey(privateKey)
	if err != nil {
		return nil, fmt.Errorf("NewTerm signer, %s", err.Error())
	}
	conn, err := ssh.Dial("tcp", hostPort, &ssh.ClientConfig{
		User: loginString,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
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

type WebConsoleProcessor interface {
	Process(ctx context.Context, line string)
	Input(line string)
	Control(ctrl string)
}

type WebConsole struct {
	log       logging.Log
	username  string
	consoleId string
	topic     string
	conn      *websocket.Conn
	connMutex sync.Mutex
	closeOnce sync.Once
	closed    atomic.Bool

	messages          []*eventbus.Event
	lastFlushTime     time.Time
	flushPeriod       time.Duration
	processor         WebConsoleProcessor
	serviceController *service.Controller
}

type webConsoleRpcNotifier struct {
	cons *WebConsole
}

func (n *webConsoleRpcNotifier) NotifyJsonRpc(session string, payload map[string]any) error {
	if n == nil || n.cons == nil {
		return nil
	}
	n.cons.connMutex.Lock()
	defer n.cons.connMutex.Unlock()
	return n.cons.conn.WriteJSON(map[string]any{
		"type":    eventbus.EVT_RPC_RSP,
		"session": session,
		"rpc":     payload,
	})
}

func NewWebConsole(username string, consoleId string, conn *websocket.Conn, serviceController *service.Controller) *WebConsole {
	if serviceController == nil {
		serviceController = defaultJsonRpcController
	}
	ret := &WebConsole{
		log:               logging.GetLog(fmt.Sprintf("console-%s-%s", username, consoleId)),
		topic:             fmt.Sprintf("console:%s:%s", username, consoleId),
		username:          username,
		consoleId:         consoleId,
		conn:              conn,
		lastFlushTime:     time.Now(),
		flushPeriod:       300 * time.Millisecond,
		serviceController: serviceController,
	}
	eventbus.Default.SubscribeAsync(ret.topic, ret.Send, true)
	return ret
}

func (cons *WebConsole) Run() {
	go cons.readerLoop()
	go cons.flushLoop()
}

func (cons *WebConsole) Close() {
	cons.closeOnce.Do(func() {
		cons.closed.Store(true)
		eventbus.Default.Unsubscribe(cons.topic, cons.Send)
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

	if cons.log.TraceEnabled() {
		cons.log.Trace("websocket: established", cons.conn.RemoteAddr().String())
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
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
			if evt.Ping != nil {
				cons.handlePing(ctx, evt.Ping)
			}
		case eventbus.EVT_RPC_REQ:
			if evt.Rpc != nil {
				go cons.handleRpc(ctx, evt.Session, evt.Rpc)
			}
		}
	}
}

func (cons *WebConsole) flushLoop() {
	ticker := time.NewTicker(cons.flushPeriod)
	for range ticker.C {
		if cons.closed.Load() {
			break
		}
		cons.Send(nil)
	}
	ticker.Stop()
}

func (cons *WebConsole) Send(evt *eventbus.Event) {
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

func (cons *WebConsole) handlePing(_ context.Context, evt *eventbus.Ping) {
	rsp := eventbus.NewPing(evt.Tick)
	cons.connMutex.Lock()
	cons.conn.WriteJSON(rsp)
	cons.connMutex.Unlock()
}

func (cons *WebConsole) handleRpc(ctx context.Context, session string, evt *eventbus.RPC) {
	rpcCtx := service.WithJsonRpcNotificationWriter(ctx, &webConsoleRpcNotifier{cons: cons})
	rpcCtx = service.WithJsonRpcSession(rpcCtx, session)

	rsp := map[string]any{
		"jsonrpc": "2.0",
		"id":      evt.ID,
	}
	result, rpcErr := cons.serviceController.CallJsonRpc(evt.Method, evt.Params, func(paramType reflect.Type) (reflect.Value, bool) {
		switch {
		case paramType == webConsoleType:
			return reflect.ValueOf(cons), true
		case paramType == contextType:
			return reflect.ValueOf(rpcCtx), true
		default:
			return reflect.Value{}, false
		}
	})
	if rpcErr == nil {
		rsp["result"] = result
	} else {
		code := rpcErr.Code
		message := rpcErr.Message
		if code == -32603 {
			code = -32000
		}
		if rpcErr.Code == -32601 {
			message = "Method not found"
		}
		rsp["error"] = map[string]any{
			"code":    code,
			"message": message,
		}
	}
	cons.connMutex.Lock()
	cons.conn.WriteJSON(map[string]any{
		"type":    eventbus.EVT_RPC_RSP,
		"session": session,
		"rpc":     rsp,
	})
	cons.connMutex.Unlock()
}

//go:embed assets/*
var assetsDir embed.FS

func AssetsDir() http.FileSystem {
	return &StaticFSWrap{
		TrimPrefix:      "/web/",
		PrependRealPath: "/assets/",
		Base:            http.FS(assetsDir),
		FixedModTime:    time.Now(),
	}
}

type StaticFSWrap struct {
	TrimPrefix      string
	PrependRealPath string
	Base            http.FileSystem
	FixedModTime    time.Time
}

type staticFile struct {
	http.File
	modTime time.Time
}

func (fsw *StaticFSWrap) Open(name string) (http.File, error) {
	if !strings.HasPrefix(name, fsw.TrimPrefix) {
		name = strings.TrimSuffix(fsw.TrimPrefix, "/") + "/" + strings.TrimPrefix(name, "/")
	}
	f, err := fsw.Base.Open(fsw.PrependRealPath + strings.TrimPrefix(name, fsw.TrimPrefix))
	if err != nil {
		return nil, err
	}
	return &staticFile{File: f, modTime: fsw.FixedModTime}, nil
}

func (f *staticFile) Stat() (fs.FileInfo, error) {
	stat, err := f.File.Stat()
	if err != nil {
		return nil, err
	}
	return &staticStat{FileInfo: stat, modTime: f.modTime}, nil
}

func (f *staticFile) ModTime() time.Time {
	return f.modTime
}

type staticStat struct {
	fs.FileInfo
	modTime time.Time
}

func (stat *staticStat) ModTime() time.Time {
	return stat.modTime
}

func WrapAssets(path string) http.FileSystem {
	fs := http.Dir(path)
	ret := &wrapFileSystem{base: fs}
	return ret
}

type wrapFileSystem struct {
	base http.FileSystem
}

func (fs *wrapFileSystem) Open(name string) (http.File, error) {
	ret, err := fs.base.Open(name)
	if err == nil {
		return ret, nil
	}
	return fs.base.Open("index.html")
}

//go:embed web/*
var webFs embed.FS

func GetAssets(dir string) http.FileSystem {
	dir = strings.TrimPrefix(strings.TrimSuffix(dir, "/"), "/")
	_, err := fs.Sub(webFs, "web/"+dir)
	if err != nil {
		panic(err)
	}

	return &assetFileSystem{
		StaticFSWrap: StaticFSWrap{
			TrimPrefix:   "",
			Base:         http.FS(webFs),
			FixedModTime: time.Now(),
		},
		prefix: "web/" + dir,
	}
}

type assetFileSystem struct {
	StaticFSWrap
	prefix string
}

func (fs *assetFileSystem) Open(name string) (http.File, error) {
	toks := strings.SplitN(name, "?", 2)
	if len(toks) == 0 {
		return nil, os.ErrNotExist
	}
	name = toks[0]
	if strings.HasSuffix(name, "/") {
		return fs.StaticFSWrap.Open(name)
	} else if isWellKnownFileType(name) {
		return fs.StaticFSWrap.Open(fs.prefix + name)
	} else {
		return fs.StaticFSWrap.Open(fs.prefix + "/index.html")
	}
}

var wellknowns = map[string]bool{
	".css":   true,
	".gif":   true,
	".html":  true,
	".htm":   true,
	".ico":   true,
	".jpg":   true,
	".jpeg":  true,
	".json":  true,
	".js":    true,
	".map":   true,
	".png":   true,
	".svg":   true,
	".ttf":   true,
	".txt":   true,
	".yaml":  true,
	".yml":   true,
	".webp":  true,
	".woff":  true,
	".woff2": true,
}

func isWellKnownFileType(name string) bool {
	ext := filepath.Ext(name)
	if len(ext) == 0 {
		return false
	}

	if _, ok := wellknowns[strings.ToLower(ext)]; ok {
		return true
	}
	return false
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

func (svr *httpd) handleRefs(ctx *gin.Context) {
	rsp := &RefsResponse{Reason: "unspecified"}
	tick := time.Now()
	path := ctx.Param("path")

	if path == "/" {
		references := &WebReferenceGroup{Label: "REFERENCES"}
		references.Items = append(references.Items, ReferenceItem{Type: "url", Title: "machbase-neo docs", Addr: "https://docs.machbase.com/neo", Target: "_blank"})
		references.Items = append(references.Items, ReferenceItem{Type: "url", Title: "machbase sql reference", Addr: "https://docs.machbase.com/dbms/sql-reference/", Target: "_docs_machbase"})
		references.Items = append(references.Items, ReferenceItem{Type: "url", Title: "https://machbase.com", Addr: "https://machbase.com/", Target: "_home_machbase"})
		references.Items = append(references.Items, ReferenceItem{Type: "url", Title: "Tutorials", Addr: "https://github.com/machbase/neo-tutorials", Target: "_blank"})
		references.Items = append(references.Items, ReferenceItem{Type: "url", Title: "Demo web app", Addr: "https://github.com/machbase/neo-apps"})

		sdk := &WebReferenceGroup{Label: "SDK"}
		sdk.Items = append(sdk.Items, ReferenceItem{Type: "url", Title: "SDK", Addr: "https://docs.machbase.com/dbms/sdk-integration/", Target: "_docs_machbase"})
		sdk.Items = append(sdk.Items, ReferenceItem{Type: "url", Title: ".NET Connector", Addr: "https://www.nuget.org/packages/UniMachNetConnector", Target: "_blank"})
		sdk.Items = append(sdk.Items, ReferenceItem{Type: "url", Title: "Python", Addr: "https://pypi.org/project/machbaseapi/", Target: "_blank"})
		sdk.Items = append(sdk.Items, ReferenceItem{Type: "url", Title: "Node.js", Addr: "https://www.npmjs.com/package/@machbase/ts-client", Target: "_blank"})
		sdk.Items = append(sdk.Items, ReferenceItem{Type: "url", Title: "Go", Addr: "https://github.com/machbase/neo-client", Target: "_blank"})

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

var (
	ginContextType = reflect.TypeOf((*gin.Context)(nil))
	contextType    = reflect.TypeOf((*context.Context)(nil)).Elem()
	webConsoleType = reflect.TypeOf((*WebConsole)(nil))
)

type rpcImplicitParamResolver func(paramType reflect.Type) (reflect.Value, bool)

var defaultJsonRpcController = &service.Controller{}

func buildRpcCallParams(handler any, rawParams []any, resolveImplicit rpcImplicitParamResolver) ([]reflect.Value, error) {
	return service.BuildRpcCallParams(handler, rawParams, service.JsonRpcImplicitParamResolver(resolveImplicit))
}

var (
	mdFileRootRegexp = regexp.MustCompile(`{{\s*file_root\s*}}`)
	mdFilePathRegexp = regexp.MustCompile(`{{\s*file_path\s*}}`)
	mdFileNameRegexp = regexp.MustCompile(`{{\s*file_name\s*}}`)
	mdFileDirRegexp  = regexp.MustCompile(`{{\s*file_dir\s*}}`)
)

// rpcMarkdownRender renders markdown to HTML.
//
// params:
//   - markdown: markdown source text
//   - darkMode: whether to render with dark-mode style
//   - referer: the referer URL
//     "http://127.0.0.1:5654/web/api/tql/sample_image.wrk" // if file has been saved
//     "http://127.0.0.1:5654/web/ui" // file is not saved
//
// return: rendered HTML text
func rpcMarkdownRender(markdown string, darkMode bool, referer string) (string, error) {
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
	fileRoot := "/web/api/tql"
	src := []byte(markdown)
	src = mdFileRootRegexp.ReplaceAll(src, []byte(fileRoot))
	src = mdFilePathRegexp.ReplaceAll(src, []byte(filePath))
	src = mdFileNameRegexp.ReplaceAll(src, []byte(fileName))
	src = mdFileDirRegexp.ReplaceAll(src, []byte(fileDir))

	conv := mdconv.New(mdconv.WithDarkMode(darkMode))
	w := &strings.Builder{}
	w.Write([]byte(`<div>`))
	if err := conv.Convert(src, w); err != nil {
		return "", err
	}
	w.Write([]byte(`</div>`))
	return w.String(), nil
}

// handleHttpRpc handles HTTP POST requests for JSON-RPC
func (svr *httpd) handleHttpRpc(ctx *gin.Context) {
	var req eventbus.RPC

	// Parse JSON-RPC request
	if err := ctx.ShouldBindJSON(&req); err != nil {
		// Invalid JSON-RPC request format
		rsp := map[string]any{
			"jsonrpc": "2.0",
			"id":      nil,
			"error": map[string]any{
				"code":    -32700,
				"message": "Parse error",
			},
		}
		ctx.JSON(http.StatusOK, rsp)
		return
	}

	rsp := map[string]any{
		"jsonrpc": "2.0",
		"id":      req.ID,
	}

	ctl := svr.serviceController
	if ctl == nil {
		ctl = defaultJsonRpcController
	}
	result, rpcErr := ctl.CallJsonRpc(req.Method, req.Params, func(paramType reflect.Type) (reflect.Value, bool) {
		switch {
		case paramType == ginContextType:
			return reflect.ValueOf(ctx), true
		case paramType == contextType:
			// Pass gin.Context as context.Context to preserve requester information.
			return reflect.ValueOf(ctx), true
		default:
			return reflect.Value{}, false
		}
	})
	if rpcErr == nil {
		rsp["result"] = result
	} else {
		code := rpcErr.Code
		message := rpcErr.Message
		if code == -32603 {
			code = -32000
		}
		if rpcErr.Code == -32601 {
			message = "Method not found"
		}
		rsp["error"] = map[string]any{
			"code":    code,
			"message": message,
		}
	}

	// Always return HTTP 200 as per JSON-RPC 2.0 specification
	ctx.JSON(http.StatusOK, rsp)
}

func strBool(str string, def bool) bool {
	if str == "" {
		return def
	}
	return strings.ToLower(str) == "true" || str == "1"
}

func strInt(str string, def int) int {
	if str == "" {
		return def
	}
	v, err := strconv.Atoi(str)
	if err != nil {
		return def
	}
	return v
}

func strString(str string, def string) string {
	if str == "" {
		return def
	}
	return str
}

func MetricsInterceptor() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		latency := time.Since(start)
		m := []metric.Measure{}
		m = append(m, metric.Measure{Name: "http:count", Value: 1, Type: metric.CounterType(metric.UnitShort)})
		m = append(m, metric.Measure{Name: "http:latency", Value: float64(latency.Nanoseconds()), Type: metric.HistogramType(metric.UnitDuration)})
		if strings.HasPrefix(c.Request.URL.Path, "/db/write") {
			m = append(m, metric.Measure{Name: "http:write:count", Value: 1, Type: metric.CounterType(metric.UnitShort)})
			m = append(m, metric.Measure{Name: "http:write:latency", Value: float64(latency.Nanoseconds()), Type: metric.HistogramType(metric.UnitDuration)})
		} else if strings.HasPrefix(c.Request.URL.Path, "/db/query") {
			m = append(m, metric.Measure{Name: "http:query:count", Value: 1, Type: metric.CounterType(metric.UnitShort)})
			m = append(m, metric.Measure{Name: "http:query:latency", Value: float64(latency.Nanoseconds()), Type: metric.HistogramType(metric.UnitDuration)})
		} else if strings.HasPrefix(c.Request.URL.Path, "/db/tql") {
			m = append(m, metric.Measure{Name: "http:tql:count", Value: 1, Type: metric.CounterType(metric.UnitShort)})
			m = append(m, metric.Measure{Name: "http:tql:latency", Value: float64(latency.Nanoseconds()), Type: metric.HistogramType(metric.UnitDuration)})
		}
		if s := c.Request.ContentLength; s > 0 {
			m = append(m, metric.Measure{Name: "http:recv_bytes", Value: float64(s), Type: metric.CounterType(metric.UnitBytes)})
		}
		if s := c.Writer.Size(); s > 0 {
			m = append(m, metric.Measure{Name: "http:send_bytes", Value: float64(s), Type: metric.CounterType(metric.UnitBytes)})
		}

		status := c.Writer.Status()
		if status < 200 {
			m = append(m, metric.Measure{Name: "http:status_1xx", Value: 1, Type: metric.CounterType(metric.UnitShort)})
		} else if status < 300 {
			m = append(m, metric.Measure{Name: "http:status_2xx", Value: 1, Type: metric.CounterType(metric.UnitShort)})
		} else if status < 400 {
			m = append(m, metric.Measure{Name: "http:status_3xx", Value: 1, Type: metric.CounterType(metric.UnitShort)})
		} else if status < 500 {
			m = append(m, metric.Measure{Name: "http:status_4xx", Value: 1, Type: metric.CounterType(metric.UnitShort)})
		} else {
			m = append(m, metric.Measure{Name: "http:status_5xx", Value: 1, Type: metric.CounterType(metric.UnitShort)})
		}
		spi.AddMetrics(m...)
	}
}

func RecoveryWithLogging(log logging.Log, recovery ...gin.RecoveryFunc) gin.HandlerFunc {
	gin.DefaultWriter = log
	gin.DefaultErrorWriter = log

	if len(recovery) > 0 {
		return gin.CustomRecoveryWithWriter(log, recovery[0])
	}
	return gin.CustomRecoveryWithWriter(log, func(c *gin.Context, err any) {
		c.AbortWithStatus(http.StatusInternalServerError)
	})
}

type HttpLoggerFilter func(req *http.Request, statusCode int, latency time.Duration) bool

var httpLoggerNewLogFile = logging.NewLogFile

func HttpLogger(loggingName string, debugMode func() (bool, time.Duration)) gin.HandlerFunc {
	return HttpLoggerWithFilter(loggingName, func(req *http.Request, statusCode int, latency time.Duration) bool {
		enabled := false
		threshold := time.Duration(-1)
		if debugMode != nil {
			enabled, threshold = debugMode()
		}
		// when log is disabled
		if !enabled {
			return false
		}
		// when status code is error
		if statusCode >= 400 {
			return true
		}
		// when logLatencyThreshold is not set
		if threshold < 0 {
			return false
		}

		// when logLatencyThreshold is set
		return latency >= threshold
	})
}

func HttpLoggerWithFilter(loggingName string, filter HttpLoggerFilter) gin.HandlerFunc {
	log := logging.GetLog(loggingName)
	return logger(log, filter)
}

func HttpLoggerWithFile(loggingName string, filename string) gin.HandlerFunc {
	return HttpLoggerWithFileConf(loggingName,
		logging.LogFileConf{
			Filename:             filename,
			Level:                "DEBUG",
			MaxSize:              10,
			MaxBackups:           2,
			MaxAge:               7,
			Compress:             false,
			Append:               true,
			RotateSchedule:       "@midnight",
			Console:              false,
			PrefixWidth:          20,
			EnableSourceLocation: false,
		})
}

func HttpLoggerWithFileConf(loggingName string, fileConf logging.LogFileConf) gin.HandlerFunc {
	return HttpLoggerWithFilterAndFileConf(loggingName, nil, fileConf)
}

func HttpLoggerWithFilterAndFileConf(loggingName string, filter HttpLoggerFilter, fileConf logging.LogFileConf) gin.HandlerFunc {
	if len(fileConf.Filename) > 0 {
		return logger(httpLoggerNewLogFile(loggingName, fileConf), filter)
	} else {
		return HttpLoggerWithFilter(loggingName, filter)
	}
}

var ignoreAccessLog = []struct {
	pathSuffix string
	method     string
}{
	{pathSuffix: "/healthz", method: http.MethodGet},
	{pathSuffix: "/statz", method: http.MethodGet},
	{pathSuffix: "/web/api/check", method: http.MethodGet},
}

func logger(log logging.Log, filter HttpLoggerFilter) gin.HandlerFunc {
	return func(c *gin.Context) {

		// Start timer
		start := time.Now()

		// Process request
		c.Next()

		for _, ignore := range ignoreAccessLog {
			if c.Request.Method == ignore.method && strings.HasSuffix(c.Request.URL.Path, ignore.pathSuffix) {
				return
			}
		}

		// Stop timer
		TimeStamp := time.Now()
		Latency := TimeStamp.Sub(start)

		StatusCode := c.Writer.Status()

		// filter exists, and it returns false not to leave log
		if filter != nil && !filter(c.Request, StatusCode, Latency) {
			return
		}

		url := c.Request.Host + c.Request.URL.Path
		raw := c.Request.URL.RawQuery
		if len(raw) > 0 {
			url = url + "?" + raw
		}

		ClientIP := c.ClientIP()
		Proto := c.Request.Proto
		Method := c.Request.Method
		ErrorMessage := c.Errors.ByType(gin.ErrorTypePrivate).String()
		if len(ErrorMessage) > 0 {
			ErrorMessage = "\n" + ErrorMessage
		}

		wSize := c.Writer.Size()
		if wSize == -1 {
			wSize = 0
		}
		WriteSize := util.HumanizeByteCount(int64(wSize))
		ReadSize := util.HumanizeByteCount(c.Request.ContentLength)

		color := ""
		reset := "\033[0m"
		level := logging.LevelDebug

		switch {
		case StatusCode >= http.StatusContinue && StatusCode < http.StatusOK:
			color, reset = "", "" // 1xx
		case StatusCode >= http.StatusOK && StatusCode < http.StatusMultipleChoices:
			color = "\033[97;42m" // 2xx green
		case StatusCode >= http.StatusMultipleChoices && StatusCode < http.StatusBadRequest:
			color = "\033[90;47m" // 3xx white
		case StatusCode >= http.StatusBadRequest && StatusCode < http.StatusInternalServerError:
			color = "\033[90;43m" // 4xx yellow
		default:
			color = "\033[97;41m" // 5xx red
			level = logging.LevelError
		}

		log.Logf(level, "%s %3d %s| %13v | %15s | %8s | %8s | %s %-7s %s%s",
			color, StatusCode, reset,
			Latency,
			ClientIP,
			ReadSize,
			WriteSize,
			Proto,
			Method,
			url,
			ErrorMessage,
		)
	}
}

type WsReadWriter struct {
	*websocket.Conn
	r  io.Reader
	mu sync.Mutex
}

var _ io.ReadWriter = (*WsReadWriter)(nil)

func (ws *WsReadWriter) Read(p []byte) (int, error) {
	if ws.r == nil {
		if _, r, err := ws.NextReader(); err != nil {
			return 0, err
		} else {
			ws.r = r
		}
	}
	n, err := ws.r.Read(p)
	if err == io.EOF {
		if _, r, err := ws.NextReader(); err != nil {
			return 0, err
		} else {
			ws.r = r
		}
		m, e := ws.r.Read(p[n:])
		n += m
		err = e
	}
	return n, err
}

func (ws *WsReadWriter) Write(data []byte) (int, error) {
	ws.mu.Lock()
	defer ws.mu.Unlock()
	err := (*ws).WriteMessage(websocket.BinaryMessage, data)
	if err != nil {
		return 0, err
	}
	return len(data), nil
}

func (svr *httpd) handleServiceProxy(ctx *gin.Context) {
	if svr.authServer == nil || svr.authServer.proxyMgr == nil {
		ctx.JSON(404, gin.H{"success": false, "reason": "proxy not registered"})
		return
	}
	svr.authServer.proxyMgr.Handle(ctx, ctx.Param("path"))
}
