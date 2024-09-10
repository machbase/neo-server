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
)

type HttpProxy struct {
	Prefix      string `yaml:"prefix" json:"prefix"`
	Address     string `yaml:"address" json:"address"`
	StripPrefix string `yaml:"strip_prefix,omitempty" json:"strip_prefix,omitempty"`
	proxy       *httputil.ReverseProxy
}

func (pb *PkgBackend) Match(path string) bool {
	return pb.HttpProxy != nil && pb.HttpProxy.Prefix != "" && strings.HasPrefix(path, pb.HttpProxy.Prefix)
}

func (pb *PkgBackend) Handle(ctx *gin.Context) {
	defer func() {
		if r := recover(); r != nil {
			pb.log.Warn("Recovered in proxy", r)
		}
	}()
	pb.RLock()
	defer pb.RUnlock()

	if pb.HttpProxy.proxy == nil {
		if pb.HttpProxy.Address == "" {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid destination address"})
			return
		}
		addr, err := url.Parse(pb.HttpProxy.Address)
		if err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if strings.HasPrefix(pb.HttpProxy.Address, "http://") {
			pb.HttpProxy.proxy = httputil.NewSingleHostReverseProxy(addr)
		} else if strings.HasPrefix(pb.HttpProxy.Address, "https://") {
			pb.HttpProxy.proxy = httputil.NewSingleHostReverseProxy(addr)
			pb.HttpProxy.proxy.Transport = &http.Transport{
				DialTLSContext:  dialTLS,
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			}
		} else if strings.HasPrefix(pb.HttpProxy.Address, "unix://") {
			target := &url.URL{Scheme: "http", Host: "127.0.0.1"}
			pb.HttpProxy.proxy = httputil.NewSingleHostReverseProxy(target)
			path := strings.TrimPrefix(pb.HttpProxy.Address, "unix://")
			if !filepath.IsAbs(path) {
				path = filepath.Join(pb.dir, path)
			}
			path = filepath.Clean(path)
			if len(path) >= 100 {
				pb.log.Error("Unix socket path is too long", path, "len:", len(path))
				ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid destination address is too long"})
				return
			}
			pb.HttpProxy.proxy.Transport = &http.Transport{
				DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
					ret, err := net.Dial("unix", path)
					if err != nil {
						pb.log.Error("Failed to dial", err)
					}
					return ret, err
				},
			}
		}
	}
	if pb.HttpProxy.StripPrefix != "" {
		ctx.Request.URL.Path = strings.TrimPrefix(ctx.Request.URL.Path, pb.HttpProxy.StripPrefix)
		ctx.Request.URL.Path = "/" + strings.TrimPrefix(ctx.Request.URL.Path, "/")
	}
	pb.HttpProxy.proxy.ServeHTTP(ctx.Writer, ctx.Request)
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
