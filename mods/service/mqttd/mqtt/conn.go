package mqtt

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"net"
	"strings"
	"time"

	"github.com/machbase/neo-server/mods/service/security"
)

type Connection interface {
	RemoteAddr() net.Addr
	RemoteAddrString() string
	RemoteCertHash() string
	Close()
	TcpConn() (*net.TCPConn, bool)
	TlsConn() (*tls.Conn, bool)
	IsSecure() bool
	CommonName() string

	Reader() io.Reader
	Writer() io.Writer

	SetReadDeadline(dur time.Duration)
}

type connection struct {
	isTls          bool
	commonName     string
	remoteAddr     net.Addr
	remoteCertHash string
	raw            net.Conn
}

func NewTcpConnection(raw net.Conn) Connection {
	return &connection{
		isTls:          false,
		commonName:     "",
		remoteAddr:     raw.RemoteAddr(),
		remoteCertHash: "",
		raw:            raw,
	}
}

func NewTlsConnection(raw net.Conn, clientCert *x509.Certificate) Connection {
	commonName := clientCert.Subject.CommonName
	remoteAddr := raw.RemoteAddr()
	certHash, _ := security.HashCertificate(clientCert)

	return &connection{
		isTls:          true,
		commonName:     commonName,
		remoteAddr:     remoteAddr,
		remoteCertHash: certHash,
		raw:            raw,
	}
}

func (c *connection) Close() {
	if c.raw != nil {
		c.raw.Close()
		c.raw = nil
	}
}

func (c *connection) Reader() io.Reader {
	return c.raw
}

func (c *connection) Writer() io.Writer {
	return c.raw
}

func (c *connection) SetReadDeadline(dur time.Duration) {
	if c.raw != nil {
		if dur.Milliseconds() > 0 {
			c.raw.SetReadDeadline(time.Now().Add(dur))
		} else {
			c.raw.SetReadDeadline(time.Time{})
		}
	}
}

func (c *connection) TcpConn() (*net.TCPConn, bool) {
	t, f := c.raw.(*net.TCPConn)
	return t, f
}

func (c *connection) TlsConn() (*tls.Conn, bool) {
	t, f := c.raw.(*tls.Conn)
	return t, f
}

func (c *connection) IsSecure() bool {
	return c.isTls
}

func (c *connection) RemoteAddr() net.Addr {
	return c.remoteAddr
}

func (c *connection) RemoteAddrString() string {
	if c.isTls {
		return fmt.Sprintf("ssl/%s", c.remoteAddr.String())
	} else {
		return fmt.Sprintf("tcp/%s", c.remoteAddr.String())
	}
}

func (c *connection) RemoteCertHash() string {
	return c.remoteCertHash
}

func (c *connection) CommonName() string {
	if c.commonName == "" {
		return strings.Repeat("-", 20)
	} else {
		return c.commonName
	}
}

type RejectReason int

const (
	RejectSslHandshake RejectReason = 1 + iota
	RejectInvalidCertificate
	RejectByAllowancePolicy
)

var RejectReasons = []string{
	"UndefinedRejectCode",
	"SslHandshakeFailed",
	"InvalidCertificate",
	"RemoteHostNotAllow",
}

type RejectConnection interface {
	RemoteHost() string
	RemotePort() int
	ReasonCode() RejectReason
	Reason() string
	Error() error
}

func NewRejectConnection(host string, port int, reason RejectReason, err error) RejectConnection {
	return &rejectConnection{
		remoteHost: host,
		remotePort: port,
		reasonCode: reason,
		err:        err,
	}
}

type rejectConnection struct {
	remoteHost string
	remotePort int
	reasonCode RejectReason
	err        error
}

func (r *rejectConnection) RemoteHost() string       { return r.remoteHost }
func (r *rejectConnection) RemotePort() int          { return r.remotePort }
func (r *rejectConnection) ReasonCode() RejectReason { return r.reasonCode }
func (r *rejectConnection) Reason() string           { return RejectReasons[r.reasonCode] }
func (r *rejectConnection) Error() error             { return r.err }
