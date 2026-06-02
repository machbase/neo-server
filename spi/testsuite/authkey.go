package testsuite

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/machbase/neo-client/api"
	"github.com/machbase/neo-client/machgo"
	"github.com/stretchr/testify/require"
)

func AuthKeyConnect(t *testing.T, db api.Database, ctx context.Context) {
	goDB, ok := db.(*machgo.Database)
	if !ok {
		t.Skip("auth key test is only for neo-client machgo database")
		return
	}

	host, port := machgoEndpoint(t, goDB)

	adminConn, err := goDB.Connect(ctx, api.WithPassword("sys", "manager"))
	require.NoError(t, err)
	defer adminConn.Close()

	type keyCase struct {
		name string
		gen  func(dir string) (privatePath string, publicPEM string, err error)
	}

	cases := []keyCase{
		{name: "ecdsa_p256", gen: generateECDSAP256KeyPair},
		{name: "rsa_2048", gen: generateRSA2048KeyPair},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			keyDir := filepath.Join(t.TempDir(), "authkey")
			require.NoError(t, os.MkdirAll(keyDir, 0o700))

			privatePath, pubPEM, err := tc.gen(keyDir)
			require.NoError(t, err)

			comment := fmt.Sprintf("testsuite auth key %s %d", tc.name, time.Now().UnixNano())
			keyID, err := registerAndActivateUserAuthKey(ctx, adminConn, "SYS", pubPEM, comment)
			require.NoError(t, err)
			require.Greater(t, keyID, 0)

			authDB, err := machgo.NewDatabase(&machgo.Config{
				Host:         host,
				Port:         port,
				MaxOpenConn:  1,
				MaxOpenQuery: 1,
			})
			require.NoError(t, err)
			defer authDB.Close()

			key, err := machgo.LoadPrivateKeyFromFile(privatePath)
			require.NoError(t, err)
			authConn, err := authDB.Connect(ctx, api.WithAuthKey("sys", key))
			require.NoError(t, err)
			defer authConn.Close()

			row := authConn.QueryRow(ctx, "select 1")
			require.NoError(t, row.Err())
			var v int64
			require.NoError(t, row.Scan(&v))
			require.Equal(t, int64(1), v)
		})
	}
}

func registerAndActivateUserAuthKey(ctx context.Context, conn api.Conn, user string, pubPEM string, comment string) (int, error) {
	pubPEM = strings.TrimSpace(pubPEM)

	validBefore := time.Now().Add(24 * time.Hour * 365 * 30).Format("2006-01-02")
	result := conn.Exec(ctx,
		fmt.Sprintf("ALTER USER %s ADD AUTH KEY (KEY = '%s', VALID_BEFORE = '%s', COMMENT = '%s')",
			user, pubPEM, validBefore, comment),
	)
	if result.Err() != nil {
		return 0, result.Err()
	}

	row := conn.QueryRow(ctx, `SELECT KEY_ID, ACTIVATED FROM V$USER_AUTH_KEYS WHERE USER_NAME=? AND COMMENT=? ORDER BY KEY_ID DESC LIMIT 1`, user, comment)
	if row.Err() != nil {
		return 0, row.Err()
	}

	var keyID int
	var activated int
	if err := row.Scan(&keyID, &activated); err != nil {
		return 0, err
	}

	if activated == 0 {
		result := conn.Exec(ctx, fmt.Sprintf("ALTER USER %s ACTIVATE AUTH KEY ID %d", user, keyID))
		if result.Err() != nil {
			return 0, result.Err()
		}
	}

	return keyID, nil
}

func generateECDSAP256KeyPair(dir string) (string, string, error) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return "", "", err
	}
	privDer, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		return "", "", err
	}
	pubDer, err := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	if err != nil {
		return "", "", err
	}
	privPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: privDer})
	pubPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubDer})
	privPath := filepath.Join(dir, "ecdsa_p256_key.pem")
	if err := os.WriteFile(privPath, privPEM, 0o600); err != nil {
		return "", "", err
	}
	return privPath, string(pubPEM), nil
}

func generateRSA2048KeyPair(dir string) (string, string, error) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return "", "", err
	}
	pubDer, err := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	if err != nil {
		return "", "", err
	}
	privPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)})
	pubPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubDer})
	privPath := filepath.Join(dir, "rsa_2048_key.pem")
	if err := os.WriteFile(privPath, privPEM, 0o600); err != nil {
		return "", "", err
	}
	return privPath, string(pubPEM), nil
}
