package security

import (
	"time"

	"github.com/gofrs/uuid/v5"
	"github.com/golang-jwt/jwt/v4"
	"github.com/pkg/errors"
)

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
