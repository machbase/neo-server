package session

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/dop251/goja"
	"github.com/machbase/neo-server/v8/mods/model"
	"github.com/machbase/neo-server/v8/spi"
)

func Module(_ context.Context, rt *goja.Runtime, module *goja.Object) {
	exports := module.Get("exports").(*goja.Object)
	exports.Set("getHttpConfig", GetHttpConfig)
	exports.Set("setHttpToken", SetHttpToken)
	exports.Set("getHttpAccessToken", GetHttpAccessToken)
	exports.Set("getHttpRefreshToken", GetHttpRefreshToken)
	exports.Set("getMachCliConfig", GetMachCliConfig)
	exports.Set("switchUser", SwitchUser)
	exports.Set("reconnect", Reconnect)
	exports.Set("useDatabase", UseDatabase)
	exports.Set("getCurrentDatabase", GetCurrentDatabase)
}

// UserScopeContext returns a copy of ctx that resolves model.UserScope from
// the live defaultSession.User on every lookup (via model.ContextWithUserScopeFunc),
// so jsh native modules (e.g. @jsh/db, @jsh/publisher) see the session's
// current user even after it changes mid-session via SwitchUser/Reconnect
// (the `connect`/`login` shell commands).
func UserScopeContext(ctx context.Context) context.Context {
	return model.ContextWithUserScopeFunc(ctx, func() model.UserScope {
		return model.UserScope{User: defaultSession.User}
	})
}

type Config struct {
	Server       string
	User         string
	Password     string
	IdentityFile string

	// Database is the machbase database selected by the `use <database>`
	// shell command. It is propagated into every new mach connection's DSN
	// (see machcli.Config.Database) instead of running "USE .." per statement.
	Database string

	httpProto string
	httpHost  string
	httpPort  int
	httpUnix  string // for unix socket, holds socket path

	machHost string
	machPort int
	env      map[string]any

	accessToken  string
	refreshToken string
}

var ErrUserOrPasswordIncorrect = errors.New("user or password is incorrect")

var defaultSession Config

func Configure(c Config) error {
	if c.env == nil {
		c.env = map[string]any{}
	}
	httpClient := http.DefaultClient
	if strings.HasPrefix(c.Server, "unix://") {
		if socketPath, err := resolveUnixSocketPath(c.Server); err != nil {
			return err
		} else {
			c.httpUnix = socketPath
		}
		c.httpProto = "http"
		c.httpHost = "unix"
		c.httpPort = 0
		httpClient = &http.Client{
			Transport: &http.Transport{
				DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
					if strings.HasPrefix(addr, "unix:") { // e.g) addr = "unix:80"
						var dialer net.Dialer
						return dialer.DialContext(ctx, "unix", c.httpUnix)
					} else {
						var dialer net.Dialer
						return dialer.DialContext(ctx, network, addr)
					}
				},
			},
		}
	} else if h, p, err := net.SplitHostPort(c.Server); err == nil {
		c.httpProto = "http"
		c.httpHost = h
		c.httpPort, err = strconv.Atoi(p)
		if err != nil {
			return err
		}
	} else {
		return err
	}

	loginPayload := map[string]string{
		"loginName": c.User,
		"password":  c.Password,
	}
	b, _ := json.Marshal(loginPayload)
	path := buildHttpURL(c.httpProto, c.httpHost, c.httpPort, "/web/api/login")

	loginReq, err := http.NewRequest("POST", path, bytes.NewBuffer(b))
	if err != nil {
		return err
	}
	loginReq.Header.Set("Content-Type", "application/json")
	rsp, err := httpClient.Do(loginReq)
	if err != nil {
		return err
	}
	defer rsp.Body.Close()
	if rsp.StatusCode != http.StatusOK {
		if rsp.StatusCode == http.StatusNotFound {
			return ErrUserOrPasswordIncorrect
		}
		return fmt.Errorf("login failed with status code %d", rsp.StatusCode)
	}
	var rspData struct {
		AccessToken  string `json:"accessToken"`
		RefreshToken string `json:"refreshToken"`
	}
	if err := json.NewDecoder(rsp.Body).Decode(&rspData); err != nil {
		return err
	}
	c.accessToken = rspData.AccessToken
	c.refreshToken = rspData.RefreshToken

	if username, proxyed := spi.ParseUserName(c.User); username.Proxy != "" && proxyed {
		c.User = username.Proxy
	}
	rpcPayload := map[string]any{
		"jsonrpc": "2.0",
		"method":  "service.port.list",
		"params":  []any{},
		"id":      1,
	}
	b, _ = json.Marshal(rpcPayload)
	rpcPath := buildHttpURL(c.httpProto, c.httpHost, c.httpPort, "/web/api/rpc")
	rpcReq, err := http.NewRequest("POST", rpcPath, bytes.NewBuffer(b))
	if err != nil {
		return err
	}
	rpcReq.Header.Set("Content-Type", "application/json")
	rpcReq.Header.Set("Authorization", "Bearer "+c.accessToken)
	rpcRsp, err := httpClient.Do(rpcReq)
	if err != nil {
		return err
	}
	defer rpcRsp.Body.Close()
	if rpcRsp.StatusCode != http.StatusOK {
		return fmt.Errorf("service.port.list failed with status code %d", rpcRsp.StatusCode)
	}
	var rpcRspData struct {
		Result []map[string]string `json:"result"`
	}
	if err := json.NewDecoder(rpcRsp.Body).Decode(&rpcRspData); err != nil {
		return err
	}
	candidates := []HostPort{}
	serviceControllerAddr := ""
	for _, portInfo := range rpcRspData.Result {
		svc := portInfo["Service"]
		addr := portInfo["Address"]
		switch svc {
		case "mach":
			if !strings.HasPrefix(addr, "tcp://") {
				continue
			}
			addr = strings.TrimPrefix(addr, "tcp://")
			host, portStr, err := net.SplitHostPort(addr)
			if err != nil {
				continue
			}
			port, err := strconv.Atoi(portStr)
			if err != nil {
				continue
			}
			candidates = append(candidates, HostPort{Host: host, Port: port})
		case "servicectl":
			if strings.HasPrefix(addr, "unix://") || serviceControllerAddr == "" {
				serviceControllerAddr = addr
			}
		}
	}
	if serviceControllerAddr != "" {
		c.env["SERVICE_CONTROLLER"] = serviceControllerAddr
	}
	if len(candidates) == 0 {
		return errors.New("service.port.list did not return any tcp:// address for mach service")
	}

	slices.SortFunc(candidates, func(a, b HostPort) int {
		// 1. Prioritize hosts matching c.httpHost
		aIsHttpHost := a.Host == c.httpHost
		bIsHttpHost := b.Host == c.httpHost
		if aIsHttpHost != bIsHttpHost {
			if aIsHttpHost {
				return -1
			}
			return 1
		}

		// 2. Prioritize loopback addresses
		aIsLoopback := isLoopback(a.Host)
		bIsLoopback := isLoopback(b.Host)
		if aIsLoopback != bIsLoopback {
			if aIsLoopback {
				return -1
			}
			return 1
		}

		// 3. Otherwise, compare hosts lexicographically
		if a.Host < b.Host {
			return -1
		} else if a.Host > b.Host {
			return 1
		}
		return 0
	})
	c.machHost = candidates[0].Host
	c.machPort = candidates[0].Port

	defaultSession = c
	return nil
}

func buildHttpURL(proto string, host string, port int, path string) string {
	if port > 0 {
		return fmt.Sprintf("%s://%s:%d%s", proto, host, port, path)
	}
	return fmt.Sprintf("%s://%s%s", proto, host, path)
}

func resolveUnixSocketPath(addr string) (string, error) {
	pwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	path := strings.TrimPrefix(addr, "unix://")

	// For unix-style absolute paths (e.g. /tmp/test.sock), attach current drive on Windows.
	if strings.HasPrefix(path, "/") && filepath.VolumeName(path) == "" {
		path = filepath.VolumeName(pwd) + path
	}
	path = filepath.FromSlash(path)

	if !filepath.IsAbs(path) {
		path = filepath.Join(pwd, path)
	}

	path = filepath.Clean(path)
	return path, nil
}

type HostPort struct {
	Host string `json:"host"`
	Port int    `json:"port"`
}

// isLoopback checks if a host is a loopback address
func isLoopback(host string) bool {
	if host == "localhost" || host == "localhost.localdomain" {
		return true
	}
	ip := net.ParseIP(host)
	if ip != nil {
		return ip.IsLoopback()
	}
	return false
}

type HttpConfig struct {
	Protocol string `json:"protocol"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	User     string `json:"user"`
	Password string `json:"password"`
}

func GetHttpConfig() HttpConfig {
	return HttpConfig{
		Protocol: "http:",
		Host:     defaultSession.httpHost,
		Port:     defaultSession.httpPort,
		User:     defaultSession.User,
		Password: defaultSession.Password,
	}
}

func SetHttpToken(accessToken string, refreshToken string) {
	defaultSession.accessToken = accessToken
	defaultSession.refreshToken = refreshToken
}

func GetHttpAccessToken() string {
	return defaultSession.accessToken
}

func GetHttpRefreshToken() string {
	return defaultSession.refreshToken
}

type MachCliConfig struct {
	Host         string `json:"host"`
	Port         int    `json:"port"`
	User         string `json:"user"`
	Password     string `json:"password"`
	IdentityFile string `json:"identityFile,omitempty"`
	Database     string `json:"database,omitempty"`
}

func GetMachCliConfig() MachCliConfig {
	password := defaultSession.Password
	identityFile := defaultSession.IdentityFile
	if identityFile != "" && strings.HasPrefix(password, "$otp$") {
		// If IdentityFile is provided and password is OTP,
		// use auth key pair for authentication and ignore the OTP password.
		password = ""
	}
	return MachCliConfig{
		Host:         defaultSession.machHost,
		Port:         defaultSession.machPort,
		User:         defaultSession.User,
		Password:     password,
		IdentityFile: identityFile,
		Database:     defaultSession.Database,
	}
}

func SwitchUser(user, password string) error {
	if defaultSession.Server == "" {
		return errors.New("session does not exist")
	}
	result, err := loginWithHttp(defaultSession.Server, user, password)
	if err != nil {
		return err
	}
	defaultSession.User = user
	defaultSession.Password = password
	defaultSession.accessToken = result.accessToken
	defaultSession.refreshToken = result.refreshToken
	return nil
}

// Reconnect re-authenticates against server/user/password and replaces the current
// session in-place, redoing the entry-time discovery (mach host/port, SERVICE_CONTROLLER)
// that Configure() performs. It is used by the `connect` shell command so that changing
// the host no longer requires spawning a nested child shell process.
func Reconnect(server, user, password string) error {
	return Configure(Config{
		Server:       server,
		User:         user,
		Password:     password,
		IdentityFile: defaultSession.IdentityFile,
	})
}

// UseDatabase records the database selected by the `use <database>` shell command.
// It does not validate the name or contact the server; the mach driver validates it
// (and reports an error) the next time a connection is opened with this database in
// its DSN.
func UseDatabase(name string) {
	defaultSession.Database = name
}

// GetCurrentDatabase returns the database most recently selected via UseDatabase,
// or "" if none was selected (i.e. the server's default database applies).
func GetCurrentDatabase() string {
	return defaultSession.Database
}

type LoginResult struct {
	accessToken  string
	refreshToken string
	httpUnix     string
	httpProto    string
	httpHost     string
	httpPort     int
}

func loginWithHttp(serverAddr string, user string, password string) (*LoginResult, error) {
	ret := &LoginResult{}
	httpClient := http.DefaultClient
	if strings.HasPrefix(serverAddr, "unix://") {
		if socketPath, err := resolveUnixSocketPath(serverAddr); err != nil {
			return nil, err
		} else {
			ret.httpUnix = socketPath
		}
		ret.httpProto = "http"
		ret.httpHost = "unix"
		ret.httpPort = 0
		httpClient = &http.Client{
			Transport: &http.Transport{
				DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
					if strings.HasPrefix(addr, "unix:") { // e.g) addr = "unix:80"
						var dialer net.Dialer
						return dialer.DialContext(ctx, "unix", ret.httpUnix)
					} else {
						var dialer net.Dialer
						return dialer.DialContext(ctx, network, addr)
					}
				},
			},
		}
	} else if h, p, err := net.SplitHostPort(serverAddr); err == nil {
		ret.httpProto = "http"
		ret.httpHost = h
		ret.httpPort, err = strconv.Atoi(p)
		if err != nil {
			return nil, err
		}
	} else {
		return nil, fmt.Errorf("invalid server address: %s", serverAddr)
	}

	loginPayload := map[string]string{
		"loginName": user,
		"password":  password,
	}
	b, _ := json.Marshal(loginPayload)
	path := buildHttpURL(ret.httpProto, ret.httpHost, ret.httpPort, "/web/api/login")

	loginReq, err := http.NewRequest("POST", path, bytes.NewBuffer(b))
	if err != nil {
		return nil, err
	}
	loginReq.Header.Set("Content-Type", "application/json")
	rsp, err := httpClient.Do(loginReq)
	if err != nil {
		return nil, err
	}
	defer rsp.Body.Close()
	if rsp.StatusCode != http.StatusOK {
		if rsp.StatusCode == http.StatusNotFound {
			return nil, ErrUserOrPasswordIncorrect
		}
		return nil, fmt.Errorf("login failed with status code %d", rsp.StatusCode)
	}
	var rspData struct {
		AccessToken  string `json:"accessToken"`
		RefreshToken string `json:"refreshToken"`
	}
	if err := json.NewDecoder(rsp.Body).Decode(&rspData); err != nil {
		return nil, err
	}
	ret.accessToken = rspData.AccessToken
	ret.refreshToken = rspData.RefreshToken
	return ret, nil
}
