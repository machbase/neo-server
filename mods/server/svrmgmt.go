package server

import (
	"context"
	"crypto"
	"errors"
	"fmt"
	"net"
	"os"
	"regexp"
	"runtime"
	"runtime/debug"
	"strings"
	"time"

	"crypto/ecdsa"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/pem"

	"github.com/gin-gonic/gin"
	"github.com/gofrs/uuid/v5"
	"github.com/machbase/neo-client/api"
	server_api "github.com/machbase/neo-server/v8/api"
	"github.com/machbase/neo-server/v8/api/machsvr"
	"github.com/machbase/neo-server/v8/booter"
	"github.com/machbase/neo-server/v8/mods"
	"github.com/machbase/neo-server/v8/mods/model"
	"google.golang.org/grpc/peer"
)

type ListKeyResponse struct {
	Success bool       `json:"success"`
	Reason  string     `json:"reason"`
	Elapse  string     `json:"elapse"`
	Keys    []*KeyInfo `json:"keys"`
}

type KeyInfo struct {
	Idx       int    `json:"idx"`
	Id        string `json:"id"`
	NotBefore int64  `json:"notBefore"`
	NotAfter  int64  `json:"notAfter"`
}

func (s *Server) ListKey(context.Context) (*ListKeyResponse, error) {
	tick := time.Now()
	rsp := &ListKeyResponse{}
	defer func() {
		rsp.Elapse = time.Since(tick).String()
	}()

	err := s.IterateAuthorizedCertificates(func(id string) bool {
		cert, err := s.AuthorizedCertificate(id)
		if err != nil {
			s.log.Warnf("fail to load certificate '%s', %s", id, err.Error())
			return true
		}
		if id != cert.Subject.CommonName {
			s.log.Warnf("certificate id '%s' has different common name '%s'", id, cert.Subject.CommonName)
			return true
		}

		item := KeyInfo{
			Idx:       len(rsp.Keys),
			Id:        cert.Subject.CommonName,
			NotBefore: cert.NotBefore.Unix(),
			NotAfter:  cert.NotAfter.Unix(),
		}

		rsp.Keys = append(rsp.Keys, &item)
		return true
	})
	if err != nil {
		rsp.Reason = err.Error()
		return rsp, nil
	}
	rsp.Success, rsp.Reason = true, "success"
	rsp.Elapse = time.Since(tick).String()
	return rsp, nil
}

type GenKeyRequest struct {
	Id        string `json:"id"`
	Type      string `json:"type"`      // rsa, ecdsa
	NotBefore int64  `json:"notBefore"` // unix epoch in seconds
	NotAfter  int64  `json:"notAfter"`  // unix epoch in seconds
}

type GenKeyResponse struct {
	Success     bool   `json:"success"`
	Reason      string `json:"reason"`
	Elapse      string `json:"elapse"`
	Id          string `json:"id"`
	Token       string `json:"token"`
	Key         string `json:"key"`
	Certificate string `json:"certificate"`
}

func (s *Server) GenKey(ctx context.Context, req *GenKeyRequest) (*GenKeyResponse, error) {
	tick := time.Now()
	rsp := &GenKeyResponse{Reason: "not specified"}
	defer func() {
		rsp.Elapse = time.Since(tick).String()
	}()

	req.Id = strings.ToLower(req.Id)
	pass, _ := regexp.MatchString("[a-z][a-z0-9_.@-]+", req.Id)
	if !pass {
		rsp.Reason = "id contains invalid character"
		return rsp, nil
	}
	if len(req.Id) > 40 {
		rsp.Reason = "id is too long, should be shorter than 40 characters"
		return rsp, nil
	}
	_, err := s.AuthorizedCertificate(req.Id)
	if err != nil && err != os.ErrNotExist {
		if err == os.ErrExist {
			rsp.Reason = fmt.Sprintf("'%s' already exists", req.Id)
		} else {
			rsp.Reason = err.Error()
		}
		return rsp, nil
	}

	ca, err := s.ServerCertificate()
	if err != nil {
		return nil, err
	}
	caKey, err := s.ServerPrivateKey()
	if err != nil {
		return nil, err
	}
	gen := GenCertReq{
		Name: pkix.Name{
			CommonName: req.Id,
		},
		NotBefore: time.Unix(req.NotBefore, 0),
		NotAfter:  time.Unix(req.NotAfter, 0),
		Issuer:    ca,
		IssuerKey: caKey,
		Type:      req.Type,
		Format:    "pkcs8",
	}
	cert, key, token, err := generateClientKey(&gen)
	if err != nil {
		rsp.Reason = err.Error()
		return rsp, nil
	}

	s.SetAuthorizedCertificate(req.Id, cert)

	rsp.Id = req.Id
	rsp.Token = string(token)
	rsp.Certificate = string(cert)
	rsp.Key = string(key)
	rsp.Success, rsp.Reason = true, "success"
	rsp.Elapse = time.Since(tick).String()
	return rsp, nil
}

type DelKeyRequest struct {
	Id string `json:"id"`
}

type DelKeyResponse struct {
	Success bool   `json:"success"`
	Reason  string `json:"reason"`
	Elapse  string `json:"elapse"`
}

func (s *Server) DelKey(ctx context.Context, req *DelKeyRequest) (*DelKeyResponse, error) {
	tick := time.Now()
	rsp := &DelKeyResponse{}
	defer func() {
		rsp.Elapse = time.Since(tick).String()
	}()

	err := s.RemoveAuthorizedCertificate(req.Id)
	if err != nil {
		rsp.Reason = err.Error()
		return rsp, nil
	}

	rsp.Success, rsp.Reason = true, "success"
	rsp.Elapse = time.Since(tick).String()
	return rsp, nil
}

type ServerKeyResponse struct {
	Success     bool   `json:"success"`
	Reason      string `json:"reason"`
	Elapse      string `json:"elapse"`
	Certificate string `json:"certificate"`
}

func (s *Server) ServerKey(ctx context.Context) (*ServerKeyResponse, error) {
	tick := time.Now()
	rsp := &ServerKeyResponse{Reason: "unspecified"}
	defer func() {
		rsp.Elapse = time.Since(tick).String()
	}()
	path := s.ServerCertificatePath()
	b, err := os.ReadFile(path)
	if err != nil {
		rsp.Reason = err.Error()
	} else {
		rsp.Success = true
		rsp.Reason = "success"
		rsp.Certificate = string(b)
	}
	return rsp, nil
}

type GenCertReq struct {
	pkix.Name
	NotBefore time.Time
	NotAfter  time.Time
	Issuer    *x509.Certificate
	IssuerKey any
	Type      string // rsa
	Format    string // pkcs1, pkcs8
}

// generateClientKey returns certificate, privatekey, token and error
func generateClientKey(req *GenCertReq) ([]byte, []byte, string, error) {
	var clientKey any
	var clientPub any
	var clientKeyPEM []byte

	switch req.Type {
	case "rsa":
		bitSize := 4096
		key, err := rsa.GenerateKey(rand.Reader, bitSize)
		if err != nil {
			return nil, nil, "", err
		}
		clientKey = key
		clientPub = &key.PublicKey
		var keyBytes []byte
		switch req.Format {
		case "pkcs1":
			if _, ok := clientKey.(*rsa.PrivateKey); ok {
				keyBytes = x509.MarshalPKCS1PrivateKey(clientKey.(*rsa.PrivateKey))
			} else {
				return nil, nil, "", fmt.Errorf("%s key type can not encoded into pkcs1 format", req.Type)
			}
		default:
			keyBytes, _ = x509.MarshalPKCS8PrivateKey(clientKey)
		}
		clientKeyPEM = pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: keyBytes})
	case "ec", "ecdsa":
		ec := NewEllipticCurveP521()
		pri, pub, err := ec.GenerateKeys()
		if err != nil {
			return nil, nil, "", err
		}
		clientKey = pri
		clientPub = pub
		marshal, err := x509.MarshalECPrivateKey(pri)
		if err != nil {
			return nil, nil, "", err
		}
		clientKeyPEM = pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: marshal})
	default:
		return nil, nil, "", errors.New("unsupported key type")
	}

	token, err := GenerateClientToken(req.CommonName, clientKey, "b")
	if err != nil {
		return nil, nil, "", err
	}

	certBytes, err := GenerateClientCertificate(req.Name, req.NotBefore, req.NotAfter, req.Issuer, req.IssuerKey, clientPub)
	if err != nil {
		return nil, nil, "", fmt.Errorf("client certificate: %s", err.Error())
	}

	return certBytes, clientKeyPEM, token, nil
}

func GenerateClientToken(clientId string, clientPriKey crypto.PrivateKey, method string) (token string, err error) {
	var signature []byte
	hash := sha256.New()
	hash.Write([]byte(clientId))
	hashSum := hash.Sum(nil)
	switch key := clientPriKey.(type) {
	case *rsa.PrivateKey:
		signature, err = rsa.SignPSS(rand.Reader, key, crypto.SHA256, hashSum, nil)
		if err != nil {
			return "", err
		}
	case *ecdsa.PrivateKey:
		signature, err = ecdsa.SignASN1(rand.Reader, key, hashSum)
		if err != nil {
			return "", err
		}
	default:
		return "", fmt.Errorf("unsupported algorithm '%T'", key)
	}
	if method != "b" {
		return "", fmt.Errorf("unsupported method '%s'", method)
	}
	token = fmt.Sprintf("%s:%s:%s", clientId, method, hex.EncodeToString(signature))
	return
}

func VerifyClientToken(token string, clientPubKey crypto.PublicKey) (bool, error) {
	parts := strings.SplitN(token, ":", 3)
	if len(parts) != 3 {
		return false, errors.New("invalid token format")
	}

	if parts[1] != "b" {
		return false, fmt.Errorf("unsupported method '%s'", parts[1])
	}

	signature, err := hex.DecodeString(parts[2])
	if err != nil {
		return false, err
	}

	hash := sha256.New()
	hash.Write([]byte(parts[0]))
	hashSum := hash.Sum(nil)

	switch key := clientPubKey.(type) {
	case *rsa.PublicKey:
		err = rsa.VerifyPSS(key, crypto.SHA256, hashSum, signature, nil)
		if err != nil {
			fmt.Printf("rsa <<< %s", err.Error())
			return false, err
		}
		return err == nil, err
	case *ecdsa.PublicKey:
		return ecdsa.VerifyASN1(key, hashSum, signature), nil
	default:
		return false, fmt.Errorf("unsupported algorithm '%T'", key)
	}
}

type ListShellResponse struct {
	Success bool               `json:"success"`
	Reason  string             `json:"reason"`
	Elapse  string             `json:"elapse"`
	Shells  []*ShellDefinition `json:"shells"`
}

type ShellDefinition struct {
	Id      string `json:"id"`
	Name    string `json:"name"`
	Command string `json:"command"`
}

func (s *Server) ListShell(context.Context) (*ListShellResponse, error) {
	tick := time.Now()
	rsp := &ListShellResponse{}
	defer func() {
		rsp.Elapse = time.Since(tick).String()
	}()
	lst := s.models.ShellProvider().GetAllShells(false)
	for _, define := range lst {
		rsp.Shells = append(rsp.Shells, &ShellDefinition{
			Id:      define.Id,
			Name:    define.Label,
			Command: define.Command,
		})
	}
	rsp.Success, rsp.Reason = true, "success"
	return rsp, nil
}

type AddShellRequest struct {
	Name    string `json:"name"`
	Command string `json:"command"`
}

type AddShellResponse struct {
	Success bool   `json:"success"`
	Reason  string `json:"reason"`
	Elapse  string `json:"elapse"`
}

func (s *Server) AddShell(ctx context.Context, req *AddShellRequest) (*AddShellResponse, error) {
	tick := time.Now()
	rsp := &AddShellResponse{Reason: "not specified"}
	defer func() {
		rsp.Elapse = time.Since(tick).String()
	}()

	def := &model.ShellDefinition{}
	if len(req.Name) > 16 {
		rsp.Reason = "name is too long, should be shorter than 16 characters"
		return rsp, nil
	}
	uid, err := uuid.DefaultGenerator.NewV4()
	if err != nil {
		return nil, err
	}
	def.Id = uid.String()
	def.Label = req.Name
	def.Type = model.SHELL_TERM
	def.Attributes = &model.ShellAttributes{Removable: true, Cloneable: true, Editable: true}

	if len(strings.TrimSpace(req.Command)) == 0 {
		rsp.Reason = "command not specified"
		return rsp, nil
	} else {
		def.Command = req.Command
	}

	if err := s.models.ShellProvider().SaveShell(def); err != nil {
		rsp.Reason = err.Error()
		return rsp, nil
	}

	rsp.Success, rsp.Reason = true, "success"
	return rsp, nil
}

type DelShellRequest struct {
	Id string `json:"id"`
}

type DelShellResponse struct {
	Success bool   `json:"success"`
	Reason  string `json:"reason"`
	Elapse  string `json:"elapse"`
}

func (s *Server) DelShell(ctx context.Context, req *DelShellRequest) (*DelShellResponse, error) {
	tick := time.Now()
	rsp := &DelShellResponse{}
	defer func() {
		rsp.Elapse = time.Since(tick).String()
	}()
	if err := s.models.ShellProvider().RemoveShell(req.Id); err != nil {
		rsp.Reason = fmt.Sprintf("fail to remove %s", req.Id)
		return rsp, nil
	}
	rsp.Success, rsp.Reason = true, "success"
	return rsp, nil
}

type ListSshKeyResponse struct {
	Success bool      `json:"success"`
	Reason  string    `json:"reason"`
	Elapse  string    `json:"elapse"`
	SshKeys []*SshKey `json:"sshKeys"`
}

type SshKey struct {
	KeyType     string `json:"keyType"`
	Fingerprint string `json:"fingerprint"`
	Comment     string `json:"comment"`
}

func (s *Server) ListSshKey(ctx context.Context) (*ListSshKeyResponse, error) {
	tick := time.Now()
	rsp := &ListSshKeyResponse{Reason: "not-implemented"}
	defer func() {
		rsp.Elapse = time.Since(tick).String()
	}()

	list, err := s.GetAllAuthorizedSshKeys()
	if err != nil {
		return nil, err
	}
	for _, rec := range list {
		rsp.SshKeys = append(rsp.SshKeys, &SshKey{KeyType: rec.KeyType, Fingerprint: rec.Fingerprint, Comment: rec.Comment})
	}
	rsp.Success, rsp.Reason = true, "success"
	return rsp, nil
}

type AddSshKeyRequest struct {
	KeyType string `json:"keyType"`
	Key     string `json:"key"`
	Comment string `json:"comment"`
}

type AddSshKeyResponse struct {
	Success bool   `json:"success"`
	Reason  string `json:"reason"`
	Elapse  string `json:"elapse"`
}

func (s *Server) AddSshKey(ctx context.Context, req *AddSshKeyRequest) (*AddSshKeyResponse, error) {
	tick := time.Now()
	rsp := &AddSshKeyResponse{Reason: "not-implemented"}
	defer func() {
		rsp.Elapse = time.Since(tick).String()
	}()

	err := s.AddAuthorizedSshKey(req.KeyType, req.Key, req.Comment)
	if err != nil {
		rsp.Reason = err.Error()
	} else {
		rsp.Success, rsp.Reason = true, "success"
	}
	return rsp, nil
}

type DelSshKeyRequest struct {
	Fingerprint string `json:"fingerprint"`
}

type DelSshKeyResponse struct {
	Success bool   `json:"success"`
	Reason  string `json:"reason"`
	Elapse  string `json:"elapse"`
}

func (s *Server) DelSshKey(ctx context.Context, req *DelSshKeyRequest) (*DelSshKeyResponse, error) {
	tick := time.Now()
	rsp := &DelSshKeyResponse{Reason: "not-implemented"}
	defer func() {
		rsp.Elapse = time.Since(tick).String()
	}()

	err := s.RemoveAuthorizedSshKey(req.Fingerprint)
	if err != nil {
		rsp.Reason = err.Error()
	} else {
		rsp.Success, rsp.Reason = true, "success"
	}
	return rsp, nil
}

type ShutdownResponse struct {
	Success bool   `json:"success"`
	Reason  string `json:"reason"`
	Elapse  string `json:"elapse"`
}

// mgmt server implements
func (s *Server) Shutdown(ctx context.Context) (*ShutdownResponse, error) {
	if ctx, ok := ctx.(*gin.Context); ok {
		remoteAddr := ctx.RemoteIP()
		isTcpLocal := false
		switch remoteAddr {
		case "127.0.0.1":
			isTcpLocal = true
		case "0:0:0:0:0:0:0:1", "::1":
			isTcpLocal = true
		}
		if !isTcpLocal {
			return nil, fmt.Errorf("remote shutdown not allowed")
		}
		booter.NotifySignal()
		return nil, nil
	}
	tick := time.Now()
	rsp := &ShutdownResponse{}
	if runtime.GOOS != "windows" {
		p, ok := peer.FromContext(ctx)
		if !ok {
			rsp.Success, rsp.Reason = false, "failed to get peer address from ctx"
			rsp.Elapse = time.Since(tick).String()
			return rsp, nil
		}
		if p.Addr == net.Addr(nil) {
			rsp.Success, rsp.Reason = false, "failed to get peer address"
			rsp.Elapse = time.Since(tick).String()
			return rsp, nil
		}
		isUnixAddr := false
		isTcpLocal := false
		if addr, ok := p.Addr.(*net.TCPAddr); ok {
			if strings.HasPrefix(addr.String(), "127.0.0.1:") {
				isTcpLocal = true
			} else if strings.HasPrefix(addr.String(), "0:0:0:0:0:0:0:1") {
				isTcpLocal = true
			}
		} else if _, ok := p.Addr.(*net.UnixAddr); ok {
			isUnixAddr = true
		}
		s.log.Infof("shutdown request from %v", p.Addr)
		if !isUnixAddr && !isTcpLocal {
			rsp.Success, rsp.Reason = false, "remote shutdown not allowed"
			rsp.Elapse = time.Since(tick).String()
			return rsp, nil
		}
	}

	booter.NotifySignal()
	rsp.Success, rsp.Reason = true, "success"
	rsp.Elapse = time.Since(tick).String()
	return rsp, nil
}

type ServicePortsRequest struct {
	Service string `json:"service"`
}

type ServicePortsResponse struct {
	Success bool           `json:"success"`
	Reason  string         `json:"reason"`
	Elapse  string         `json:"elapse"`
	Ports   []*ServicePort `json:"ports"`
}

type ServicePort struct {
	Service string `json:"service"`
	Address string `json:"address"`
}

func (s *Server) ServicePorts(ctx context.Context, req *ServicePortsRequest) (*ServicePortsResponse, error) {
	tick := time.Now()
	rsp := &ServicePortsResponse{}

	ret := []*ServicePort{}
	ports, err := s.getServicePorts(req.Service)
	if err != nil {
		return nil, err
	}
	for _, p := range ports {
		ret = append(ret, &ServicePort{
			Service: p.Service,
			Address: p.Address,
		})
	}

	rsp.Ports = ret
	rsp.Elapse = time.Since(tick).String()
	return rsp, nil
}

type ServerInfoResponse struct {
	Success bool     `json:"success"`
	Reason  string   `json:"reason"`
	Elapse  string   `json:"elapse"`
	Version *Version `json:"version"`
	Runtime *Runtime `json:"runtime"`
}

type Version struct {
	Major          int32  `json:"major"`
	Minor          int32  `json:"minor"`
	Patch          int32  `json:"patch"`
	GitSHA         string `json:"gitSHA"`
	BuildTimestamp string `json:"buildTimestamp"`
	BuildCompiler  string `json:"buildCompiler"`
	Engine         string `json:"engine"`
}

type Runtime struct {
	OS             string            `json:"OS,omitempty"`
	Arch           string            `json:"arch,omitempty"`
	Pid            int32             `json:"pid,omitempty"`
	UptimeInSecond int64             `json:"uptimeInSecond,omitempty"`
	Processes      int32             `json:"processes,omitempty"`
	Goroutines     int32             `json:"goroutines,omitempty"`
	Mem            map[string]uint64 `json:"mem,omitempty"`
}

func (s *Server) ServerInfo(ctx context.Context) (*ServerInfoResponse, error) {
	tick := time.Now()
	rsp := &ServerInfoResponse{}
	defer func() {
		if panic := recover(); panic != nil {
			s.log.Error("GetServerInfo panic recover", panic)
		}
		if rsp != nil {
			rsp.Elapse = time.Since(tick).String()
		}
	}()
	if r, err := s.getServerInfo(); err != nil {
		return nil, err
	} else {
		rsp = r
	}

	rsp.Success = true
	rsp.Reason = "success"
	return rsp, nil
}

type SessionsRequest struct {
	Statz      bool `json:"statz"`
	Sessions   bool `json:"sessions"`
	ResetStatz bool `json:"resetStatz"`
}

type SessionsResponse struct {
	Success  bool              `json:"success"`
	Reason   string            `json:"reason"`
	Elapse   string            `json:"elapse"`
	Statz    *server_api.Statz `json:"statz"`
	Sessions []*Session        `json:"sessions"`
}

type Session struct {
	Id            string `json:"id"`
	CreTime       int64  `json:"creTime"`
	LatestSqlTime int64  `json:"latestSqlTime"`
	LatestSql     string `json:"latestSql"`
}

func (s *Server) Sessions(ctx context.Context, req *SessionsRequest) (*SessionsResponse, error) {
	rsp := &SessionsResponse{}
	tick := time.Now()
	defer func() {
		if panic := recover(); panic != nil {
			s.log.Error("Sessions panic recover", panic)
			debug.PrintStack()
		}
		rsp.Elapse = time.Since(tick).String()
	}()

	if req.ResetStatz {
		server_api.ResetQueryStatz()
	}
	if req.Statz {
		rsp.Statz = server_api.StatzSnapshot()
	}
	if req.Sessions {
		sessions := []*Session{}
		if db, ok := api.Default().(*machsvr.Database); ok {
			db.ListWatcher(func(st *machsvr.ConnState) bool {
				sessions = append(sessions, &Session{
					Id:            st.Id,
					CreTime:       st.CreatedTime.UnixNano(),
					LatestSqlTime: st.LatestTime.UnixNano(),
					LatestSql:     st.LatestSql,
				})
				return true
			})
		}
		rsp.Sessions = sessions
	}
	rsp.Success = true
	rsp.Reason = "success"
	return rsp, nil
}

type KillSessionRequest struct {
	Id    string `json:"id"`
	Force bool   `json:"force"`
}

type KillSessionResponse struct {
	Success bool   `json:"success"`
	Reason  string `json:"reason"`
	Elapse  string `json:"elapse"`
}

func (s *Server) KillSession(ctx context.Context, req *KillSessionRequest) (*KillSessionResponse, error) {
	rsp := &KillSessionResponse{}
	tick := time.Now()
	defer func() {
		if panic := recover(); panic != nil {
			s.log.Error("Sessions kill panic recover", panic)
		}
		rsp.Elapse = time.Since(tick).String()
	}()

	if db, ok := api.Default().(*machsvr.Database); ok {
		if err := db.KillConnection(req.Id, req.Force); err != nil {
			rsp.Reason = err.Error()
		} else {
			rsp.Success = true
			rsp.Reason = "success"
		}
	} else {
		rsp.Success = false
		rsp.Reason = "Session kill not supported in head-only mode"
	}
	return rsp, nil
}

type LimitSessionRequest struct {
	Cmd          string `json:"cmd"`                    // get, set
	MaxOpenConn  int32  `json:"maxOpenConn,omitempty"`  // set
	MaxOpenQuery int32  `json:"maxOpenQuery,omitempty"` // set
	MaxPoolSize  int32  `json:"maxPoolSize,omitempty"`  // set
}

type LimitSessionResponse struct {
	Success           bool   `json:"success"`
	Reason            string `json:"reason"`
	Elapse            string `json:"elapse"`
	MaxOpenConn       int32  `json:"maxOpenConn,omitempty"`       // get
	RemainedOpenConn  int32  `json:"remainedOpenConn,omitempty"`  // get
	MaxOpenQuery      int32  `json:"maxOpenQuery,omitempty"`      // get
	RemainedOpenQuery int32  `json:"remainedOpenQuery,omitempty"` // get
	MaxPoolSize       int32  `json:"maxPoolSize,omitempty"`       // get
}

func (s *Server) LimitSession(ctx context.Context, req *LimitSessionRequest) (*LimitSessionResponse, error) {
	rsp := &LimitSessionResponse{}
	tick := time.Now()
	defer func() {
		if panic := recover(); panic != nil {
			s.log.Error("LimitSession panic recover", panic)
		}
		rsp.Elapse = time.Since(tick).String()
	}()

	if db, ok := api.Default().(*machsvr.Database); ok {
		if strings.ToLower(req.Cmd) == "set" {
			if limit := int(req.MaxOpenConn); limit >= -1 {
				db.SetMaxOpenConn(limit)
			}
			if limit := int(req.MaxOpenQuery); limit >= -1 {
				db.SetMaxOpenQuery(limit)
			}
			if limit := int(req.MaxPoolSize); limit >= -1 {
				db.SetWorkerPoolSize(limit)
			}
		}
		limitConn, remainsConn := db.MaxOpenConn()
		limitQuery, remainsQuery := db.MaxOpenQuery()
		rsp.MaxPoolSize = int32(db.WorkerPoolSize())
		rsp.MaxOpenConn = int32(limitConn)
		rsp.RemainedOpenConn = int32(remainsConn)
		rsp.MaxOpenQuery = int32(limitQuery)
		rsp.RemainedOpenQuery = int32(remainsQuery)
		rsp.Success = true
		rsp.Reason = "success"
	} else {
		rsp.Success = false
		rsp.Reason = "Session limit not supported in head-only mode"
	}
	return rsp, nil
}

type HttpDebugModeRequest struct {
	Cmd        string `json:"cmd"`                  // get, set
	Enable     bool   `json:"enable,omitempty"`     // set
	LogLatency int64  `json:"logLatency,omitempty"` // set
}

type HttpDebugModeResponse struct {
	Success    bool   `json:"success"`
	Reason     string `json:"reason"`
	Elapse     string `json:"elapse"`
	Enable     bool   `json:"enable,omitempty"`     // get
	LogLatency int64  `json:"logLatency,omitempty"` // get
}

func (s *Server) HttpDebugMode(ctx context.Context, req *HttpDebugModeRequest) (*HttpDebugModeResponse, error) {
	rsp := &HttpDebugModeResponse{}
	tick := time.Now()
	defer func() {
		if panic := recover(); panic != nil {
			s.log.Error("HttpDebugMode panic recover", panic)
		}
		rsp.Elapse = time.Since(tick).String()
	}()

	if strings.ToLower(req.Cmd) == "set" {
		s.httpd.SetDebugMode(req.Enable, time.Duration(req.LogLatency))
	}
	enable, logLatency := s.httpd.DebugMode()
	rsp.Enable = enable
	rsp.LogLatency = int64(logLatency)
	rsp.Success = true
	rsp.Reason = "success"
	return rsp, nil
}

var maxProcessors int32
var pid int32
var ver *mods.Version

func (s *Server) getServerInfo() (*ServerInfoResponse, error) {
	if maxProcessors == 0 {
		maxProcessors = int32(runtime.GOMAXPROCS(-1))
	}
	if ver == nil {
		ver = mods.GetVersion()
	}
	if pid == 0 {
		pid = int32(os.Getpid())
	}

	rsp := &ServerInfoResponse{
		Version: &Version{
			Engine:         machsvr.LinkInfo(),
			Major:          int32(ver.Major),
			Minor:          int32(ver.Minor),
			Patch:          int32(ver.Patch),
			GitSHA:         ver.GitSHA,
			BuildTimestamp: mods.BuildTimestamp(),
			BuildCompiler:  mods.BuildCompiler(),
		},
		Runtime: &Runtime{
			OS:             runtime.GOOS,
			Arch:           runtime.GOARCH,
			Pid:            pid,
			UptimeInSecond: int64(time.Since(s.startupTime).Seconds()),
			Processes:      maxProcessors,
			Goroutines:     int32(runtime.NumGoroutine()),
		},
	}

	mem := runtime.MemStats{}
	runtime.ReadMemStats(&mem)
	rsp.Runtime.Mem = map[string]uint64{
		"sys":               mem.Sys,
		"alloc":             mem.Alloc,
		"total_alloc":       mem.TotalAlloc,
		"lookups":           mem.Lookups,
		"mallocs":           mem.Mallocs,
		"frees":             mem.Frees,
		"lives":             mem.Mallocs - mem.Frees,
		"heap_alloc":        mem.HeapAlloc,
		"heap_sys":          mem.HeapSys,
		"heap_idle":         mem.HeapIdle,
		"heap_in_use":       mem.HeapInuse,
		"heap_released":     mem.HeapReleased,
		"heap_objects":      mem.HeapObjects,
		"stack_in_use":      mem.StackInuse,
		"stack_sys":         mem.StackSys,
		"mspan_in_use":      mem.MSpanInuse,
		"mspan_sys":         mem.MSpanSys,
		"mcache_in_use":     mem.MCacheInuse,
		"mcache_sys":        mem.MCacheSys,
		"buck_hash_sys":     mem.BuckHashSys,
		"gc_sys":            mem.GCSys,
		"other_sys":         mem.OtherSys,
		"gc_next":           mem.NextGC,
		"gc_last":           mem.LastGC,
		"gc_pause_total_ns": mem.PauseTotalNs,
	}
	return rsp, nil
}
