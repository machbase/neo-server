package session

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
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

	machHost string
	machPort int

	accessToken  string
	refreshToken string
}

var defaultSession Config

func Configure(c Config) error {
	if h, p, err := net.SplitHostPort(c.Server); err == nil {
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
	path := fmt.Sprintf("%s://%s:%d/web/api/login", c.httpProto, c.httpHost, c.httpPort)
	rsp, err := http.Post(path, "application/json", bytes.NewBuffer(b))
	if err != nil {
		return err
	}
	defer rsp.Body.Close()
	if rsp.StatusCode != http.StatusOK {
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
	rpcPath := fmt.Sprintf("%s://%s:%d/web/api/rpc", c.httpProto, c.httpHost, c.httpPort)
	rpcReq, err := http.NewRequest("POST", rpcPath, bytes.NewBuffer(b))
	if err != nil {
		return err
	}
	rpcReq.Header.Set("Content-Type", "application/json")
	rpcReq.Header.Set("Authorization", "Bearer "+c.accessToken)
	rpcRsp, err := http.DefaultClient.Do(rpcReq)
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
