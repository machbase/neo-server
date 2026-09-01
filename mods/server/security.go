package server

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/md5"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gofrs/uuid/v5"
	"github.com/golang-jwt/jwt/v4"
	"github.com/machbase/neo-server/v8/booter"
	"github.com/machbase/neo-server/v8/spi"
	"golang.org/x/crypto/ssh"
)

// regular expression for splitting 'sys as other_user'
var proxyLoginRegex = regexp.MustCompile(`(?i)^(\w+)(?:\s+as\s+(\w+))?$`)

// ParseProxyLoginName parses the login name for proxy login.
// If the login name matches the pattern 'sys as other_user',
// it returns 'other_user' as the login name and 'sys' as the proxy user and true.
// If the login name does not match the pattern,
// it returns the original login name and an empty proxy user and false.
func ParseProxyLoginName(loginName string) (string, string, bool) {
	matches := proxyLoginRegex.FindStringSubmatch(strings.ToLower(loginName))
	proxyUser := ""
	isProxyLogin := false
	if len(matches) == 3 && matches[2] != "" {
		// proxy login, use the second group as the login name
		loginName = matches[2]
		proxyUser = matches[1]
		isProxyLogin = true
	}
	return loginName, proxyUser, isProxyLogin
}

type AuthServer interface {
	ValidateClientToken(ctx context.Context, token string) (string, bool, error)
	ValidateClientCertificate(clientId string, certHash string) (bool, error)
	ValidateUserPublicKey(ctx context.Context, user string, publicKey ssh.PublicKey) (bool, error)
	ValidateUserPassword(ctx context.Context, user string, password string) (bool, string, error)
	ServerPrivateKeyPath() string
}

type JwtCacheValue struct {
	Rt   string
	When time.Time
}

type JwtCache interface {
	SetRefreshToken(id string, rt string)
	GetRefreshToken(id string) (string, bool)
	RemoveRefreshToken(id string)
}

type jwtMemCache struct {
	rtTable map[string]*JwtCacheValue
	rtLock  sync.RWMutex
}

func NewJwtCache() JwtCache {
	return &jwtMemCache{
		rtTable: make(map[string]*JwtCacheValue),
	}
}

func (svr *jwtMemCache) SetRefreshToken(id string, rt string) {
	svr.rtLock.Lock()
	defer svr.rtLock.Unlock()
	svr.rtTable[id] = &JwtCacheValue{
		Rt:   rt,
		When: time.Now(),
	}
}

func (svr *jwtMemCache) GetRefreshToken(id string) (string, bool) {
	svr.rtLock.RLock()
	defer svr.rtLock.RUnlock()
	val, ok := svr.rtTable[id]
	if val != nil {
		return val.Rt, ok
	} else {
		return "", ok
	}
}

func (svr *jwtMemCache) RemoveRefreshToken(id string) {
	svr.rtLock.Lock()
	defer svr.rtLock.Unlock()
	delete(svr.rtTable, id)
}

type JwtConfig struct {
	AtDuration time.Duration
	RtDuration time.Duration
	Secret     string
}

var jwtConf = &JwtConfig{
	AtDuration: 5 * time.Minute,
	RtDuration: 60 * time.Minute,
	Secret:     "__secr3t__",
}

func JwtConfigure(conf *JwtConfig) error {
	if conf != nil && conf.AtDuration > 0 && conf.RtDuration > 0 {
		jwtConf = conf
	}
	return nil
}

var idgen = uuid.NewGen()

type Claim = *jwt.RegisteredClaims

func NewClaimEmpty() Claim {
	return &jwt.RegisteredClaims{}
}

func NewClaim(loginName string) Claim {
	id, _ := idgen.NewV6()
	claim := &jwt.RegisteredClaims{
		Issuer:    "machbase-neo",
		Subject:   loginName,
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(jwtConf.AtDuration)),
		NotBefore: jwt.NewNumericDate(time.Now()),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
		ID:        id.String(),
	}
	return claim
}

func NewClaimForRefresh(claim Claim) Claim {
	c := NewClaim(claim.Subject)
	c.ExpiresAt = jwt.NewNumericDate(time.Now().Add(jwtConf.RtDuration))
	return c
}

func SignTokenWithClaim(claim Claim) (string, error) {
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claim)
	signedTok, err := tok.SignedString([]byte(jwtConf.Secret))
	return signedTok, err
}

func VerifyToken(token string) (bool, error) {
	return VerifyTokenWithClaim(token, nil)
}

func VerifyTokenWithClaim(token string, claim Claim) (bool, error) {
	if claim == nil {
		claim = &jwt.RegisteredClaims{}
	}
	key := func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return false, errors.New("unexpected signing method")
		}
		return []byte(jwtConf.Secret), nil
	}

	tok, err := jwt.ParseWithClaims(token, claim, key)
	if err != nil {
		return false, err
	}
	return tok.Valid, nil
}

type EllipticCurve struct {
	pubKeyCurve elliptic.Curve
	privateKey  *ecdsa.PrivateKey
	publicKey   *ecdsa.PublicKey
}

func NewEllipticCurveP256() *EllipticCurve {
	return NewEllipticCurve(elliptic.P256())
}

func NewEllipticCurveP521() *EllipticCurve {
	return NewEllipticCurve(elliptic.P521())
}

func NewEllipticCurve(curve elliptic.Curve) *EllipticCurve {
	return &EllipticCurve{
		pubKeyCurve: curve,
		privateKey:  new(ecdsa.PrivateKey),
	}
}

func (ec *EllipticCurve) GenerateKeys() (*ecdsa.PrivateKey, *ecdsa.PublicKey, error) {
	var err error
	privKey, err := ecdsa.GenerateKey(ec.pubKeyCurve, rand.Reader)
	if err != nil {
		return nil, nil, err
	}
	ec.privateKey = privKey
	ec.publicKey = &privKey.PublicKey
	return ec.privateKey, ec.publicKey, nil
}

// EncodePrivate private key
func (ec *EllipticCurve) EncodePrivate(privKey *ecdsa.PrivateKey) (string, error) {
	encoded, err := x509.MarshalECPrivateKey(privKey)
	if err != nil {
		return "", err
	}
	pemEncoded := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: encoded})
	return string(pemEncoded), nil
}

// EncodePublic public key
func (ec *EllipticCurve) EncodePublic(pubKey *ecdsa.PublicKey) (string, error) {
	encoded, err := x509.MarshalPKIXPublicKey(pubKey)
	if err != nil {
		return "", err
	}
	pemEncodedPub := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: encoded})
	return string(pemEncodedPub), nil
}

// DecodePrivate private key
func (ec *EllipticCurve) DecodePrivate(pemEncodedPriv string) (*ecdsa.PrivateKey, error) {
	blockPriv, _ := pem.Decode([]byte(pemEncodedPriv))
	x509EncodedPriv := blockPriv.Bytes
	privateKey, err := x509.ParseECPrivateKey(x509EncodedPriv)
	return privateKey, err
}

// DecodePublic public key
func (ec *EllipticCurve) DecodePublic(pemEncodedPub string) (*ecdsa.PublicKey, error) {
	blockPub, _ := pem.Decode([]byte(pemEncodedPub))
	x509EncodedPub := blockPub.Bytes
	genericPublicKey, err := x509.ParsePKIXPublicKey(x509EncodedPub)
	publicKey := genericPublicKey.(*ecdsa.PublicKey)
	return publicKey, err
}

// VerifySignature sign ecdsa style and verify signature
func (ec *EllipticCurve) VerifySignature(privKey *ecdsa.PrivateKey, pubKey *ecdsa.PublicKey) ([]byte, bool, error) {
	h := md5.New()
	io.WriteString(h, "This is a message to be signed and verified by ECDSA!")
	signhash := h.Sum(nil)

	r, s, serr := ecdsa.Sign(rand.Reader, privKey, signhash)
	if serr != nil {
		return []byte(""), false, serr
	}

	signature := r.Bytes()
	signature = append(signature, s.Bytes()...)

	verify := ecdsa.Verify(pubKey, signhash, r, s)

	return signature, verify, nil
}

// Test encode, decode and test it with deep equal
func (ec *EllipticCurve) Test(privKey *ecdsa.PrivateKey, pubKey *ecdsa.PublicKey) error {
	encPriv, err := ec.EncodePrivate(privKey)
	if err != nil {
		return err
	}
	encPub, err := ec.EncodePublic(pubKey)
	if err != nil {
		return err
	}
	priv2, err := ec.DecodePrivate(encPriv)
	if err != nil {
		return err
	}
	pub2, err := ec.DecodePublic(encPub)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(privKey, priv2) {
		return errors.New("private keys do not match")
	}
	if !reflect.DeepEqual(pubKey, pub2) {
		return errors.New("public keys do not match")
	}
	return nil
}

// listSshKeys returns authorized SSH keys for the current user.
//
// params:
//
// return: authorized SSH key list
func (s *Server) listSshKeys(ctx context.Context) ([]*AuthorizedSshKey, error) {
	// typ exists for compatibility with ssh key types.
	user := "sys"
	if c, ok := ctx.(*gin.Context); ok {
		claim, ok := s.httpd.getJwtClaim(c)
		if !ok || claim == nil {
			return nil, fmt.Errorf("unauthenticated")
		}
		user = claim.Subject
	}
	rsp, err := s.ListSshKey(ctx, user)
	if err != nil {
		return nil, fmt.Errorf("fail to list ssh keys, %s", err.Error())
	}
	return rsp.SshKeys, nil
}

type ListSshKeyResponse struct {
	Success bool                `json:"success"`
	Reason  string              `json:"reason"`
	Elapse  string              `json:"elapse"`
	SshKeys []*AuthorizedSshKey `json:"sshKeys"`
}

func (s *Server) ListSshKey(ctx context.Context, user string) (*ListSshKeyResponse, error) {
	tick := time.Now()
	rsp := &ListSshKeyResponse{Reason: "not-implemented"}
	defer func() {
		rsp.Elapse = time.Since(tick).String()
	}()

	conn, err := spi.Connect(ctx, "sys")
	if err != nil {
		return nil, fmt.Errorf("fail to connect database, %s", err.Error())
	}
	defer conn.Close()

	keys, err := getUserAuthKeys(ctx, conn, user)
	if err != nil {
		return nil, fmt.Errorf("fail to get user auth keys, %s", err.Error())
	}
	for _, k := range keys {
		sk, err := ConvertUserAuthInfoToAuthorizedSshKey(k)
		if err != nil {
			s.log.Warnf("fail to convert user auth key to authorized ssh key, %s", err.Error())
			continue
		}
		rsp.SshKeys = append(rsp.SshKeys, sk)
	}
	rsp.Success, rsp.Reason = true, "success"
	return rsp, nil
}

func ConvertUserAuthInfoToAuthorizedSshKey(k *UserAuthKeyInfo) (*AuthorizedSshKey, error) {
	if k == nil {
		return nil, fmt.Errorf("user auth key info is nil")
	}

	block, _ := pem.Decode([]byte(strings.TrimSpace(k.PubKey)))
	if block == nil {
		return nil, fmt.Errorf("fail to decode public key, invalid PEM format")
	}

	var pubAny any
	var err error

	switch block.Type {
	case "PUBLIC KEY":
		pubAny, err = x509.ParsePKIXPublicKey(block.Bytes)
	case "RSA PUBLIC KEY":
		pubAny, err = x509.ParsePKCS1PublicKey(block.Bytes)
	case "CERTIFICATE":
		var cert *x509.Certificate
		cert, err = x509.ParseCertificate(block.Bytes)
		if err == nil {
			pubAny = cert.PublicKey
		}
	default:
		pubAny, err = x509.ParsePKIXPublicKey(block.Bytes)
		if err != nil {
			if rsaPub, errRSA := x509.ParsePKCS1PublicKey(block.Bytes); errRSA == nil {
				pubAny = rsaPub
				err = nil
			}
		}
	}
	if err != nil {
		return nil, fmt.Errorf("fail to parse public key, %s", err.Error())
	}

	sshPub, err := ssh.NewPublicKey(pubAny)
	if err != nil {
		return nil, fmt.Errorf("fail to convert to ssh public key, %s", err.Error())
	}

	return &AuthorizedSshKey{
		KeyType:     sshPub.Type(),
		Fingerprint: ssh.FingerprintSHA256(sshPub),
		Comment:     k.Comment,
	}, nil
}

func ConvertAuthorizedSshKeyToUserAuthInfo(k *AuthorizedSshKey) (*UserAuthKeyInfo, error) {
	if k == nil {
		return nil, fmt.Errorf("authorized ssh key is nil")
	}

	keyRaw := strings.TrimSpace(k.Key)
	if keyRaw == "" {
		return nil, fmt.Errorf("ssh public key is empty")
	}

	pubKey, _, _, _, err := ssh.ParseAuthorizedKey([]byte(keyRaw))
	if err != nil {
		return nil, fmt.Errorf("fail to parse authorized public key, %s", err.Error())
	}

	cryptoPub, ok := pubKey.(ssh.CryptoPublicKey)
	if !ok {
		return nil, fmt.Errorf("unsupported key type")
	}

	pubDer, err := x509.MarshalPKIXPublicKey(cryptoPub.CryptoPublicKey())
	if err != nil {
		return nil, fmt.Errorf("fail to encode public key, %s", err.Error())
	}

	pubKeyPem := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubDer})
	return &UserAuthKeyInfo{
		PubKey:  string(pubKeyPem),
		Comment: k.Comment,
	}, nil
}

// addSshKey adds an authorized SSH public key.
//
// params:
//   - typ: SSH key type prefix from authorized key format
//   - key: SSH public key body
//   - comment: key comment text
//
// return: null on success
func (s *Server) addSshKey(ctx context.Context, typ string, key string, comment string) error {
	return s.AddAuthorizedSshKey(ctx, "sys", strings.Join([]string{typ, key, comment}, " "))
}

func (s *Server) AddAuthorizedSshKey(ctx context.Context, user string, rawKey string) error {
	conn, err := spi.Connect(ctx, "sys")
	if err != nil {
		return fmt.Errorf("fail to connect database, %s", err.Error())
	}
	defer conn.Close()

	rawKey = strings.TrimSpace(rawKey)
	var (
		sshPub  ssh.PublicKey
		keyPEM  []byte
		comment string
	)

	if block, _ := pem.Decode([]byte(rawKey)); block != nil {
		var pubAny any
		pubAny, err = x509.ParsePKIXPublicKey(block.Bytes)
		if err != nil {
			if rsaPub, errRSA := x509.ParsePKCS1PublicKey(block.Bytes); errRSA == nil {
				pubAny = rsaPub
				err = nil
			}
		}
		if err != nil {
			return fmt.Errorf("invalid PEM key format, %s", err.Error())
		}

		sshPub, err = ssh.NewPublicKey(pubAny)
		if err != nil {
			return fmt.Errorf("unsupported key type, %s", err.Error())
		}

		der, err := x509.MarshalPKIXPublicKey(pubAny)
		if err != nil {
			return fmt.Errorf("failed to encode ssh key, %s", err.Error())
		}

		keyPEM = pem.EncodeToMemory(&pem.Block{
			Type:  "PUBLIC KEY",
			Bytes: der,
		})
	} else {
		sshPub, comment, _, _, err = ssh.ParseAuthorizedKey([]byte(rawKey))
		if err != nil {
			return fmt.Errorf("invalid key format, %s", err.Error())
		}

		cryptoPub, ok := sshPub.(ssh.CryptoPublicKey)
		if !ok {
			return fmt.Errorf("unsupported key type")
		}

		der, err := x509.MarshalPKIXPublicKey(cryptoPub.CryptoPublicKey())
		if err != nil {
			return fmt.Errorf("failed to encode ssh key, %s", err.Error())
		}

		keyPEM = pem.EncodeToMemory(&pem.Block{
			Type:  "PUBLIC KEY",
			Bytes: der,
		})
	}

	if len(keyPEM) == 0 {
		return fmt.Errorf("failed to convert key to PEM")
	}

	comment = strings.ReplaceAll(strings.TrimSpace(comment), "'", "''")
	// 30 years later
	validBefore := time.Now().Add(time.Hour * 24 * 365 * 30).Format("2006-01-02")

	_, err = conn.ExecContext(ctx,
		fmt.Sprintf("ALTER USER %s ADD AUTH KEY (KEY = '%s', VALID_BEFORE = '%s', COMMENT = '%s')",
			strings.ToUpper(user), strings.TrimSpace(string(keyPEM)), validBefore, comment))
	if err != nil {
		return fmt.Errorf("fail to register user auth key, %s", err.Error())
	}
	return nil
}

// deleteSshKey removes an authorized SSH key by fingerprint.
//
// params:
//   - fingerprint: SSH key fingerprint
//
// return: null on success
func (s *Server) deleteSshKey(ctx context.Context, fingerprint string) error {
	user := "sys"
	if c, ok := ctx.(*gin.Context); ok {
		claim, ok := s.httpd.getJwtClaim(c)
		if !ok || claim == nil {
			return fmt.Errorf("unauthenticated")
		}
		user = claim.Subject
	}

	rsp, err := s.DelSshKey(ctx, &DelSshKeyRequest{User: user, Fingerprint: fingerprint})
	if err != nil {
		return err
	}
	if !rsp.Success {
		return errors.New(rsp.Reason)
	}
	return nil
}

type DelSshKeyRequest struct {
	User        string `json:"-"`
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

	user := "sys"
	if req.User != "" {
		user = req.User
	}

	conn, err := spi.Connect(ctx, "sys")
	if err != nil {
		return nil, fmt.Errorf("fail to connect database, %s", err.Error())
	}
	defer conn.Close()

	keys, err := getUserAuthKeys(ctx, conn, user)
	if err != nil {
		return nil, fmt.Errorf("fail to get user auth keys, %s", err.Error())
	}
	for _, k := range keys {
		sk, err := ConvertUserAuthInfoToAuthorizedSshKey(k)
		if err != nil {
			s.log.Warnf("fail to convert user auth key to authorized ssh key, %s", err.Error())
			continue
		}
		if sk.Fingerprint == req.Fingerprint {
			// found the key to delete
			err := dropUserAuthKey(ctx, conn, user, k.KeyID)
			if err != nil {
				return nil, fmt.Errorf("fail to delete user auth key, %s", err.Error())
			}
			rsp.Success, rsp.Reason = true, "success"
			return rsp, nil
		}
	}
	rsp.Success, rsp.Reason = false, "key not found"
	return rsp, nil
}

type ShutdownResponse struct {
	Success bool   `json:"success"`
	Reason  string `json:"reason"`
	Elapse  string `json:"elapse"`
}

// Shutdown requests server shutdown from a local caller.
//
// params:
//
// return: shutdown status
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

	booter.NotifySignal()
	rsp.Success, rsp.Reason = true, "success"
	rsp.Elapse = time.Since(tick).String()
	return rsp, nil
}

type Session struct {
	Id            string `json:"id"`
	CreTime       int64  `json:"creTime"`
	LatestSqlTime int64  `json:"latestSqlTime"`
	LatestSql     string `json:"latestSql"`
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
