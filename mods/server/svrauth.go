package server

import (
	"crypto/x509"
	"errors"
	"sync"
	"time"

	"github.com/gofrs/uuid/v5"
	"github.com/golang-jwt/jwt/v4"
	"golang.org/x/crypto/ssh"
)

type AuthServer interface {
	ValidateClientToken(token string) (bool, error)
	ValidateClientCertificate(clientId string, certHash string) (bool, error)
	ValidateUserPublicKey(user string, publicKey ssh.PublicKey) (bool, string, error)
	ValidateUserPassword(user string, password string) (bool, string, error)
	ValidateUserOtp(user string, otp string) (bool, error)
	GenerateOtp(user string) (string, error)
	GenerateSnowflake() string
}

type AuthHandler interface {
	Enabled() bool
	AuthId(id string, password string) (bool, error)
	AuthCert(cert *x509.Certificate) (bool, error)
}

func NewAuthenticator(serverCertFile string, authorizedKeysDir string, enabled bool) AuthHandler {
	return &authZero{
		enabled: enabled,
	}
}

type authZero struct {
	enabled bool
}

func (az *authZero) Enabled() bool {
	return az.enabled
}

func (az *authZero) AuthId(id string, password string) (bool, error) {
	return true, nil
}

func (az *authZero) AuthCert(cert *x509.Certificate) (bool, error) {
	return true, nil
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

func JwtConfigure(conf *JwtConfig) {
	jwtConf = conf
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
