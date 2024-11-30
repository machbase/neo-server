package server

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/eclipse/paho.golang/autopaho"
	"github.com/eclipse/paho.golang/paho"
	"github.com/machbase/neo-server/v8/api"
	"github.com/stretchr/testify/require"
)

type MqttTestCase struct {
	Ver  uint
	Name string

	Topic      string
	Payload    []byte
	Properties map[string]string

	Subscribe string
	Expect    any
}

func runMqttTest(t *testing.T, tc *MqttTestCase) {
	t.Helper()

	brokerUrl, err := url.Parse("tcp://" + mqttServerAddress)
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
		t.Fatalf("Test %q failed, publish error: %s", tc.Name, err.Error())
	}
	if pubAck.ReasonCode != 0 {
		t.Fatalf("Test %q failed, publish failed: %d", tc.Name, pubAck.ReasonCode)
	}

	if tc.Subscribe != "" {
		wg.Wait() // wait message
	}
	if tc.Expect == nil {
		return
	}

	switch expect := tc.Expect.(type) {
	case *QueryResponse:
		actual := QueryResponse{}
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

func TestMqttQuery(t *testing.T) {
	tests := []MqttTestCase{
		{
			Name:      "db/query simple",
			Topic:     "db/query",
			Payload:   []byte(`{"q": "select * from example where name = 'temp'" }`),
			Subscribe: "db/reply",
			Expect: &QueryResponse{
				Success: true,
				Reason:  "success",
				Data: &QueryData{
					Columns: []string{"NAME", "TIME", "VALUE"},
					Types:   []api.DataType{api.DataTypeString, api.DataTypeDatetime, api.DataTypeFloat64},
					Rows: [][]any{
						{"temp", testTimeTick.UnixNano(), 3.14},
					},
				},
			},
		},
		{
			Name:      "db/query simple timeformat",
			Topic:     "db/query",
			Payload:   []byte(`{"q": "select * from example where name = 'temp'", "format":"json", "tz":"UTC", "timeformat": "DEFAULT" }`),
			Subscribe: "db/reply",
			Expect: &QueryResponse{
				Success: true,
				Reason:  "success",
				Data: &QueryData{
					Columns: []string{"NAME", "TIME", "VALUE"},
					Types:   []api.DataType{api.DataTypeString, api.DataTypeDatetime, api.DataTypeFloat64},
					Rows: [][]any{
						{"temp", "2024-01-15 04:10:59", 3.14},
					},
				},
			},
		},
		{
			Name:      "db/query json timeformat rowsFlatten",
			Topic:     "db/query",
			Payload:   []byte(`{"q": "select * from example where name = 'temp'", "format":"json", "tz":"UTC", "timeformat": "DEFAULT", "rowsFlatten": true }`),
			Subscribe: "db/reply",
			Expect:    `/r/{"data":{"columns":\["NAME","TIME","VALUE"\],"types":\["string","datetime","double"\],"rows":\["temp","2024-01-15 04:10:59",3.14\]},"success":true,"reason":"success","elapse":".*"}`,
		},
		{
			Name:      "db/query json transpose",
			Topic:     "db/query",
			Payload:   []byte(`{"q": "select * from example where name = 'temp'", "format":"json", "transpose": true }`),
			Subscribe: "db/reply",
			Expect:    `/r/{"data":{"columns":\["NAME","TIME","VALUE"\],"types":\["string","datetime","double"\],"cols":\[\["temp"\],\[1705291859000000000\],\[3.14\]\]},"success":true,"reason":"success","elapse":".+"}`,
		},
		{
			Name:      "db/query json timeformat rowsArray",
			Topic:     "db/query",
			Payload:   []byte(`{"q": "select * from example where name = 'temp'", "format":"json", "tz":"UTC", "timeformat": "DEFAULT", "rowsArray": true }`),
			Subscribe: "db/reply",
			Expect:    `/r/{"data":{"columns":\["NAME","TIME","VALUE"\],"types":\["string","datetime","double"\],"rows":\[{"NAME":"temp","TIME":"2024-01-15 04:10:59","VALUE":3.14}\]},"success":true,"reason":"success","elapse":".+"}`,
		},
		{
			Name:      "db/query simple, format=csv, reply",
			Topic:     "db/query",
			Payload:   []byte(`{"q": "select * from example where name = 'temp'", "format": "csv", "reply":"db/reply/123" }`),
			Subscribe: "db/reply/123",
			Expect:    "NAME,TIME,VALUE\ntemp,1705291859000000000,3.14\n\n",
		},
		{
			Name:      "db/query simple, format=csv",
			Topic:     "db/query",
			Payload:   []byte(`{"q": "select * from example where name = 'temp'", "format": "csv" }`),
			Subscribe: "db/reply",
			Expect:    "NAME,TIME,VALUE\ntemp,1705291859000000000,3.14\n\n",
		},
		{
			Name:      "db/query simple, format=csv, compress",
			Topic:     "db/query",
			Payload:   []byte(`{"q": "select * from example where name = 'temp'", "format": "csv", "compress":"gzip" }`),
			Subscribe: "db/reply",
			Expect:    compress([]byte("NAME,TIME,VALUE\ntemp,1705291859000000000,3.14\n\n")),
		},
		{
			Name:      "db/query simple, format=csv, timeformat",
			Topic:     "db/query",
			Payload:   []byte(`{"q": "select * from example where name = 'temp'", "format": "csv", "tz": "UTC", "timeformat": "DEFAULT" }`),
			Subscribe: "db/reply",
			Expect:    "NAME,TIME,VALUE\ntemp,2024-01-15 04:10:59,3.14\n\n",
		},
	}

	for _, ver := range []uint{4, 5} {
		for _, tt := range tests {
			t.Run(tt.Name, func(t *testing.T) {
				tt.Ver = ver
				runMqttTest(t, &tt)
			})
		}
	}
}

func TestWriteResponse(t *testing.T) {
	brokerUrl, err := url.Parse("tcp://" + mqttServerAddress)
	require.NoError(t, err)

	ctx := context.Background()

	cfg := autopaho.ClientConfig{
		ServerUrls:                    []*url.URL{brokerUrl},
		KeepAlive:                     20,
		CleanStartOnInitialConnection: true,
	}

	readyWg := sync.WaitGroup{}
	var receiveTopic string
	var receivePayload []byte
	cfg.OnConnectionUp = func(cm *autopaho.ConnectionManager, connAck *paho.Connack) {
		defer readyWg.Done()

		t.Log("CONN", connAck.ReasonCode)
		if connAck.ReasonCode != 0 {
			t.Fail()
			return
		}
		subAck, err := cm.Subscribe(ctx, &paho.Subscribe{
			Subscriptions: []paho.SubscribeOptions{
				{Topic: "db/reply/#", QoS: 1},
			},
		})
		if err != nil {
			t.Log("ERROR", "SUB", err.Error())
			t.Fail()
		}
		t.Log("SUB:", subAck.Reasons)
	}
	cfg.OnConnectError = func(err error) {
		t.Log("ERROR", "OnConnect", err.Error())
	}
	cfg.ClientConfig.ClientID = "mqtt2-test"
	cfg.ClientConfig.OnPublishReceived = append(cfg.ClientConfig.OnPublishReceived,
		func(r paho.PublishReceived) (bool, error) {
			receiveTopic = r.Packet.Topic
			receivePayload = r.Packet.Payload
			readyWg.Done()
			return true, nil
		})
	cfg.ClientConfig.OnClientError = func(err error) {
		t.Log("ERROR", "OnClient", err.Error())
	}
	cfg.ClientConfig.OnServerDisconnect = func(d *paho.Disconnect) {
		t.Log("ServerDisconnect", d.ReasonCode)
	}

	readyWg.Add(1)
	c, err := autopaho.NewConnection(ctx, cfg)
	require.NoError(t, err)
	defer c.Disconnect(ctx)
	readyWg.Wait()

	readyWg.Add(1)
	props := &paho.PublishProperties{}
	// props.ResponseTopic = "db/reply/123"
	props.User.Add("method", "insert")
	props.User.Add("format", "csv")
	props.User.Add("reply", "db/reply/123")
	c.Publish(ctx, &paho.Publish{
		Topic:      "db/write/example",
		Payload:    []byte(`my-car,1705291859000000000,1.2345`),
		QoS:        2,
		Properties: props,
	})
	readyWg.Wait()
	require.Equal(t, "db/reply/123", receiveTopic)
	response := map[string]any{}
	json.Unmarshal(receivePayload, &response)
	require.Equal(t, true, response["success"])
	require.Equal(t, "success, 1 record(s) inserted", response["reason"])
}

func TestMqttWrite(t *testing.T) {
	tests := []struct {
		Vers        []uint
		TC          MqttTestCase
		ExpectSql   string
		ExpectCount int
	}{
		{
			TC: MqttTestCase{
				Name:  "mqtt-write-json",
				Topic: "db/write/test_mqtt",
				Payload: []byte(`[
					["json1", 1705291859000000000, 1.2345],
					["json1", 1705291860000000000, 2.3456]
				]`),
			},
			ExpectSql:   `select count(*) from test_mqtt where name = 'json1'`,
			ExpectCount: 2,
		},
		{
			TC: MqttTestCase{
				Name:  "mqtt-write-json-columns",
				Topic: "db/write/test_mqtt",
				Payload: []byte(`
					{
						"data": {
							"columns": ["NAME","TIME","VALUE"],
							"rows": [
								["json2", 1705291861000000000, 1.2345],
								["json2", 1705291862000000000, 2.3456]
							]
						}
					}`),
			},
			ExpectSql:   `select count(*) from test_mqtt where name = 'json2'`,
			ExpectCount: 2,
		},
		{
			TC: MqttTestCase{
				Name:  "mqtt-write-ndjson",
				Topic: "db/write/test_mqtt",
				Payload: []byte(`{"NAME":"ndjson1", "TIME":1705291859, "VALUE":1.2345}` + "\n" +
					`{"NAME":"ndjson1", "TIME":1705291860, "VALUE":2.3456}` + "\n"),
				Properties: map[string]string{"format": "ndjson", "timeformat": "s"},
			},
			ExpectSql:   `select count(*) from test_mqtt where name = 'ndjson1'`,
			ExpectCount: 2,
		},
		{
			TC: MqttTestCase{
				Name:    "mqtt-write-csv",
				Topic:   "db/write/test_mqtt:csv",
				Payload: []byte("csv1,1705291863000000000,1.2345\ncsv1,1705291864000000000,2.3456"),
			},
			ExpectSql:   `select count(*) from test_mqtt where name = 'csv1'`,
			ExpectCount: 2,
		},
		{
			TC: MqttTestCase{
				Name:       "mqtt-write-csv-v5",
				Topic:      "db/write/test_mqtt",
				Properties: map[string]string{"format": "csv", "timeformat": "s"},
				Payload:    []byte("csv2,1705291865,1.2345\ncsv2,170529166,2.3456"),
			},
			ExpectSql:   `select count(*) from test_mqtt where name = 'csv2'`,
			ExpectCount: 2,
			Vers:        []uint{5},
		},
		{
			TC: MqttTestCase{
				Name:       "mqtt-write-csv-v5-time-value",
				Topic:      "db/write/test_mqtt",
				Properties: map[string]string{"format": "csv", "timeformat": "s", "header": "columns"},
				Payload:    []byte("TIME,VALUE,NAME\n1705291867,1.2345,csv3\n1705291868,2.3456,csv3"),
			},
			ExpectSql:   `select count(*) from test_mqtt where name = 'csv3'`,
			ExpectCount: 2,
			Vers:        []uint{5},
		},
		{
			TC: MqttTestCase{
				Name:    "mqtt-write-json-gzip",
				Topic:   "db/write/test_mqtt:json:gzip",
				Payload: compress([]byte(`[["json3", 1705291869000000000, 1.2345], ["json3", 1705291870000000000, 2.3456]]`)),
			},
			ExpectSql:   `select count(*) from test_mqtt where name = 'json3'`,
			ExpectCount: 2,
		},
		{
			TC: MqttTestCase{
				Name:    "mqtt-write-csv-gzip",
				Topic:   "db/write/test_mqtt:csv:gzip",
				Payload: compress([]byte("csv4,1705291871000000000,1.2345\ncsv4,1705291872000000000,2.3456")),
			},
			ExpectSql:   `select count(*) from test_mqtt where name = 'csv4'`,
			ExpectCount: 2,
		},
		{
			TC: MqttTestCase{
				Name:    "mqtt-write-ilp",
				Topic:   "db/metrics/test_mqtt",
				Payload: []byte("ilp speed=1.2345 1732742196000000000\nilp speed=2.3456 1732742197000000000\n"),
			},
			ExpectSql:   `select count(*) from test_mqtt where name = 'ilp.speed'`,
			ExpectCount: 2,
		},
	}

	at, _, err := jwtLogin("sys", "manager")
	require.NoError(t, err)

	creTable := `create tag table test_mqtt (
		name varchar(200) primary key,
		time datetime basetime,
		value double -- summarized,
		-- jsondata json,
		-- ival int,
		-- sval short
	)`
	req, _ := http.NewRequest(http.MethodGet, httpServerAddress+"/db/query?q="+url.QueryEscape(creTable), nil)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", at))
	rsp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, rsp.StatusCode)
	rsp.Body.Close()

	defer func() {
		dropTable := `drop table test_mqtt`
		req, _ := http.NewRequest(http.MethodGet, httpServerAddress+"/db/query?q="+url.QueryEscape(dropTable), nil)
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", at))
		rsp, _ := http.DefaultClient.Do(req)
		require.Equal(t, http.StatusOK, rsp.StatusCode)
	}()

	for _, tt := range tests {
		vers := tt.Vers
		if len(vers) == 0 {
			vers = []uint{4, 5}
		}
		for _, ver := range vers {
			t.Run(tt.TC.Name, func(t *testing.T) {
				tt.TC.Ver = ver
				runMqttTest(t, &tt.TC)

				conn, err := mqttServer.db.Connect(context.Background(), api.WithTrustUser("sys"))
				require.NoError(t, err)
				conn.QueryRow(context.Background(), "EXEC table_flush(test_mqtt)")
				var count int
				conn.QueryRow(context.Background(), tt.ExpectSql).Scan(&count)
				require.Equal(t, tt.ExpectCount, count)
				conn.Close()
			})
		}
	}
}

func TestAppend(t *testing.T) {
	jsonData := []byte(`[["my-car", 1705291859000000000, 1.2345], ["my-car", 1705291860000000000, 2.3456]]`)
	csvData := []byte("my-car,1705291859000000000,1.2345\nmy-car,1705291860000000000,2.3456")
	jsonGzipData := compress(jsonData)
	csvGzipData := compress(csvData)
	tests := []MqttTestCase{
		{
			Name:    "db/append/example",
			Topic:   "db/append/example",
			Payload: jsonData,
			Ver:     uint(4),
		},
		{
			Name:    "db/append/example",
			Topic:   "db/append/example",
			Payload: jsonData,
			Ver:     uint(5),
		},
		{
			Name:       "db/write/example?method=append",
			Topic:      "db/write/example",
			Payload:    jsonData,
			Ver:        uint(5),
			Properties: map[string]string{"method": "append"},
		},
		{
			Name:    "db/append/example json",
			Topic:   "db/append/example:json",
			Payload: jsonData,
			Ver:     uint(4),
		},
		{
			Name:    "db/append/example json",
			Topic:   "db/append/example:json",
			Payload: jsonData,
			Ver:     uint(5),
		},
		{
			Name:    "db/append/example json gzip",
			Topic:   "db/append/example:json:gzip",
			Payload: jsonGzipData,
			Ver:     uint(4),
		},
		{
			Name:    "db/append/example json gzip",
			Topic:   "db/append/example:json:gzip",
			Payload: jsonGzipData,
			Ver:     uint(5),
		},
		{
			Name:       "db/write/example?method=append&format=json&compress=gzip",
			Topic:      "db/write/example",
			Payload:    jsonGzipData,
			Ver:        uint(5),
			Properties: map[string]string{"method": "append", "format": "json", "compress": "gzip"},
		},
		{
			Name:    "db/append/example csv",
			Topic:   "db/append/example:csv",
			Payload: csvData,
			Ver:     uint(4),
		},
		{
			Name:    "db/append/example csv",
			Topic:   "db/append/example:csv",
			Payload: csvData,
			Ver:     uint(5),
		},
		{
			Name:    "db/append/example csv gzip",
			Topic:   "db/append/example:csv: gzip",
			Payload: csvGzipData,
			Ver:     uint(4),
		},
		{
			Name:       "db/write/example?format=csv&method=append",
			Topic:      "db/write/example",
			Payload:    csvData,
			Ver:        uint(5),
			Properties: map[string]string{"method": "append", "format": "csv"},
		},
		{
			Name:    "db/append/example csv gzip",
			Topic:   "db/append/example:csv: gzip",
			Payload: csvGzipData,
			Ver:     uint(5),
		},
		{
			Name:  "db/write/example?format=ndjson&method=append",
			Topic: "db/write/example",
			Payload: []byte(
				`{"NAME":"my-car", "TIME":1705291859, "VALUE":1.2345}` + "\n" +
					`{"NAME":"my-car", "TIME":1705291860, "VALUE":2.3456}` + "\n"),
			Ver:        uint(5),
			Properties: map[string]string{"method": "append", "format": "ndjson", "timeformat": "s"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			runMqttTest(t, &tt)

			conn, err := mqttServer.db.Connect(context.Background(), api.WithTrustUser("sys"))
			if err != nil {
				t.Fatalf("Test %q failed, connect error: %s", tt.Name, err.Error())
			}
			defer conn.Close()
			conn.QueryRow(context.Background(), "EXEC table_flush(example)")
			var count int
			var tag = "my-car"
			conn.QueryRow(context.Background(), "select count(*) from example where name = ?", tag).Scan(&count)
			if count != 2 {
				t.Logf("Test %q expect 2 rows, got %d", tt.Name, count)
				t.Fail()
			}
			conn.QueryRow(context.Background(), "delete from example where name = ?", tag)
		})
	}
}

func compress(data []byte) []byte {
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)

	_, err := zw.Write(data)
	if err != nil {
		panic(err)
	}

	zw.Close()

	return buf.Bytes()
}

func TestTql(t *testing.T) {
	csvData := []byte("my-car,1705291859000000000,1.2345\nmy-car,1705291860000000000,2.3456")

	tests := []MqttTestCase{
		{
			Name:    "db/tql/csv_append.tql",
			Topic:   "db/tql/csv_append.tql",
			Payload: csvData,
		},
	}
	for _, ver := range []uint{4, 5} {
		for _, tt := range tests {
			t.Run(tt.Name, func(t *testing.T) {
				tt.Ver = ver
				runMqttTest(t, &tt)

				conn, err := mqttServer.db.Connect(context.Background(), api.WithTrustUser("sys"))
				if err != nil {
					t.Fatalf("Test %q failed, connect error: %s", tt.Name, err.Error())
				}
				defer conn.Close()
				conn.QueryRow(context.Background(), "EXEC table_flush(example)")
				var count int
				var tag = "my-car"
				conn.QueryRow(context.Background(), "select count(*) from example where name = ?", tag).Scan(&count)
				if count != 2 {
					t.Logf("Test %q expect 2 rows, got %d", tt.Name, count)
					t.Fail()
				}
				conn.QueryRow(context.Background(), "delete from example where name = ?", tag)
			})
		}
	}
}
