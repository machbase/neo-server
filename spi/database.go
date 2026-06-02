package spi

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"strconv"
	"strings"
	"time"

	"github.com/machbase/neo-client/api"
)

var defaultDatabase api.Database
var defaultDatabaseKey crypto.PrivateKey

func SetDefault(db api.Database, key crypto.PrivateKey) {
	defaultDatabase = db
	defaultDatabaseKey = key
}

func Default() api.Database {
	return defaultDatabase
}

func DefaultKey() crypto.PrivateKey {
	return defaultDatabaseKey
}

// IssueToken returns signed current timestamp.
// neo-shell uses it as a password for the session.
func IssueToken() string {
	skey := DefaultKey()
	signer, ok := skey.(crypto.Signer)
	if !ok {
		return ""
	}

	ts := strconv.FormatInt(time.Now().UnixMilli(), 10)
	hash := sha256.Sum256([]byte(ts))
	sig, err := signer.Sign(rand.Reader, hash[:], crypto.SHA256)
	if err != nil {
		return ""
	}

	return ts + ":" + base64.RawURLEncoding.EncodeToString(sig)
}

// VerifyToken verifies the token is valid and not expired.
func VerifyToken(token string, ttl time.Duration) bool {
	skey := DefaultKey()
	signer, ok := skey.(crypto.Signer)
	if !ok {
		return false
	}

	parts := strings.SplitN(token, ":", 2)
	if len(parts) != 2 {
		return false
	}

	tsMillis, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return false
	}
	issuedAt := time.UnixMilli(tsMillis)
	now := time.Now()
	if ttl > 0 && now.Sub(issuedAt) > ttl {
		return false
	}

	sig, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return false
	}

	hash := sha256.Sum256([]byte(parts[0]))
	pub := signer.Public()
	switch key := pub.(type) {
	case *rsa.PublicKey:
		return rsa.VerifyPKCS1v15(key, crypto.SHA256, hash[:], sig) == nil
	case *ecdsa.PublicKey:
		return ecdsa.VerifyASN1(key, hash[:], sig)
	default:
		return false
	}
}
