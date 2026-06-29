package server

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	crand "crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/machbase/neo-client/api"
	"github.com/machbase/neo-server/v8/mods/model"
	"github.com/machbase/neo-server/v8/spi"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
)

type coverageStubDB struct{}

func (d *coverageStubDB) Connect(ctx context.Context, options ...api.ConnectOption) (api.Conn, error) {
	return nil, errors.New("not implemented")
}

func (d *coverageStubDB) UserAuth(ctx context.Context, user string, password string) (bool, string, error) {
	return false, "", nil
}

func (d *coverageStubDB) Ping(ctx context.Context) (time.Duration, error) {
	return 0, nil
}

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

func TestServicePortsResponse(t *testing.T) {
	svr := &Server{
		servicePorts: map[string][]*model.ServicePort{
			"http": {
				{Service: "http", Address: "tcp://127.0.0.1:5654"},
				{Service: "http", Address: "unix:///tmp/neo-http.sock"},
			},
		},
	}

	rsp, err := svr.ServicePorts(context.Background(), &ServicePortsRequest{Service: "http"})
	require.NoError(t, err)
	require.NotNil(t, rsp)
	require.Len(t, rsp.Ports, 2)
	require.Equal(t, "http", rsp.Ports[0].Service)
	require.NotEmpty(t, rsp.Elapse)
}

func TestServerInfoResponse(t *testing.T) {
	svr := &Server{startupTime: time.Now().Add(-2 * time.Second)}

	rsp, err := svr.ServerInfo(context.Background())
	require.NoError(t, err)
	require.NotNil(t, rsp)
	require.True(t, rsp.Success)
	require.Equal(t, "success", rsp.Reason)
	require.NotNil(t, rsp.Version)
	require.NotNil(t, rsp.Runtime)
	require.NotEmpty(t, rsp.Elapse)
}

func TestSessionsAndLimitsInHeadOnlyDBMode(t *testing.T) {
	svr := &Server{}

	sessionsRsp, err := svr.Sessions(context.Background(), &SessionsRequest{Sessions: true})
	require.NoError(t, err)
	require.NotNil(t, sessionsRsp)
	require.True(t, sessionsRsp.Success)
	require.Equal(t, "success", sessionsRsp.Reason)
	require.NotNil(t, sessionsRsp.Sessions)

	killRsp, err := svr.KillSession(context.Background(), &KillSessionRequest{Id: "missing", Force: false})
	require.NoError(t, err)
	require.NotNil(t, killRsp)
	require.False(t, killRsp.Success)
	require.Contains(t, killRsp.Reason, "not supported")

	limitRsp, err := svr.LimitSession(context.Background(), &LimitSessionRequest{Cmd: "get"})
	require.NoError(t, err)
	require.NotNil(t, limitRsp)
	require.True(t, limitRsp.Success)
	require.Equal(t, "success", limitRsp.Reason)
}

func TestShutdownRejectsRemoteGinRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodPost, "/web/api/shutdown", nil)
	req.RemoteAddr = "8.8.8.8:12345"
	c.Request = req

	svr := &Server{}
	rsp, err := svr.Shutdown(c)
	require.Nil(t, rsp)
	require.Error(t, err)
	require.Contains(t, err.Error(), "remote shutdown not allowed")
}

func TestHttpDebugModeRPC(t *testing.T) {
	svr := &Server{httpd: &httpd{}}

	rsp, err := svr.HttpDebugMode(context.Background(), &HttpDebugModeRequest{Cmd: "set", Enable: true, LogLatency: int64(250 * time.Millisecond)})
	require.NoError(t, err)
	require.NotNil(t, rsp)
	require.True(t, rsp.Success)
	require.True(t, rsp.Enable)
	require.Equal(t, int64(250*time.Millisecond), rsp.LogLatency)

	rsp, err = svr.HttpDebugMode(context.Background(), &HttpDebugModeRequest{Cmd: "get"})
	require.NoError(t, err)
	require.NotNil(t, rsp)
	require.True(t, rsp.Success)
	require.True(t, rsp.Enable)
}

func TestSessionRPCDefaultDatabaseBranches(t *testing.T) {
	oldDB := spi.Default()
	oldKey := spi.DefaultKey()
	spi.SetDefault(&coverageStubDB{}, oldKey)
	t.Cleanup(func() {
		spi.SetDefault(oldDB, oldKey)
	})

	svr := &Server{}

	sessionsRsp, err := svr.Sessions(context.Background(), &SessionsRequest{Sessions: true})
	require.NoError(t, err)
	require.NotNil(t, sessionsRsp)
	require.True(t, sessionsRsp.Success)
	require.Equal(t, "success", sessionsRsp.Reason)

	killRsp, err := svr.KillSession(context.Background(), &KillSessionRequest{Id: "x", Force: false})
	require.NoError(t, err)
	require.NotNil(t, killRsp)
	require.False(t, killRsp.Success)
	require.Contains(t, killRsp.Reason, "not supported")

	limitRsp, err := svr.LimitSession(context.Background(), &LimitSessionRequest{Cmd: "set", MaxOpenConn: 5, MaxOpenQuery: 5})
	require.NoError(t, err)
	require.NotNil(t, limitRsp)
	require.False(t, limitRsp.Success)
	require.Contains(t, limitRsp.Reason, "not supported")
}
