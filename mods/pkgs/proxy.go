package pkgs

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/machbase/neo-server/mods/logging"
)

type HttpProxy struct {
	Prefix      string `yaml:"prefix"`
	Address     string `yaml:"address"`
	StripPrefix string `yaml:"strip_prefix,omitempty"`
	log         logging.Log
	baseDir     string
	proxy       *httputil.ReverseProxy
}

func (hp *HttpProxy) Match(path string) bool {
	return hp != nil && hp.Prefix != "" && strings.HasPrefix(path, hp.Prefix)
}

func (hp *HttpProxy) Handle(ctx *gin.Context) {
	defer func() {
		if r := recover(); r != nil {
			hp.log.Warn("Recovered in proxy", r)
		}
	}()

	if hp.proxy == nil {
		if hp.Address == "" {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid destination address"})
			return
		}
		addr, err := url.Parse(hp.Address)
		if err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if strings.HasPrefix(hp.Address, "http://") {
			hp.proxy = httputil.NewSingleHostReverseProxy(addr)
		} else if strings.HasPrefix(hp.Address, "https://") {
			hp.proxy = httputil.NewSingleHostReverseProxy(addr)
			hp.proxy.Transport = &http.Transport{
				DialTLSContext:  dialTLS,
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			}
		} else if strings.HasPrefix(hp.Address, "unix://") {
			target := &url.URL{Scheme: "http", Host: "127.0.0.1"}
			hp.proxy = httputil.NewSingleHostReverseProxy(target)
			path := strings.TrimPrefix(hp.Address, "unix://")
			if !filepath.IsAbs(path) {
				path = filepath.Join(hp.baseDir, path)
			}
			path = filepath.Clean(path)
			if len(path) >= 100 {
				hp.log.Error("Unix socket path is too long", path, "len:", len(path))
				ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid destination address is too long"})
				return
			}
			hp.proxy.Transport = &http.Transport{
				DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
					ret, err := net.Dial("unix", path)
					if err != nil {
						hp.log.Error("Failed to dial", err)
					}
					return ret, err
				},
			}
		}
	}
	if hp.StripPrefix != "" {
		ctx.Request.URL.Path = strings.TrimPrefix(ctx.Request.URL.Path, hp.StripPrefix)
		ctx.Request.URL.Path = "/" + strings.TrimPrefix(ctx.Request.URL.Path, "/")
	}
	hp.proxy.ServeHTTP(ctx.Writer, ctx.Request)
}

func dialTLS(ctx context.Context, network, addr string) (net.Conn, error) {
	conn, err := net.Dial(network, addr)
	if err != nil {
		return nil, err
	}

	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}
	cfg := &tls.Config{ServerName: host}

	tlsConn := tls.Client(conn, cfg)
	if err := tlsConn.Handshake(); err != nil {
		conn.Close()
		return nil, err
	}

	cs := tlsConn.ConnectionState()
	cert := cs.PeerCertificates[0]

	cert.VerifyHostname(host)

	return tlsConn, nil
}
