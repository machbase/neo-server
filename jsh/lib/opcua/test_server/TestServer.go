//go:build ignore

package main

import (
	"context"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"log"
	"math/big"
	"net"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/gopcua/opcua/id"
	"github.com/gopcua/opcua/server"
	opc_server "github.com/gopcua/opcua/server"
	"github.com/gopcua/opcua/ua"
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/load"
	"github.com/shirou/gopsutil/v4/mem"
)

// Run:
// go run ./jsh/lib/opcua/test_server
func main() {
	svr := startOPCUAServer()
	defer svr.Close()

	// signal handler
	quitChan := make(chan os.Signal, 1)
	signal.Notify(quitChan, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	// wait signal
	<-quitChan
}

func startOPCUAServer() *opc_server.Server {
	var opts []opc_server.Option
	port := 4840

	opts = append(opts,
		opc_server.EnableSecurity("None", ua.MessageSecurityModeNone),
		opc_server.EnableSecurity("Basic128Rsa15", ua.MessageSecurityModeSign),
		opc_server.EnableSecurity("Basic128Rsa15", ua.MessageSecurityModeSignAndEncrypt),
		opc_server.EnableSecurity("Basic256", ua.MessageSecurityModeSign),
		opc_server.EnableSecurity("Basic256", ua.MessageSecurityModeSignAndEncrypt),
		opc_server.EnableSecurity("Basic256Sha256", ua.MessageSecurityModeSignAndEncrypt),
		opc_server.EnableSecurity("Basic256Sha256", ua.MessageSecurityModeSign),
		opc_server.EnableSecurity("Aes128_Sha256_RsaOaep", ua.MessageSecurityModeSign),
		opc_server.EnableSecurity("Aes128_Sha256_RsaOaep", ua.MessageSecurityModeSignAndEncrypt),
		opc_server.EnableSecurity("Aes256_Sha256_RsaPss", ua.MessageSecurityModeSign),
		opc_server.EnableSecurity("Aes256_Sha256_RsaPss", ua.MessageSecurityModeSignAndEncrypt),
	)

	opts = append(opts,
		opc_server.EnableAuthMode(ua.UserTokenTypeAnonymous),
		opc_server.EnableAuthMode(ua.UserTokenTypeUserName),
		opc_server.EnableAuthMode(ua.UserTokenTypeCertificate),
		//		server.EnableAuthWithoutEncryption(), // Dangerous and not recommended, shown for illustration only
	)

	opts = append(opts,
		opc_server.EndPoint("localhost", port),
	)

	var certDir = os.TempDir()
	var certFilePath = filepath.Join(certDir, "opcserver_cert.pem")
	var keyFilePath = filepath.Join(certDir, "opcserver_key.pem")

	// Generate self-signed certificate for testing if not exist
	if _, err := os.Stat(certFilePath); os.IsNotExist(err) {
		c, k, err := GenerateCert([]string{"localhost"}, 4096, time.Minute*60*24*365*10)
		if err != nil {
			log.Fatalf("Error generating certificate: %s", err)
		}
		err = os.WriteFile(certFilePath, c, 0600)
		if err != nil {
			log.Fatalf("Error writing certificate file: %s", err)
		}
		err = os.WriteFile(keyFilePath, k, 0600)
		if err != nil {
			log.Fatalf("Error writing key file: %s", err)
		}
	}

	// Load certificate and key files and add to server options
	func() {
		c, err := tls.LoadX509KeyPair(certFilePath, keyFilePath)
		if err != nil {
			log.Fatalf("Error loading certificate and key files: %s", err)
		}
		pk, ok := c.PrivateKey.(*rsa.PrivateKey)
		if !ok {
			log.Fatalf("Private key is not RSA")
		}
		cert := c.Certificate[0]
		opts = append(opts, opc_server.PrivateKey(pk), server.Certificate(cert))
	}()

	s := opc_server.New(opts...)

	root_ns, _ := s.Namespace(0)
	obj_node := root_ns.Objects()

	// Create a new node namespace.  You can add namespaces before or after starting the server.
	nodeNS := opc_server.NewNodeNameSpace(s, "NodeNamespace")
	// add it to the server.
	s.AddNamespace(nodeNS)
	nns_obj := nodeNS.Objects()
	// add the reference for this namespace's root object folder to the server's root object folder
	obj_node.AddRef(nns_obj, id.HasComponent, true)

	// Create cpu usage node
	n := nodeNS.AddNewVariableStringNode("sys_cpu", cpuPercent)
	n.SetAttribute(ua.AttributeIDUserAccessLevel, &ua.DataValue{EncodingMask: ua.DataValueValue, Value: ua.MustVariant(byte(1))})
	nns_obj.AddRef(n, id.HasComponent, true)

	// Create memory usage node
	n = nodeNS.AddNewVariableStringNode("sys_mem", memPercent)
	n.SetAttribute(ua.AttributeIDUserAccessLevel, &ua.DataValue{EncodingMask: ua.DataValueValue, Value: ua.MustVariant(byte(1))})
	nns_obj.AddRef(n, id.HasComponent, true)

	// Create load average node
	n = nodeNS.AddNewVariableStringNode("load1", loadAvg(1))
	n.SetAttribute(ua.AttributeIDUserAccessLevel, &ua.DataValue{EncodingMask: ua.DataValueValue, Value: ua.MustVariant(byte(1))})
	nns_obj.AddRef(n, id.HasComponent, true)

	n = nodeNS.AddNewVariableStringNode("load5", loadAvg(5))
	n.SetAttribute(ua.AttributeIDUserAccessLevel, &ua.DataValue{EncodingMask: ua.DataValueValue, Value: ua.MustVariant(byte(1))})
	nns_obj.AddRef(n, id.HasComponent, true)

	n = nodeNS.AddNewVariableStringNode("load15", loadAvg(15))
	n.SetAttribute(ua.AttributeIDUserAccessLevel, &ua.DataValue{EncodingMask: ua.DataValueValue, Value: ua.MustVariant(byte(1))})
	nns_obj.AddRef(n, id.HasComponent, true)

	// Create a new node namespace.  You can add namespaces before or after starting the server.
	gopcuaNS := opc_server.NewNodeNameSpace(s, "http://gopcua.com/")
	// add it to the server.
	s.AddNamespace(gopcuaNS)
	nns_obj = gopcuaNS.Objects()
	// add the reference for this namespace's root object folder to the server's root object folder
	obj_node.AddRef(nns_obj, id.HasComponent, true)

	// Create a new node namespace.  You can add namespaces before or after starting the server.
	// Start the server
	if err := s.Start(context.Background()); err != nil {
		log.Fatalf("Error starting server, exiting: %s", err)
	}
	return s
}

func cpuPercent() *ua.DataValue {
	var interval time.Duration
	perCPU := false
	ctx := context.Background()
	cpuPercents, err := cpu.PercentWithContext(ctx, interval, perCPU)
	if err != nil {
		panic(err)
	}
	return opc_server.DataValueFromValue(cpuPercents[0])
}

func memPercent() *ua.DataValue {
	ctx := context.Background()
	stat, err := mem.VirtualMemoryWithContext(ctx)
	if err != nil {
		panic(err)
	}
	return opc_server.DataValueFromValue(stat.UsedPercent)
}

func loadAvg(m int) func() *ua.DataValue {
	return func() *ua.DataValue {
		ctx := context.Background()
		loadAvg, err := load.AvgWithContext(ctx)
		if err != nil {
			panic(err)
		}
		ret := 0.0
		switch m {
		default:
			ret = loadAvg.Load1
		case 5:
			ret = loadAvg.Load5
		case 15:
			ret = loadAvg.Load15
		}
		return opc_server.DataValueFromValue(ret)
	}
}

func GenerateCert(host []string, rsaBits int, validFor time.Duration) (certPEM, keyPEM []byte, err error) {
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

		KeyUsage:              x509.KeyUsageContentCommitment | x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageDataEncipherment | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
	}

	for _, h := range host {
		if ip := net.ParseIP(h); ip != nil {
			template.IPAddresses = append(template.IPAddresses, ip)
		} else {
			template.DNSNames = append(template.DNSNames, h)
		}
		if uri, err := url.Parse(h); err == nil {
			template.URIs = append(template.URIs, uri)
		}
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, publicKey(priv), priv)
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
