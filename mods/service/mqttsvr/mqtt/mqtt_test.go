package mqtt_test

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	paho "github.com/eclipse/paho.mqtt.golang"
	"github.com/machbase/neo-server/mods/logging"
	"github.com/machbase/neo-server/mods/service/mqttsvr/mqtt"
	"github.com/machbase/neo-server/mods/service/security"
	"github.com/machbase/neo-server/mods/util/glob"
	"github.com/stretchr/testify/assert"
)

var (
	_, b, _, _ = runtime.Caller(0)
	basepath   = filepath.Dir(b)

	//logFile          = filepath.Join(basepath, "../tmp/mqtt01.log")
	logFile          = "-"
	admSockFile      = filepath.Join(basepath, "../tmp/mqtt.sock")
	handshakeTimeout = 3
	serverCertFile   = filepath.Join(basepath, "../test/test_server_cert.pem")
	serverKeyFile    = filepath.Join(basepath, "../test/test_server_key.pem")
	clientCertFile   = filepath.Join(basepath, "../test/test_client_cert.pem")
	clientKeyFile    = filepath.Join(basepath, "../test/test_client_key.pem")
)

func TestMain(m *testing.M) {

	useTls := true

	var tlsConfig mqtt.TlsListenerConfig
	if useTls {
		tlsConfig = mqtt.TlsListenerConfig{
			LoadSystemCAs:    false,
			LoadPrivateCAs:   true,
			CertFile:         serverCertFile,
			KeyFile:          serverKeyFile,
			HandshakeTimeout: time.Duration(handshakeTimeout) * time.Second,
		}
	}
	svrConfig := &mqtt.MqttConfig{
		Name: "test-mqsvr",
		TcpListeners: []mqtt.TcpListenerConfig{
			{
				ListenAddress: "127.0.0.1:1884",
				SoLinger:      0,
				KeepAlive:     10,
				NoDelay:       true,
				Tls:           tlsConfig,
			},
		},
	}

	// logger
	logConfig := &logging.Config{
		Console:            true,
		Filename:           logFile,
		Append:             false,
		MaxSize:            10,
		MaxBackups:         1,
		MaxAge:             1,
		Compress:           false,
		DefaultPrefixWidth: 30,
		DefaultLevel:       "TRACE",
	}
	// Set Logging
	logging.Configure(logConfig)

	delegate := &tdelegate{}

	// Start Server
	svr := mqtt.NewServer(svrConfig, delegate)
	err := svr.Start()
	if err != nil {
		fmt.Printf("Server start failed: %+v\n", tlsConfig)
		panic(err)
	}

	code := m.Run()

	svr.Stop()

	os.Exit(code)
}

type tdelegate struct {
	mqtt.ServerDelegate
}

func (td *tdelegate) OnConnect(evt *mqtt.EvtConnect) (mqtt.AuthCode, *mqtt.ConnectResult, error) {
	result := &mqtt.ConnectResult{
		AllowedPublishTopicPatterns:   []string{"m/*"},
		AllowedSubscribeTopicPatterns: []string{"m/rpc"},
	}

	return mqtt.AuthSuccess, result, nil
}

func (td *tdelegate) OnDisconnect(evt *mqtt.EvtDisconnect) {

}

func (td *tdelegate) OnMessage(evt *mqtt.EvtMessage) error {
	return nil
}

var deviceIdSeq = 1

func getOptions(broker string, keepAlive time.Duration) *paho.ClientOptions {
	cfg := paho.NewClientOptions()
	cfg.SetKeepAlive(keepAlive)
	cfg.SetCleanSession(true)
	cfg.SetConnectRetry(false)
	cfg.SetAutoReconnect(false)
	cfg.SetProtocolVersion(4)

	if strings.HasPrefix(broker, "mqtt:") || strings.HasPrefix(broker, "unix:") {
		deviceId := fmt.Sprintf("SIM1_1234567890%05d", deviceIdSeq)
		deviceIdSeq++

		otpGen, _ := security.NewGenerator([]byte(deviceId), 60, []int{-1, 1}, security.GeneratorHex12)
		otp, _ := otpGen.Generate()
		cfg.SetClientID(otp)

		cfg.AddBroker(broker)
		cfg.SetUsername(deviceId)
	} else if strings.HasPrefix(broker, "mqtts:") || strings.HasPrefix(broker, "tls:") || strings.HasPrefix(broker, "ssl:") {
		cfg.AddBroker(broker)

		deviceId := "SIM1_1234567890ABCDEF"
		otpGen, _ := security.NewGenerator([]byte(deviceId), 60, []int{-1, 1}, security.GeneratorHex12)
		otp, _ := otpGen.Generate()
		cfg.SetClientID(otp)

		////////////////////
		// append root ca
		rootCAs := x509.NewCertPool()
		ca, err := ioutil.ReadFile(serverCertFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "fail to load keys: %s\n", err)
			return nil
		}
		rootCAs.AppendCertsFromPEM(ca)

		cert, err := tls.LoadX509KeyPair(clientCertFile, clientKeyFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "fail to load keys: %s\n", err)
			return nil
		}

		tlsCfg := &tls.Config{
			InsecureSkipVerify: true,
			RootCAs:            rootCAs,
			ClientAuth:         tls.NoClientCert,
			ClientCAs:          nil,
			Certificates:       []tls.Certificate{cert},
		}
		cfg.SetTLSConfig(tlsCfg)
	}
	return cfg
}

func TestTlsSocket(t *testing.T) {
	ops := getOptions("mqtts://127.0.0.1:1884", 3*time.Second)
	assert.NotNil(t, ops)

	conn, err := tls.Dial("tcp", "127.0.0.1:1884", ops.TLSConfig)
	if err != nil {
		t.Logf("Socket error: %s", err)
	}

	//t.Logf("Socket: %+v", conn.ConnectionState())
	t.Logf("Remote: %+v", conn.RemoteAddr())
	t.Logf("Local : %+v", conn.LocalAddr())

	// wait handshake
	time.Sleep(time.Second * 1)
}

func TestTlsHandshakeTimeout(t *testing.T) {
	// tls 포트로 plain socket을 연결하고 ssl handshake timeout을 기다리는 시험.
	conn, err := net.Dial("tcp", "127.0.0.1:1884")
	if err != nil {
		t.Logf("Socket error: %s", err)
	}

	t.Logf("Remote: %+v", conn.RemoteAddr())
	t.Logf("Local : %+v", conn.LocalAddr())

	one := make([]byte, 1)
	conn.SetReadDeadline(time.Now().Add(time.Second * time.Duration(handshakeTimeout+1)))
	n, err := conn.Read(one)
	assert.True(t, err == io.EOF, "socket should be closed by remote peer, because of handshake timeout")
	assert.True(t, n == 0, "read bytes should be zero")
}

func TestConnect(t *testing.T) {
	brokers := []string{
		//"mqtt://127.0.0.1:1883",
		"mqtts://127.0.0.1:1884",
		// "unix:///tmp/cmqd.sock",
	}
	for _, broker := range brokers {
		ops := getOptions(broker, 3*time.Second)
		assert.NotNil(t, ops)

		client := paho.NewClient(ops)
		assert.NotNil(t, client)

		token := client.Connect()
		ok := token.WaitTimeout(time.Second)
		assert.True(t, ok)
		connack := token.(*paho.ConnectToken)
		assert.Nil(t, connack.Error())

		// ping test
		time.Sleep(time.Second * 4)

		client.Disconnect(100)
		time.Sleep(time.Second * 1)
	}
}

func TestPublish(t *testing.T) {
	brokers := []string{
		// "mqtt://127.0.0.1:1883",
		"mqtts://127.0.0.1:1884",
		//"unix://tmp/cmqd.sock",
	}
	for _, broker := range brokers {
		ops := getOptions(broker, 30*time.Second)
		client := paho.NewClient(ops)
		assert.NotNil(t, client)

		result := client.Connect()
		ok := result.WaitTimeout(time.Second)
		if result.Error() != nil {
			t.Logf("CONNECT: %s", result.Error())
		}
		assert.True(t, ok)
		assert.Nil(t, result.Error())

		result = client.Publish("m/log", 1, false, []byte(`{"message":"hello"}`))
		ok = result.WaitTimeout(time.Second)
		assert.True(t, ok)

		//wg := sync.WaitGroup()

		result = client.Subscribe("s/test", 1, func(cli paho.Client, msg paho.Message) {
			t.Logf("--------> Client Recv: %v", hex.Dump(msg.Payload()))
		})
		ok = result.WaitTimeout(time.Second)
		assert.True(t, ok)

		result = client.Unsubscribe("s/test")
		ok = result.WaitTimeout(time.Second)
		assert.True(t, ok)

		client.Disconnect(100)
		time.Sleep(time.Second * 1)
	}
}

func Benchmark_publish(b *testing.B) {
	broker := "mqtts://127.0.0.1:1884"
	var qos byte = 1

	ops := getOptions(broker, 30*time.Second)
	client := paho.NewClient(ops)
	assert.NotNil(b, client)

	token := client.Connect()
	ok := token.WaitTimeout(time.Second)
	if token.Error() != nil {
		b.Logf("CONNECT: %s", token.Error())
	}
	assert.True(b, ok)

	for i := 0; i < b.N; i++ {
		token = client.Publish("m/log", qos, false, []byte(fmt.Sprintf(`{"message":"hello-%d"}`, i)))
		ok = token.WaitTimeout(time.Second)
		assert.True(b, ok)

	}

	client.Disconnect(100)
}

func Benchmark_multi_client(b *testing.B) {
	broker := "mqtt://127.0.0.1:1884"
	var qos byte = 1
	var num_clients = 64
	var clients = make([]paho.Client, num_clients)

	for i := 0; i < num_clients; i++ {
		ops := getOptions(broker, 30*time.Second)
		client := paho.NewClient(ops)
		assert.NotNil(b, client)
		clients[i] = client

		token := client.Connect()
		ok := token.WaitTimeout(time.Second)
		if token.Error() != nil {
			b.Logf("CONNECT: %s", token.Error())
		}
		assert.True(b, ok)
	}

	for i := 0; i < b.N; i++ {
		client := clients[i%num_clients]

		token := client.Publish("m/log", qos, false, []byte(fmt.Sprintf(`{"message":"hello-%d"}`, i)))
		ok := token.WaitTimeout(time.Second)
		assert.True(b, ok)
	}

	for i := 0; i < num_clients; i++ {
		clients[i].Disconnect(100)
	}
}

func TestPeerLogLevel(t *testing.T) {
	patterns := []string{
		"*",
		"UNK?_*",
		"UNK1_*789",
		"LUX1_*",
		"UNK?_BBC*789",
	}

	// key: device_id
	// val: expected pattern
	targets := map[string]string{
		"UNK1_1234567890EFG": "UNK?_*",
		"LUX1_ABCEEFGABCDEF": "LUX1_*",
		"UNK1_ABCDEFGHIJ789": "UNK1_*789",
		"UNK2_BBCDEFGHIJ789": "UNK?_BBC*789",
		"TLK1_ABCDEFGHIJKLM": "*",
	}

	var matched string = ""

	for deviceId, expected := range targets {
		for _, pattern := range patterns {
			if ismatch, err := glob.Match(pattern, deviceId); ismatch && err == nil {
				if matched == "" {
					matched = pattern
				} else {
					if len(matched) < len(pattern) {
						matched = pattern
					}
				}
			}
		}
		assert.Equal(t, expected, matched)
		matched = ""
	}
}

func TestOtpPrefixes(t *testing.T) {
	prefixes := mqtt.NewOtpPrefixes()
	prefixes.Set("UNK1", "12345678")
	prefixes.Set("UNK2", "23456789")

	k, ok := prefixes.Match("UNK1_1234567890ABCDE")
	assert.True(t, ok)
	assert.Equal(t, "12345678UNK1_1234567890ABCDE", k)
}
