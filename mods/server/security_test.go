package server

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	crand "crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v4"
	client "github.com/machbase/neo-client/v2"
	"github.com/machbase/neo-server/v8/mods/model"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
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

func TestKeyGen(t *testing.T) {
	ec := NewEllipticCurveP256()
	pri, pub, err := ec.GenerateKeys()
	require.Nil(t, err)
	require.NotNil(t, pri)
	require.NotNil(t, pub)

	pri_pem, err := ec.EncodePrivate(pri)
	require.Nil(t, err)
	require.NotEmpty(t, pri_pem)

	pub_pem, err := ec.EncodePublic(pub)
	require.Nil(t, err)
	require.NotEmpty(t, pub_pem)
}

func TestNewEllipticCurveP256(t *testing.T) {
	ec := NewEllipticCurveP256()
	pri, _, err := ec.GenerateKeys()
	require.Nil(t, err)
	require.NotNil(t, pri)
	require.Equal(t, 256, pri.Curve.Params().BitSize)
}

func TestNewEllipticCurveP521(t *testing.T) {
	ec := NewEllipticCurveP521()
	pri, _, err := ec.GenerateKeys()
	require.Nil(t, err)
	require.NotNil(t, pri)
	require.Equal(t, 521, pri.Curve.Params().BitSize)
}

func TestKeyEncodeDecode(t *testing.T) {
	ec := NewEllipticCurveP256()
	pri, pub, err := ec.GenerateKeys()
	require.Nil(t, err)

	// Encode and decode private key
	priPem, err := ec.EncodePrivate(pri)
	require.Nil(t, err)
	decodedPri, err := ec.DecodePrivate(priPem)
	require.Nil(t, err)
	require.True(t, pri.Equal(decodedPri))

	// Encode and decode public key
	pubPem, err := ec.EncodePublic(pub)
	require.Nil(t, err)
	decodedPub, err := ec.DecodePublic(pubPem)
	require.Nil(t, err)
	require.True(t, pub.Equal(decodedPub))
}

func TestKeyVerifySignature(t *testing.T) {
	ec := NewEllipticCurveP256()
	pri, pub, err := ec.GenerateKeys()
	require.Nil(t, err)

	sig, verified, err := ec.VerifySignature(pri, pub)
	require.Nil(t, err)
	require.True(t, verified)
	require.NotEmpty(t, sig)
}

func TestKeyTest(t *testing.T) {
	ec := NewEllipticCurveP256()
	pri, pub, err := ec.GenerateKeys()
	require.Nil(t, err)

	err = ec.Test(pri, pub)
	require.Nil(t, err)
}

func TestHashCertificate(t *testing.T) {
	ec := NewEllipticCurveP256()
	pri, pub, err := ec.GenerateKeys()
	require.Nil(t, err)

	certPEM, err := GenerateServerCertificate(pri, pub)
	require.Nil(t, err)

	block, _ := pem.Decode(certPEM)
	require.NotNil(t, block)
	cert, err := x509.ParseCertificate(block.Bytes)
	require.Nil(t, err)

	hash, err := HashCertificate(cert)
	require.Nil(t, err)
	require.NotEmpty(t, hash)
	// SHA3-256 produces 64 hex chars
	require.Len(t, hash, 64)
}

func TestCert(t *testing.T) {
	ec := NewEllipticCurveP256()

	// server key
	svrPri, svrPub, err := ec.GenerateKeys()
	require.Nil(t, err)
	require.NotNil(t, svrPri)
	require.NotNil(t, svrPub)

	svrKeyPEMStr, err := ec.EncodePrivate(svrPri)
	svrKeyPEM := []byte(svrKeyPEMStr)
	require.Nil(t, err)
	require.True(t, len(svrKeyPEM) > 0)

	// server cert
	svrCertPEM, err := GenerateServerCertificate(svrPri, svrPub)
	require.Nil(t, err)
	require.NotNil(t, svrCertPEM)

	// re-parse server cert
	block, _ := pem.Decode([]byte(svrCertPEM))
	svrCert, err := x509.ParseCertificate(block.Bytes)
	require.Nil(t, err)
	require.NotNil(t, svrCert)

	// client key
	ec = NewEllipticCurveP256()
	cliPri, cliPub, err := ec.GenerateKeys()
	require.Nil(t, err)
	require.NotNil(t, cliPri)
	require.NotNil(t, cliPub)

	cliKeyPEMStr, err := ec.EncodePrivate(cliPri)
	cliKeyPEM := []byte(cliKeyPEMStr)
	require.Nil(t, err)
	require.True(t, len(cliKeyPEM) > 0)

	// client cert
	cliCertPEM, err := GenerateClientCertificate(pkix.Name{CommonName: "TheClient"}, []string{"TheClient"}, nil, time.Now(), time.Now().Add(time.Hour), svrCert, svrPri, cliPub)
	require.Nil(t, err)
	require.NotNil(t, cliCertPEM)

	// configure server TLS
	var rootCAs *x509.CertPool = x509.NewCertPool()
	rootCAs.AppendCertsFromPEM(svrCertPEM)

	svrKeyPair, err := tls.X509KeyPair(svrCertPEM, svrKeyPEM)
	if err != nil {
		t.Error(err)
		return
	}
	svrTlsConf := &tls.Config{
		Certificates: []tls.Certificate{svrKeyPair},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    rootCAs,
	}

	// test SSL
	listener := newLocalListener(t, svrTlsConf)
	defer listener.Close()

	complete := make(chan bool)
	defer close(complete)

	go func() {
		t.Log("server listening...", listener.Addr().String())
		incoming, err := listener.Accept()
		if err != nil {
			t.Log(err.Error())
			t.Error(err)
			return
		}
		t.Log("server accepted")
		conn, ok := incoming.(*tls.Conn)
		require.True(t, ok)

		err = conn.HandshakeContext(t.Context())
		require.Nil(t, err)
		t.Log("server-client handshake done")

		<-complete
		conn.Close()
	}()

	// configure client TLS
	cliKeyPair, err := tls.X509KeyPair(cliCertPEM, cliKeyPEM)
	if err != nil {
		panic(err)
	}
	cliTlsConf := &tls.Config{
		Certificates:       []tls.Certificate{cliKeyPair},
		RootCAs:            rootCAs,
		InsecureSkipVerify: true,
	}
	listener_addr := listener.Addr().(*net.TCPAddr)
	hostPort := fmt.Sprintf("%s:%d", listener_addr.IP.String(), listener_addr.Port)
	t.Log("client dialing...", listener_addr.Network(), hostPort)
	conn, err := tls.Dial(listener_addr.Network(), hostPort, cliTlsConf)
	require.Nil(t, err)
	require.NotNil(t, conn)
	conn.Close()
	t.Log("client done")
	complete <- true
}

func newLocalListener(t *testing.T, tlsConf *tls.Config) net.Listener {
	ln, err := tls.Listen("tcp", "127.0.0.1:0", tlsConf)
	if err != nil {
		ln, err = tls.Listen("tcp6", "[::1]:0", tlsConf)
	}
	if err != nil {
		t.Fatal(err)
	}
	return ln
}

type coverageStubDB struct{}

func (d *coverageStubDB) Connect(ctx context.Context) (*client.Appender, error) {
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

func TestServerKeyFormatCases(t *testing.T) {
	tmpDir := t.TempDir()
	svr := &Server{certDirPath: tmpDir}

	ec := NewEllipticCurveP256()
	pri, pub, err := ec.GenerateKeys()
	require.NoError(t, err)

	certPEM, err := GenerateServerCertificate(pri, pub)
	require.NoError(t, err)

	certPath := filepath.Join(tmpDir, "machbase_cert.pem")
	err = os.WriteFile(certPath, certPEM, 0o644)
	require.NoError(t, err)

	block, _ := pem.Decode(certPEM)
	require.NotNil(t, block)
	expectedDERBase64 := base64.StdEncoding.EncodeToString(block.Bytes)

	t.Run("pem", func(t *testing.T) {
		rsp, err := svr.ServerKey(context.Background(), &ServerKeyRequest{Format: "pem"})
		require.NoError(t, err)
		require.NotNil(t, rsp)
		require.True(t, rsp.Success)
		require.Equal(t, "success", rsp.Reason)
		require.Equal(t, "PEM", rsp.Format)
		require.Equal(t, string(certPEM), rsp.Certificate)
	})

	t.Run("der", func(t *testing.T) {
		rsp, err := svr.ServerKey(context.Background(), &ServerKeyRequest{Format: "der"})
		require.NoError(t, err)
		require.NotNil(t, rsp)
		require.True(t, rsp.Success)
		require.Equal(t, "success", rsp.Reason)
		require.Equal(t, "DER", rsp.Format)
		require.Equal(t, expectedDERBase64, rsp.Certificate)
	})

	t.Run("wrong format", func(t *testing.T) {
		rsp, err := svr.ServerKey(context.Background(), &ServerKeyRequest{Format: "xxx"})
		require.NoError(t, err)
		require.NotNil(t, rsp)
		require.False(t, rsp.Success)
		require.Equal(t, "XXX", rsp.Format)
		require.Contains(t, rsp.Reason, "unsupported format")
		require.Empty(t, rsp.Certificate)
	})
}
