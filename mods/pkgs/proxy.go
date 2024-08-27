package pkgs

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
)

type HttpProxy struct {
	Prefix      string `yaml:"prefix"`
	DestPort    int    `yaml:"port"`
	DestHost    string `yaml:"host,omitempty"`
	StripPrefix string `yaml:"strip_prefix,omitempty"`
	proxy       *httputil.ReverseProxy
}

func (hp *HttpProxy) Match(path string) bool {
	return hp != nil && hp.Prefix != "" && strings.HasPrefix(path, hp.Prefix)
}

func (hp *HttpProxy) Handle(ctx *gin.Context) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in f", r)
		}
	}()

	if hp.proxy == nil {
		host := fmt.Sprintf("127.0.0.1:%d", hp.DestPort)
		if hp.DestHost != "" {
			host = fmt.Sprintf("%s:%d", hp.DestHost, hp.DestPort)
		}
		target := &url.URL{
			Scheme: "http",
			Host:   host,
		}
		hp.proxy = httputil.NewSingleHostReverseProxy(target)
		if target.Scheme == "https" {
			hp.proxy.Transport = &http.Transport{
				DialTLSContext:  dialTLS,
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
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
