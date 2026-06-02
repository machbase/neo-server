package server

import (
	"crypto/rand"
	"crypto/rsa"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/stretchr/testify/require"
)

func TestParseProxyLoginName(t *testing.T) {
	tests := []struct {
		name          string
		loginName     string
		wantLoginName string
		wantProxyUser string
		wantIsProxy   bool
	}{
		{
			name:          "normal login",
			loginName:     "user",
			wantLoginName: "user",
			wantProxyUser: "",
			wantIsProxy:   false,
		},
		{
			name:          "proxy login with sys",
			loginName:     "sys as other_user",
			wantLoginName: "other_user",
			wantProxyUser: "sys",
			wantIsProxy:   true,
		},
		{
			name:          "proxy login with different case",
			loginName:     "SYS as OTHER_USER",
			wantLoginName: "other_user",
			wantProxyUser: "sys",
			wantIsProxy:   true,
		},
		{
			name:          "invalid proxy login",
			loginName:     "sys as",
			wantLoginName: "sys as",
			wantProxyUser: "",
			wantIsProxy:   false,
		},
		{
			name:          "invalid proxy login with extra spaces",
			loginName:     "sys   as   other_user",
			wantLoginName: "other_user",
			wantProxyUser: "sys",
			wantIsProxy:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotLoginName, gotProxyUser, gotIsProxy := ParseProxyLoginName(tt.loginName)
			require.Equal(t, tt.wantLoginName, gotLoginName)
			require.Equal(t, tt.wantProxyUser, gotProxyUser)
			require.Equal(t, tt.wantIsProxy, gotIsProxy)
		})
	}
}

func withTestJwtConfig(t *testing.T, conf *JwtConfig) {
	t.Helper()
	prev := jwtConf
	JwtConfigure(conf)
	t.Cleanup(func() {
		JwtConfigure(prev)
	})
}

func TestNewAuthenticatorAndAuthZero(t *testing.T) {
	auth := NewAuthenticator("server.crt", "authorized_keys", true)
	require.NotNil(t, auth)
	require.True(t, auth.Enabled())

	ok, err := auth.AuthId("user", "password")
	require.NoError(t, err)
	require.True(t, ok)

	ok, err = auth.AuthCert(nil)
	require.NoError(t, err)
	require.True(t, ok)

	disabled := NewAuthenticator("server.crt", "authorized_keys", false)
	require.False(t, disabled.Enabled())
}

func TestJwtMemCacheLifecycle(t *testing.T) {
	cache := NewJwtCache()
	require.NotNil(t, cache)

	value, ok := cache.GetRefreshToken("missing")
	require.False(t, ok)
	require.Empty(t, value)

	cache.SetRefreshToken("user", "refresh-token")
	value, ok = cache.GetRefreshToken("user")
	require.True(t, ok)
	require.Equal(t, "refresh-token", value)

	cache.RemoveRefreshToken("user")
	value, ok = cache.GetRefreshToken("user")
	require.False(t, ok)
	require.Empty(t, value)
}

func TestNewClaimEmpty(t *testing.T) {
	claim := NewClaimEmpty()
	require.NotNil(t, claim)
	require.Empty(t, claim.Subject)
	require.Empty(t, claim.Issuer)
	require.Nil(t, claim.ExpiresAt)
}

func TestNewClaimAndRefreshClaim(t *testing.T) {
	withTestJwtConfig(t, &JwtConfig{
		AtDuration: 2 * time.Minute,
		RtDuration: 15 * time.Minute,
		Secret:     "claim-secret",
	})

	before := time.Now()
	claim := NewClaim("neo-user")
	require.Equal(t, "machbase-neo", claim.Issuer)
	require.Equal(t, "neo-user", claim.Subject)
	require.NotNil(t, claim.IssuedAt)
	require.NotNil(t, claim.NotBefore)
	require.NotNil(t, claim.ExpiresAt)
	require.NotEmpty(t, claim.ID)
	require.WithinDuration(t, before, claim.IssuedAt.Time, 2*time.Second)
	require.WithinDuration(t, before, claim.NotBefore.Time, 2*time.Second)
	require.WithinDuration(t, before.Add(2*time.Minute), claim.ExpiresAt.Time, 2*time.Second)

	refresh := NewClaimForRefresh(claim)
	require.Equal(t, claim.Subject, refresh.Subject)
	require.Equal(t, "machbase-neo", refresh.Issuer)
	require.NotEmpty(t, refresh.ID)
	require.NotEqual(t, claim.ID, refresh.ID)
	require.True(t, refresh.ExpiresAt.Time.After(claim.ExpiresAt.Time))
	require.WithinDuration(t, time.Now().Add(15*time.Minute), refresh.ExpiresAt.Time, 2*time.Second)
}

func TestSignAndVerifyToken(t *testing.T) {
	withTestJwtConfig(t, &JwtConfig{
		AtDuration: time.Minute,
		RtDuration: 5 * time.Minute,
		Secret:     "sign-secret",
	})

	claim := NewClaim("jwt-user")
	token, err := SignTokenWithClaim(claim)
	require.NoError(t, err)
	require.NotEmpty(t, token)

	parsedClaim := NewClaimEmpty()
	valid, err := VerifyTokenWithClaim(token, parsedClaim)
	require.NoError(t, err)
	require.True(t, valid)
	require.Equal(t, claim.Subject, parsedClaim.Subject)
	require.Equal(t, claim.Issuer, parsedClaim.Issuer)
	require.Equal(t, claim.ID, parsedClaim.ID)

	valid, err = VerifyToken(token)
	require.NoError(t, err)
	require.True(t, valid)
}

func TestVerifyTokenFailures(t *testing.T) {
	withTestJwtConfig(t, &JwtConfig{
		AtDuration: time.Minute,
		RtDuration: 5 * time.Minute,
		Secret:     "verify-secret",
	})

	valid, err := VerifyToken("not-a-token")
	require.Error(t, err)
	require.False(t, valid)

	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	rsaToken := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.RegisteredClaims{Subject: "jwt-user"})
	signed, err := rsaToken.SignedString(rsaKey)
	require.NoError(t, err)

	valid, err = VerifyToken(signed)
	require.Error(t, err)
	require.ErrorContains(t, err, "unexpected signing method")
	require.False(t, valid)

	otherConf := &JwtConfig{
		AtDuration: time.Minute,
		RtDuration: 5 * time.Minute,
		Secret:     "other-secret",
	}
	JwtConfigure(otherConf)

	claim := NewClaim("jwt-user")
	token, err := SignTokenWithClaim(claim)
	require.NoError(t, err)

	JwtConfigure(&JwtConfig{
		AtDuration: time.Minute,
		RtDuration: 5 * time.Minute,
		Secret:     "wrong-secret",
	})

	valid, err = VerifyToken(token)
	require.Error(t, err)
	require.False(t, valid)
}
