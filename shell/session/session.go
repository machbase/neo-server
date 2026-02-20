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
)

func Module(rt *goja.Runtime, module *goja.Object) {
	exports := module.Get("exports").(*goja.Object)
	exports.Set("getHttpConfig", GetHttpConfig)
	exports.Set("setHttpToken", SetHttpToken)
	exports.Set("getHttpAccessToken", GetHttpAccessToken)
	exports.Set("getHttpRefreshToken", GetHttpRefreshToken)
	exports.Set("getMachCliConfig", GetMachCliConfig)
}

type Config struct {
	Server   string
	User     string
	Password string

	httpProto string
	httpHost  string
	httpPort  int
	httpUnix  string // for unix socket, holds socket path

	machHost string
	machPort int

	accessToken  string
	refreshToken string
}

var ErrUserOrPasswordIncorrect = errors.New("user or password is incorrect")

var defaultSession Config

func Configure(c Config) error {
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

	rpcPayload := map[string]any{
		"jsonrpc": "2.0",
		"method":  "getServicePorts",
		"params":  []any{"mach"},
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
		return fmt.Errorf("getServicePorts failed with status code %d", rpcRsp.StatusCode)
	}
	var rpcRspData struct {
		Result []map[string]string `json:"result"`
	}
	if err := json.NewDecoder(rpcRsp.Body).Decode(&rpcRspData); err != nil {
		return err
	}
	candidates := []HostPort{}
	for _, portInfo := range rpcRspData.Result {
		addr := portInfo["Address"]
		if strings.HasPrefix(addr, "tcp://") {
			addr = strings.TrimPrefix(addr, "tcp://")
			host, portStr, err := net.SplitHostPort(addr)
			if err != nil {
				return err
			}
			port, err := strconv.Atoi(portStr)
			if err != nil {
				return err
			}
			candidates = append(candidates, HostPort{Host: host, Port: port})
		}
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
	if strings.HasPrefix(addr, "unix://../") {
		addr = fmt.Sprintf("unix:///%s", filepath.Join(filepath.Dir(pwd), addr[len("unix://../"):]))
	} else if strings.HasPrefix(addr, "../") {
		addr = fmt.Sprintf("unix:///%s", filepath.Join(filepath.Dir(pwd), addr[len("../"):]))
	} else if strings.HasPrefix(addr, "unix://./") {
		addr = fmt.Sprintf("unix:///%s", filepath.Join(pwd, addr[len("unix://./"):]))
	} else if strings.HasPrefix(addr, "./") {
		addr = fmt.Sprintf("unix:///%s", filepath.Join(pwd, addr[len("./"):]))
	} else if strings.HasPrefix(addr, "/") {
		addr = fmt.Sprintf("unix://%s", addr)
	}

	path := strings.TrimPrefix(addr, "unix://")
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
	Host     string `json:"host"`
	Port     int    `json:"port"`
	User     string `json:"user"`
	Password string `json:"password"`
}

func GetMachCliConfig() MachCliConfig {
	return MachCliConfig{
		Host:     defaultSession.machHost,
		Port:     defaultSession.machPort,
		User:     defaultSession.User,
		Password: defaultSession.Password,
	}
}
