//go:build ignore

package main

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/gopcua/opcua"
	"github.com/gopcua/opcua/ua"
)

type testOPCUALogger struct{}

func (testOPCUALogger) Debug(msg string, args ...any) { fmt.Printf("opcua-debug: "+msg, args...) }
func (testOPCUALogger) Error(msg string, args ...any) { fmt.Printf("opcua-error: "+msg, args...) }
func (testOPCUALogger) Info(msg string, args ...any)  { fmt.Printf("opcua-info: "+msg, args...) }
func (testOPCUALogger) Warn(msg string, args ...any)  { fmt.Printf("opcua-warn: "+msg, args...) }

// ServerListen             client-endpointURL            Connect
// --------------------------------------------------------------
// localhost:4840           opc.tcp://localhost:4840           OK
// localhost:4840           opc.tcp://[::1]:4840               OK
// localhost:4840           opc.tcp://127.0.0.1:4840           connection refused
// localhost:4840           opc.tcp://192.168.1.172:4840       connection refused
//
// 127.0.0.1:4840           opc.tcp://localhost:4840           connection refused
// 127.0.0.1:4840           opc.tcp://[::1]:4840               connection refused
// 127.0.0.1:4840           opc.tcp://127.0.0.1:4840           OK
// 127.0.0.1:4840           opc.tcp://192.168.1.172:4840       connection refused
//
// 0.0.0.0:4840             opc.tcp://localhost:4840           OK
// 0.0.0.0:4840             opc.tcp://[::1]:4840               OK
// 0.0.0.0:4840             opc.tcp://127.0.0.1:4840           OK
// 0.0.0.0:4840             opc.tcp://192.168.1.172:4840       OK
//
// 192.168.1.172:4840       opc.tcp://localhost:4840           connection refused
// 192.168.1.172:4840       opc.tcp://[::1]:4840               connection refused
// 192.168.1.172:4840       opc.tcp://127.0.0.1:4840           connection refused
// 192.168.1.172:4840       opc.tcp://192.168.1.172:4840       OK

func main() {
	ctx := context.Background()
	opts := []opcua.Option{}
	endpointURL := "opc.tcp://localhost:4840"

	onSecure := false
	if onSecure {
		certDir := os.TempDir()
		certName := fmt.Sprintf("opcua_%d", os.Getpid())
		clientCertFilePath := filepath.Join(certDir, certName+"_client_cert.pem")
		clientKeyFilePath := filepath.Join(certDir, certName+"_client_key.pem")

		clientCertPEM, clientKeyPEM, err := generateCert([]string{"urn:neo:opcua:test-client"}, 2048, time.Hour, nil)
		if err != nil {
			panic(err)
		}
		os.WriteFile(clientCertFilePath, clientCertPEM, 0644)
		os.WriteFile(clientKeyFilePath, clientKeyPEM, 0644)

		clientPair, err := tls.LoadX509KeyPair(clientCertFilePath, clientKeyFilePath)
		if err != nil {
			panic(err)
		}
		clientPrivKey, ok := clientPair.PrivateKey.(*rsa.PrivateKey)
		if !ok {
			panic("client private key is not RSA")
		}
		opts = append(opts,
			//opcua.RemoteCertificateFile(serverCertFilePath),
			opcua.Certificate(clientPair.Certificate[0]),
			opcua.PrivateKey(clientPrivKey),
			//opcua.AuthCertificate(clientPair.Certificate[0]),
			opcua.AuthPrivateKey(clientPrivKey),
		)
	}

	endpoints, err := opcua.GetEndpoints(ctx, endpointURL)
	if err != nil {
		panic(fmt.Sprintf("GetEndpoints failed: %s", err))
	}

	var selected *ua.EndpointDescription
	for _, ep := range endpoints {
		if ep.SecurityMode != ua.MessageSecurityModeSignAndEncrypt {
			continue
		}
		if ep.SecurityPolicyURI != ua.SecurityPolicyURIPrefix+"Basic256" {
			continue
		}
		for _, tok := range ep.UserIdentityTokens {
			if tok.TokenType == ua.UserTokenTypeCertificate {
				selected = ep
				break
			}
		}
		if selected != nil {
			break
		}
		opts = append(opts,
			opcua.SecurityFromEndpoint(selected, ua.UserTokenTypeCertificate),
		)
	}

	client, err := opcua.NewClient(endpointURL, opts...)
	if err != nil {
		panic(fmt.Sprintf("Failed to create client: %s", err))
	}
	if err := client.Connect(ctx); err != nil {
		panic(fmt.Sprintf("Failed to connect client: %s", err))
	}
	defer client.Close(ctx)

	readReq := &ua.ReadRequest{
		TimestampsToReturn: ua.TimestampsToReturnBoth,
		NodesToRead: []*ua.ReadValueID{{
			NodeID:      ua.MustParseNodeID("ns=1;s=sys_mem"),
			AttributeID: ua.AttributeIDValue,
		}},
	}
	readRsp, err := client.Read(ctx, readReq)
	if err != nil {
		panic(fmt.Sprintf("Read failed: %s", err))
	}
	fmt.Println("Read response:", readRsp.Results[0].Status, readRsp.Results[0].Value.Value())
}

func generateCert(host []string, rsaBits int, validFor time.Duration, signer crypto.Signer) (certPEM, keyPEM []byte, err error) {
	if len(host) == 0 {
		return nil, nil, fmt.Errorf("missing required host parameter")
	}
	if rsaBits == 0 {
		rsaBits = 2048
	}

	priv, err := rsa.GenerateKey(rand.Reader, rsaBits)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate private key: %s", err)
	}

	notBefore := time.Now()
	notAfter := notBefore.Add(validFor)

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate serial number: %s", err)
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName: "Gopcua Server",
		},
		NotBefore: notBefore,
		NotAfter:  notAfter,

		KeyUsage: x509.KeyUsageContentCommitment | x509.KeyUsageKeyEncipherment |
			x509.KeyUsageDigitalSignature | x509.KeyUsageDataEncipherment | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
	}

	for _, h := range host {
		if ip := net.ParseIP(h); ip != nil {
			template.IPAddresses = append(template.IPAddresses, ip)
		} else {
			template.DNSNames = append(template.DNSNames, h)
		}
		if uri, err := url.Parse(h); err == nil && uri.Scheme != "" {
			template.URIs = append(template.URIs, uri)
		}
	}
	if signer == nil {
		signer = priv
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, publicKey(priv), signer)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create certificate: %s", err)
	}

	certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	keyPEM = pem.EncodeToMemory(pemBlockForKey(priv))
	return
}

func publicKey(priv interface{}) interface{} {
	switch k := priv.(type) {
	case *rsa.PrivateKey:
		return &k.PublicKey
	case *ecdsa.PrivateKey:
		return &k.PublicKey
	default:
		return nil
	}
}

func pemBlockForKey(priv interface{}) *pem.Block {
	switch k := priv.(type) {
	case *rsa.PrivateKey:
		return &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(k)}
	case *ecdsa.PrivateKey:
		b, err := x509.MarshalECPrivateKey(k)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Unable to marshal ECDSA private key: %v", err)
			os.Exit(2)
		}
		return &pem.Block{Type: "EC PRIVATE KEY", Bytes: b}
	default:
		return nil
	}
}
