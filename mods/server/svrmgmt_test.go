package server

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	crand "crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
)

type sshTestFixture struct {
	AuthorizedKey string
	PEM           string
	Fingerprint   string
}

func newTestSSHKeyFixture(t *testing.T) sshTestFixture {
	t.Helper()

	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), crand.Reader)
	require.NoError(t, err)

	sshPub, err := ssh.NewPublicKey(&privateKey.PublicKey)
	require.NoError(t, err)

	publicKeyDer, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	require.NoError(t, err)

	return sshTestFixture{
		AuthorizedKey: strings.TrimSpace(string(ssh.MarshalAuthorizedKey(sshPub))),
		PEM: strings.TrimSpace(string(pem.EncodeToMemory(&pem.Block{
			Type:  "PUBLIC KEY",
			Bytes: publicKeyDer,
		}))),
		Fingerprint: ssh.FingerprintSHA256(sshPub),
	}
}

func TestClientTokenRSA(t *testing.T) {
	rc, err := rsa.GenerateKey(rand.Reader, 4096)
	require.Nil(t, err)

	token, err := GenerateClientToken("abcdefg", rc, "b")
	require.Nil(t, err)
	require.True(t, len(token) > 0)
	// t.Logf("Token: %s", token)

	pass, err := VerifyClientToken(token, &rc.PublicKey)
	require.Nil(t, err)
	require.True(t, pass)

	pass, err = VerifyClientToken(token+"wrong", rc)
	require.NotNil(t, err)
	require.False(t, pass)
}

func TestClientTokenECDSA(t *testing.T) {
	ec := NewEllipticCurveP256()
	pri, pub, err := ec.GenerateKeys()
	require.Nil(t, err)
	require.NotNil(t, pri)
	require.NotNil(t, pub)

	token, err := GenerateClientToken("abcdefg", pri, "b")
	require.Nil(t, err)
	require.True(t, len(token) > 0)
	t.Logf("Token: %s", token)

	pass, err := VerifyClientToken(token, &pri.PublicKey)
	require.Nil(t, err)
	require.True(t, pass)

	pass, err = VerifyClientToken(token+"wrong", pri)
	require.NotNil(t, err)
	require.False(t, pass)
}

func TestConvertUserAuthInfoToAuthorizedSshKey(t *testing.T) {
	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	pubDER, err := x509.MarshalPKIXPublicKey(&rsaKey.PublicKey)
	require.NoError(t, err)

	pubPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubDER})
	require.NotEmpty(t, pubPEM)

	expectedSSHPub, err := ssh.NewPublicKey(&rsaKey.PublicKey)
	require.NoError(t, err)

	k := &UserAuthKeyInfo{PubKey: string(pubPEM), Comment: "test key"}
	authKey, err := ConvertUserAuthInfoToAuthorizedSshKey(k)
	require.NoError(t, err)
	require.NotNil(t, authKey)
	require.Equal(t, expectedSSHPub.Type(), authKey.KeyType)
	require.Equal(t, ssh.FingerprintSHA256(expectedSSHPub), authKey.Fingerprint)
	require.Equal(t, "test key", authKey.Comment)

	// invalid PEM format
	k.PubKey = "invalid PEM format"
	authKey, err = ConvertUserAuthInfoToAuthorizedSshKey(k)
	require.Error(t, err)
	require.Nil(t, authKey)

	// valid PEM envelope but invalid key payload
	k.PubKey = "-----BEGIN PUBLIC KEY-----\nAAAA\n-----END PUBLIC KEY-----"
	authKey, err = ConvertUserAuthInfoToAuthorizedSshKey(k)
	require.Error(t, err)
	require.Nil(t, authKey)
}

func TestConvertAuthorizedSshKeyToUserAuthInfo(t *testing.T) {
	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	sshPub, err := ssh.NewPublicKey(&rsaKey.PublicKey)
	require.NoError(t, err)

	input := &AuthorizedSshKey{
		Key:     string(ssh.MarshalAuthorizedKey(sshPub)),
		Comment: "test key",
	}

	userAuth, err := ConvertAuthorizedSshKeyToUserAuthInfo(input)
	require.NoError(t, err)
	require.NotNil(t, userAuth)
	require.Equal(t, "test key", userAuth.Comment)

	block, _ := pem.Decode([]byte(userAuth.PubKey))
	require.NotNil(t, block)
	require.Equal(t, "PUBLIC KEY", block.Type)

	parsedPubAny, err := x509.ParsePKIXPublicKey(block.Bytes)
	require.NoError(t, err)

	parsedSSHPub, err := ssh.NewPublicKey(parsedPubAny)
	require.NoError(t, err)
	require.Equal(t, sshPub.Type(), parsedSSHPub.Type())
	require.Equal(t, ssh.FingerprintSHA256(sshPub), ssh.FingerprintSHA256(parsedSSHPub))

	_, err = ConvertAuthorizedSshKeyToUserAuthInfo(&AuthorizedSshKey{Key: "invalid key"})
	require.Error(t, err)
}
