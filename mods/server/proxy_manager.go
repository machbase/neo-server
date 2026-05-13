package server

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

var (
	errProxyNotFound = errors.New("proxy not found")
	errProxyConflict = errors.New("proxy already registered")
)

type ProxyRegisterRequest struct {
	Service     string `json:"service"`
	Prefix      string `json:"prefix"`
	Target      string `json:"target"`
	StripPrefix string `json:"strip_prefix,omitempty"`
	HealthPath  string `json:"health_path,omitempty"`
}

type ProxyUnregisterRequest struct {
	Service string `json:"service"`
	Prefix  string `json:"prefix,omitempty"`
}

type ProxyGetRequest struct {
	Service string `json:"service"`
	Prefix  string `json:"prefix"`
}

type ProxyEntrySnapshot struct {
	Service      string    `json:"service"`
	Prefix       string    `json:"prefix"`
	Target       string    `json:"target"`
	StripPrefix  string    `json:"strip_prefix,omitempty"`
	HealthPath   string    `json:"health_path,omitempty"`
	RegisteredAt time.Time `json:"registered_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type ProxyManager struct {
	mu      sync.RWMutex
	entries map[string]*proxyEntry
}

type proxyEntry struct {
	ProxyEntrySnapshot
	targetURL  *url.URL
	unixSocket string
	proxy      *httputil.ReverseProxy
}

func NewProxyManager() *ProxyManager {
	return &ProxyManager{entries: map[string]*proxyEntry{}}
}

func (pm *ProxyManager) Register(req ProxyRegisterRequest) (ProxyEntrySnapshot, error) {
	if pm == nil {
		return ProxyEntrySnapshot{}, fmt.Errorf("proxy manager is not initialized")
	}
	req.Service = strings.TrimSpace(req.Service)
	req.Prefix = normalizeProxyPrefix(req.Prefix)
	req.Target = strings.TrimSpace(req.Target)
	req.StripPrefix = strings.TrimSpace(req.StripPrefix)
	req.HealthPath = strings.TrimSpace(req.HealthPath)
	if req.Service == "" {
		return ProxyEntrySnapshot{}, fmt.Errorf("proxy service is required")
	}
	if req.Prefix == "" {
		return ProxyEntrySnapshot{}, fmt.Errorf("proxy prefix is required")
	}
	if req.Prefix == "/" {
		return ProxyEntrySnapshot{}, fmt.Errorf("proxy prefix '/' is not allowed")
	}
	targetURL, unixSocket, err := parseProxyTarget(req.Target)
	if err != nil {
		return ProxyEntrySnapshot{}, err
	}
	now := time.Now()
	key := proxyKey(req.Service, req.Prefix)

	pm.mu.Lock()
	defer pm.mu.Unlock()
	if existing := pm.entries[key]; existing != nil {
		if existing.Target != req.Target || existing.StripPrefix != req.StripPrefix || existing.HealthPath != req.HealthPath {
			return ProxyEntrySnapshot{}, fmt.Errorf("%w: %s %s", errProxyConflict, req.Service, req.Prefix)
		}
		existing.UpdatedAt = now
		return existing.ProxyEntrySnapshot, nil
	}
	routePrefix := proxyServiceRoutePrefix(req.Service, req.Prefix)
	for _, existing := range pm.entries {
		if proxyServiceRoutePrefix(existing.Service, existing.Prefix) == routePrefix {
			return ProxyEntrySnapshot{}, fmt.Errorf("%w: public route %s", errProxyConflict, routePrefix)
		}
	}
	entry := &proxyEntry{
		ProxyEntrySnapshot: ProxyEntrySnapshot{
			Service:      req.Service,
			Prefix:       req.Prefix,
			Target:       req.Target,
			StripPrefix:  req.StripPrefix,
			HealthPath:   req.HealthPath,
			RegisteredAt: now,
			UpdatedAt:    now,
		},
		targetURL:  targetURL,
		unixSocket: unixSocket,
	}
	entry.proxy = entry.newReverseProxy()
	pm.entries[key] = entry
	return entry.ProxyEntrySnapshot, nil
}

func (pm *ProxyManager) Unregister(req ProxyUnregisterRequest) ([]ProxyEntrySnapshot, error) {
	if pm == nil {
		return nil, fmt.Errorf("proxy manager is not initialized")
	}
	req.Service = strings.TrimSpace(req.Service)
	req.Prefix = normalizeProxyPrefix(req.Prefix)
	if req.Service == "" {
		return nil, fmt.Errorf("proxy service is required")
	}

	pm.mu.Lock()
	defer pm.mu.Unlock()
	removed := []ProxyEntrySnapshot{}
	if req.Prefix != "" {
		key := proxyKey(req.Service, req.Prefix)
		if entry := pm.entries[key]; entry != nil {
			removed = append(removed, entry.ProxyEntrySnapshot)
			delete(pm.entries, key)
		}
		return removed, nil
	}
	for key, entry := range pm.entries {
		if entry.Service == req.Service {
			removed = append(removed, entry.ProxyEntrySnapshot)
			delete(pm.entries, key)
		}
	}
	sortProxySnapshots(removed)
	return removed, nil
}

func (pm *ProxyManager) Get(req ProxyGetRequest) (ProxyEntrySnapshot, error) {
	if pm == nil {
		return ProxyEntrySnapshot{}, fmt.Errorf("proxy manager is not initialized")
	}
	service := strings.TrimSpace(req.Service)
	prefix := normalizeProxyPrefix(req.Prefix)
	if service == "" || prefix == "" {
		return ProxyEntrySnapshot{}, fmt.Errorf("proxy service and prefix are required")
	}
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	entry := pm.entries[proxyKey(service, prefix)]
	if entry == nil {
		return ProxyEntrySnapshot{}, errProxyNotFound
	}
	return entry.ProxyEntrySnapshot, nil
}

func (pm *ProxyManager) List(service string) []ProxyEntrySnapshot {
	if pm == nil {
		return []ProxyEntrySnapshot{}
	}
	service = strings.TrimSpace(service)
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	result := []ProxyEntrySnapshot{}
	for _, entry := range pm.entries {
		if service == "" || entry.Service == service {
			result = append(result, entry.ProxyEntrySnapshot)
		}
	}
	sortProxySnapshots(result)
	return result
}

func (pm *ProxyManager) Handle(ctx *gin.Context, servicePath string) bool {
	entry := pm.match(servicePath)
	if entry == nil {
		ctx.JSON(http.StatusNotFound, gin.H{"success": false, "reason": "proxy not registered"})
		return true
	}
	entry.serve(ctx)
	return true
}

func (pm *ProxyManager) match(servicePath string) *proxyEntry {
	if pm == nil {
		return nil
	}
	servicePath = cleanProxyPath(servicePath)
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	var selected *proxyEntry
	selectedPrefixLen := 0
	for _, entry := range pm.entries {
		routePrefix := proxyServiceRoutePrefix(entry.Service, entry.Prefix)
		if !proxyPrefixMatch(servicePath, routePrefix) {
			continue
		}
		if selected == nil || len(routePrefix) > selectedPrefixLen {
			selected = entry
			selectedPrefixLen = len(routePrefix)
		}
	}
	return selected
}

func (pe *proxyEntry) serve(ctx *gin.Context) {
	req := ctx.Request
	basePath := "/web/services/" + pe.Service + strings.TrimSuffix(pe.Prefix, "/")
	stripPrefix := pe.StripPrefix
	if stripPrefix == "" {
		stripPrefix = basePath
	}
	req.URL.Path = stripProxyRequestPath(req.URL.Path, stripPrefix)
	req.URL.RawPath = ""
	req.Header.Set("X-Forwarded-Host", req.Host)
	if req.TLS != nil {
		req.Header.Set("X-Forwarded-Proto", "https")
	} else {
		req.Header.Set("X-Forwarded-Proto", "http")
	}
	req.Header.Set("X-Forwarded-Prefix", basePath)
	pe.proxy.ServeHTTP(ctx.Writer, req)
}

func (pe *proxyEntry) newReverseProxy() *httputil.ReverseProxy {
	target := pe.targetURL
	proxy := httputil.NewSingleHostReverseProxy(target)
	if pe.unixSocket != "" {
		proxy.Transport = &http.Transport{
			DialContext: func(ctx context.Context, network string, address string) (net.Conn, error) {
				return net.Dial("unix", pe.unixSocket)
			},
		}
	}
	return proxy
}

func parseProxyTarget(raw string) (*url.URL, string, error) {
	if raw == "" {
		return nil, "", fmt.Errorf("proxy target is required")
	}
	if strings.HasPrefix(raw, "unix://") {
		socketPath := filepath.Clean(strings.TrimPrefix(raw, "unix://"))
		if !filepath.IsAbs(socketPath) {
			return nil, "", fmt.Errorf("proxy unix socket must be absolute")
		}
		if len(socketPath) >= 100 {
			return nil, "", fmt.Errorf("proxy unix socket path is too long")
		}
		return &url.URL{Scheme: "http", Host: "127.0.0.1"}, socketPath, nil
	}
	targetURL, err := url.Parse(raw)
	if err != nil {
		return nil, "", err
	}
	if targetURL.Scheme != "http" {
		return nil, "", fmt.Errorf("proxy target scheme %q is not allowed", targetURL.Scheme)
	}
	host := targetURL.Hostname()
	if host == "" {
		return nil, "", fmt.Errorf("proxy target host is required")
	}
	if !isLoopbackHost(host) {
		return nil, "", fmt.Errorf("proxy target host %q is not allowed", host)
	}
	return targetURL, "", nil
}

func isLoopbackHost(host string) bool {
	host = strings.Trim(strings.ToLower(host), "[]")
	if host == "localhost" {
		return true
	}
	if ip := net.ParseIP(host); ip != nil {
		return ip.IsLoopback()
	}
	return false
}

func normalizeProxyPrefix(prefix string) string {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		return ""
	}
	prefix = cleanProxyPath(prefix)
	if prefix != "/" && !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}
	return prefix
}

func cleanProxyPath(value string) string {
	if value == "" {
		return "/"
	}
	if !strings.HasPrefix(value, "/") {
		value = "/" + value
	}
	return path.Clean(value)
}

func proxyPrefixMatch(appPath string, prefix string) bool {
	appPath = cleanProxyPath(appPath)
	prefix = normalizeProxyPrefix(prefix)
	if prefix == "/" {
		return true
	}
	trimmed := strings.TrimSuffix(prefix, "/")
	return appPath == trimmed || strings.HasPrefix(appPath, prefix)
}

func stripProxyRequestPath(requestPath string, stripPrefix string) string {
	if stripPrefix == "" {
		return requestPath
	}
	requestPath = cleanProxyPath(requestPath)
	stripPrefix = strings.TrimSuffix(cleanProxyPath(stripPrefix), "/")
	if requestPath == stripPrefix {
		return "/"
	}
	if strings.HasPrefix(requestPath, stripPrefix+"/") {
		return strings.TrimPrefix(requestPath, stripPrefix)
	}
	return requestPath
}

func proxyKey(service string, prefix string) string {
	return service + "\x00" + normalizeProxyPrefix(prefix)
}

func proxyServiceRoutePrefix(service string, prefix string) string {
	return normalizeProxyPrefix("/" + strings.Trim(strings.TrimSpace(service), "/") + normalizeProxyPrefix(prefix))
}

func sortProxySnapshots(entries []ProxyEntrySnapshot) {
	sort.Slice(entries, func(i int, j int) bool {
		if entries[i].Service != entries[j].Service {
			return entries[i].Service < entries[j].Service
		}
		return entries[i].Prefix < entries[j].Prefix
	})
}
