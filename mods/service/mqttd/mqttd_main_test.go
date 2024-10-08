package mqttd

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	paho "github.com/eclipse/paho.mqtt.golang"
	"github.com/machbase/neo-server/api"
	"github.com/machbase/neo-server/mods/service/msg"
	"github.com/machbase/neo-server/mods/tql"
	"github.com/machbase/neo-server/mods/util/ssfs"
	"github.com/stretchr/testify/require"
)

//go:generate moq -out ./mqttd_mock_test.go -pkg mqttd ../../../api Conn Rows Row Result Appender

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
var mqttServer *mqttd

func TestMain(m *testing.M) {
	fileDirs := []string{"/=./test"}
	serverFs, _ := ssfs.NewServerSideFileSystem(fileDirs)
	ssfs.SetDefault(serverFs)

	tqlLoader := tql.NewLoader()

	opts := []Option{
		OptionListenAddress("tcp://127.0.0.1:0"),
		OptionTqlLoader(tqlLoader),
	}
	if svr, err := New(database, opts...); err != nil {
		panic(err)
	} else {
		mqttServer = svr.(*mqttd)
	}

	if err := mqttServer.Start(); err != nil {
		panic(err)
	}

	brokerAddr = mqttServer.mqttd.Listeners()[0].Address()
	m.Run()

	mqttServer.Stop()
}

type TestCase struct {
	Name     string
	ConnMock *ConnMock

	Topic   string
	Payload []byte

	Subscribe string
	Expect    any
}

func runTest(t *testing.T, tc *TestCase) {
	t.Helper()

	databaseLock.Lock()
	mqttServer.db = &dbMock{conn: tc.ConnMock}
	defer databaseLock.Unlock()

	cfg := paho.NewClientOptions()
	cfg.SetCleanSession(true)
	cfg.SetConnectRetry(false)
	cfg.SetAutoReconnect(false)
	cfg.SetProtocolVersion(4)
	cfg.SetClientID("machbase-test-cli")
	cfg.AddBroker(brokerAddr)
	cfg.SetKeepAlive(3 * time.Second)

	//// connect mqtt server
	cli := paho.NewClient(cfg)
	require.NotNil(t, cli)

	var Wait sync.WaitGroup
	var recvPayload []byte
	var recvTopic string
	var timeout = 2 * time.Second

	conAck := cli.Connect()
	if !conAck.WaitTimeout(timeout) {
		t.Logf("Test %q failed, connect timed out", tc.Name)
		t.Fail()
	}
	defer cli.Disconnect(0)

	if tc.Subscribe != "" {
		Wait.Add(1)
		subAck := cli.Subscribe(tc.Subscribe, 1, func(c paho.Client, m paho.Message) {
			// received
			recvPayload = m.Payload()
			recvTopic = m.Topic()
			if recvTopic != tc.Subscribe {
				t.Logf("Expect recv topic %q, got %q", tc.Subscribe, recvTopic)
				t.Fail()
			}
			Wait.Done()
		})
		if !subAck.WaitTimeout(timeout) {
			t.Logf("Test %q failed, subscribe timed out", tc.Name)
			t.Fail()
		}
	}

	Wait.Add(1)
	pubAck := cli.Publish(tc.Topic, 1, false, tc.Payload)
	if pubAck.WaitTimeout(timeout) {
		Wait.Done()
	} else {
		t.Logf("Test %q failed, publish timed out", tc.Name)
		t.Fail()
	}

	Wait.Wait()

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
