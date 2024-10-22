package mqtt2

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/eclipse/paho.golang/autopaho"
	"github.com/eclipse/paho.golang/paho"

	"github.com/machbase/neo-server/api"
	"github.com/machbase/neo-server/mods/logging"
	"github.com/machbase/neo-server/mods/service/msg"
	"github.com/machbase/neo-server/mods/tql"
	"github.com/machbase/neo-server/mods/util/ssfs"
	"github.com/stretchr/testify/require"
)

//go:generate moq -out ./mqtt_mock_test.go -pkg mqtt2 ../../../api Conn Rows Row Result Appender

type dbMock struct {
	api.Database
	conn *ConnMock
}

func (fda *dbMock) Connect(ctx context.Context, options ...api.ConnectOption) (api.Conn, error) {
	if fda.conn == nil {
		return &ConnMock{
			CloseFunc: func() error { return nil },
		}, nil
	}
	return fda.conn, nil
}

var brokerAddr = ""
var testTimeTick = time.Unix(1705291859, 0)
var database = &dbMock{}
var databaseLock sync.Mutex
var mqttServer *mqtt2

func TestMain(m *testing.M) {
	logging.Configure(&logging.Config{
		Console:                     true,
		Filename:                    "-",
		Append:                      false,
		DefaultPrefixWidth:          10,
		DefaultEnableSourceLocation: false,
		DefaultLevel:                "TRACE",
	})

	fileDirs := []string{"/=./test"}
	serverFs, _ := ssfs.NewServerSideFileSystem(fileDirs)
	ssfs.SetDefault(serverFs)

	tqlLoader := tql.NewLoader()

	opts := []Option{
		WithTcpListener("127.0.0.1:0", nil),
		WithTqlLoader(tqlLoader),
	}
	if svr, err := New(database, opts...); err != nil {
		panic(err)
	} else {
		mqttServer = svr.(*mqtt2)
	}

	if err := mqttServer.Start(); err != nil {
		panic(err)
	}

	if addr, ok := mqttServer.broker.Listeners.Get("mqtt2-tcp-0"); !ok {
		panic("Listener not found")
	} else {
		brokerAddr = strings.TrimPrefix(addr.Address(), "tcp://")
	}
	m.Run()

	mqttServer.Stop()
}

type TestCase struct {
	Ver      uint
	Name     string
	ConnMock *ConnMock

	Topic      string
	Payload    []byte
	Properties map[string]string

	Subscribe string
	Expect    any
}

func runTest(t *testing.T, tc *TestCase) {
	t.Helper()

	if tc.ConnMock != nil {
		databaseLock.Lock()
		mqttServer.db = &dbMock{conn: tc.ConnMock}
		defer databaseLock.Unlock()
	}

	brokerUrl, err := url.Parse("tcp://" + brokerAddr)
	require.NoError(t, err)

	wg := sync.WaitGroup{}

	var recvPayload []byte
	// var timeout = 2 * time.Second

	cliCfg := autopaho.ClientConfig{
		ServerUrls:                    []*url.URL{brokerUrl},
		KeepAlive:                     20,
		CleanStartOnInitialConnection: true,
		SessionExpiryInterval:         3,
		ConnectRetryDelay:             1 * time.Second,
		OnConnectionUp: func(cm *autopaho.ConnectionManager, connAck *paho.Connack) {
			if tc.Subscribe != "" {
				// Subscribing in the OnConnectionUp callback is recommended (ensures the subscription is reestablished if
				// the connection drops)
				if _, err := cm.Subscribe(context.Background(), &paho.Subscribe{
					Subscriptions: []paho.SubscribeOptions{
						{Topic: tc.Subscribe, QoS: 1},
					},
				}); err != nil {
					fmt.Printf("failed to subscribe (%s). This is likely to mean no messages will be received.", err)
					t.Fail()
				}
			}
			wg.Done()
		},
		OnConnectError: func(err error) { fmt.Printf("error whilst attempting connection: %s\n", err) },
		// eclipse/paho.golang/paho provides base mqtt functionality, the below config will be passed in for each connection
		ClientConfig: paho.ClientConfig{
			ClientID: "mqtt-test-cli",
			// OnPublishReceived is a slice of functions that will be called when a message is received.
			// You can write the function(s) yourself or use the supplied Router
			OnPublishReceived: []func(paho.PublishReceived) (bool, error){
				func(pr paho.PublishReceived) (bool, error) {
					recvPayload = pr.Packet.Payload
					wg.Done()
					// fmt.Printf("received message on topic %s; body: %s (retain: %t)\n", pr.Packet.Topic, pr.Packet.Payload, pr.Packet.Retain)
					return true, nil
				}},
			OnClientError: func(err error) { fmt.Printf("client error: %s\n", err) },
			OnServerDisconnect: func(d *paho.Disconnect) {
				if d.Properties != nil {
					fmt.Printf("server requested disconnect: %s\n", d.Properties.ReasonString)
				} else {
					fmt.Printf("server requested disconnect; reason code: %d\n", d.ReasonCode)
				}
			},
		},
	}

	ctx := context.Background()

	wg.Add(1)
	c, err := autopaho.NewConnection(ctx, cliCfg)
	if err != nil {
		t.Logf("Test %q failed, connect error: %s", tc.Name, err.Error())
		t.Fail()
	}
	defer c.Disconnect(ctx)

	wg.Wait() // wait connect

	pub := &paho.Publish{
		Topic:   tc.Topic,
		QoS:     2,
		Payload: tc.Payload,
	}
	if tc.Properties != nil {
		pub.Properties = &paho.PublishProperties{}
		for k, v := range tc.Properties {
			pub.Properties.User.Add(k, v)
		}
	}

	if tc.Subscribe != "" {
		wg.Add(1)
	}

	pubAck, err := c.Publish(ctx, pub)
	if err != nil {
		t.Logf("Test %q failed, publish error: %s", tc.Name, err.Error())
		t.Fail()
	}
	if pubAck.ReasonCode != 0 {
		t.Logf("Test %q failed, publish failed: %d", tc.Name, pubAck.ReasonCode)
		t.Fail()
	}

	if tc.Subscribe != "" {
		wg.Wait() // wait message
	}
	if tc.Expect == nil {
		return
	}

	switch expect := tc.Expect.(type) {
	case *msg.QueryResponse:
		actual := msg.QueryResponse{}
		if err := json.Unmarshal(recvPayload, &actual); err != nil {
			t.Logf("Test %q response malformed; %s", tc.Name, err.Error())
			t.Fail()
		}
		require.Equal(t, expect.Success, actual.Success)
		require.Equal(t, expect.Reason, actual.Reason)
		expectJson, _ := json.Marshal(expect.Data)
		actualJson, _ := json.Marshal(actual.Data)
		require.JSONEq(t, string(expectJson), string(actualJson), string(recvPayload))

	case string:
		actual := string(recvPayload)
		if strings.HasPrefix(expect, "/r/") {
			reg := regexp.MustCompile("^" + strings.TrimPrefix(expect, "/r/"))
			if !reg.MatchString(actual) {
				t.Logf("Test  : %s", tc.Name)
				t.Logf("Expect: %s", expect)
				t.Logf("Actual: %s", actual)
				t.Fail()
			}
		} else {
			require.Equal(t, expect, actual)
		}
	case []byte:
		actual := recvPayload
		require.Equal(t, hex.Dump(expect), hex.Dump(actual))
	}
}
