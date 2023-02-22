package server_test

import (
	"crypto/rand"
	"crypto/rsa"
	"testing"

	"github.com/machbase/neo-server/mods/server"
	"github.com/stretchr/testify/require"
)

func TestClientTokenRSA(t *testing.T) {
	rc, err := rsa.GenerateKey(rand.Reader, 512)
	require.Nil(t, err)

	token, err := server.GenerateClientToken("abcdefg", rc, "b")
	require.Nil(t, err)
	require.True(t, len(token) > 0)
	t.Logf("Token: %s", token)

	pass, err := server.VerifyClientToken(token, &rc.PublicKey)
	require.Nil(t, err)
	require.True(t, pass)

	pass, err = server.VerifyClientToken(token+"wrong", rc)
	require.NotNil(t, err)
	require.False(t, pass)
}

func TestClientTokenECDSA(t *testing.T) {
	ec := server.NewEllipticCurveP521()
	pri, pub, err := ec.GenerateKeys()
	require.Nil(t, err)
	require.NotNil(t, pri)
	require.NotNil(t, pub)

	token, err := server.GenerateClientToken("abcdefg", pri, "b")
	require.Nil(t, err)
	require.True(t, len(token) > 0)
	t.Logf("Token: %s", token)

	pass, err := server.VerifyClientToken(token, &pri.PublicKey)
	require.Nil(t, err)
	require.True(t, pass)

	pass, err = server.VerifyClientToken(token+"wrong", pri)
	require.NotNil(t, err)
	require.False(t, pass)
}
