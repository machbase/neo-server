package server

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/eclipse/paho.golang/autopaho"
	"github.com/eclipse/paho.golang/paho"
	"github.com/machbase/neo-server/v8/mods/logging"
	"github.com/machbase/neo-server/v8/spi"
	mqtt "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/packets"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
	"golang.org/x/crypto/ssh"
)

type MqttTestCase struct {
	Ver  uint
	Name string

	Topic      string
	Payload    []byte
	Properties map[string]string

	Subscribe  string
	ExpectFunc func(*testing.T, []byte)
	ExpectCSV  []string
	ExpectBin  []byte
}

func runMqttTest(t *testing.T, tc *MqttTestCase) {
	t.Helper()

	brokerUrl, err := url.Parse("tcp://" + mqttServerAddress)
	require.NoError(t, err)

	wg := sync.WaitGroup{}

	var recvPayload []byte

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
				if _, err := cm.Subscribe(t.Context(), &paho.Subscribe{
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

	wg.Add(1)
	c, err := autopaho.NewConnection(t.Context(), cliCfg)
	if err != nil {
		t.Logf("Test %q failed, connect error: %s", tc.Name, err.Error())
		t.Fail()
	}
	defer c.Disconnect(t.Context())

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

	pubAck, err := c.Publish(t.Context(), pub)
	if err != nil {
		t.Fatalf("Test %q failed, publish error: %s", tc.Name, err.Error())
	}
	if pubAck.ReasonCode != 0 {
		t.Fatalf("Test %q failed, publish failed: %d", tc.Name, pubAck.ReasonCode)
	}

	if tc.Subscribe != "" {
		wg.Wait() // wait message
	}

	if tc.ExpectFunc != nil {
		tc.ExpectFunc(t, recvPayload)
		return
	}
	if tc.ExpectCSV != nil {
		require.EqualValues(t, strings.Join(tc.ExpectCSV, "\n"), string(recvPayload))
		return
	}
	if tc.ExpectBin != nil {
		require.Equal(t, hex.Dump(tc.ExpectBin), hex.Dump(recvPayload))
		return
	}
}

func TestMqttQuery(t *testing.T) {
	tests := []MqttTestCase{
		{
			Name:      "query_simple",
			Topic:     "db/query",
			Payload:   []byte(`{"q": "select * from example where name = 'temp'" }`),
			Subscribe: "db/reply",
			ExpectFunc: func(t *testing.T, payload []byte) {
				strPayload := string(payload)
				require.True(t, gjson.Get(strPayload, "success").Bool(), strPayload)
				require.Equal(t, "success", gjson.Get(strPayload, "reason").String(), strPayload)
				require.Equal(t, `["NAME","TIME","VALUE"]`, gjson.Get(strPayload, "data.columns").String(), strPayload)
				require.Equal(t, `["string","datetime","double"]`, gjson.Get(strPayload, "data.types").String(), strPayload)
				require.Equal(t, `temp`, gjson.Get(strPayload, "data.rows.0.0").String(), strPayload)
				require.Equal(t, testTimeTick.UnixNano(), gjson.Get(strPayload, "data.rows.0.1").Int(), strPayload)
				require.Equal(t, 3.14, gjson.Get(strPayload, "data.rows.0.2").Float(), strPayload)
			},
		},
		{
			Name:      "query_simple_timeformat",
			Topic:     "db/query",
			Payload:   []byte(`{"q": "select * from example where name = 'temp'", "format":"json", "tz":"UTC", "timeformat": "DEFAULT" }`),
			Subscribe: "db/reply",
			ExpectFunc: func(t *testing.T, payload []byte) {
				strPayload := string(payload)
				require.True(t, gjson.Get(strPayload, "success").Bool(), strPayload)
				require.Equal(t, "success", gjson.Get(strPayload, "reason").String(), strPayload)
				require.Equal(t, `["NAME","TIME","VALUE"]`, gjson.Get(strPayload, "data.columns").String(), strPayload)
				require.Equal(t, `["string","datetime","double"]`, gjson.Get(strPayload, "data.types").String(), strPayload)
				require.Equal(t, `temp`, gjson.Get(strPayload, "data.rows.0.0").String(), strPayload)
				require.Equal(t, "2024-01-15 04:10:59", gjson.Get(strPayload, "data.rows.0.1").String(), strPayload)
				require.Equal(t, 3.14, gjson.Get(strPayload, "data.rows.0.2").Float(), strPayload)
			},
		},
		{
			Name:      "query_bind_params",
			Topic:     "db/query",
			Payload:   []byte(`{"q": "select * from example where name = ?", "p":["temp"] }`),
			Subscribe: "db/reply",
			ExpectFunc: func(t *testing.T, payload []byte) {
				strPayload := string(payload)
				require.True(t, gjson.Get(strPayload, "success").Bool(), strPayload)
				require.Equal(t, "success", gjson.Get(strPayload, "reason").String(), strPayload)
				require.Equal(t, `temp`, gjson.Get(strPayload, "data.rows.0.0").String(), strPayload)
				require.Equal(t, testTimeTick.UnixNano(), gjson.Get(strPayload, "data.rows.0.1").Int(), strPayload)
				require.Equal(t, 3.14, gjson.Get(strPayload, "data.rows.0.2").Float(), strPayload)
			},
		},
		{
			Name:      "query_bind_params_invalid_nested",
			Topic:     "db/query",
			Payload:   []byte(`{"q": "select * from example where name = ?", "p":[["temp"]] }`),
			Subscribe: "db/reply",
			ExpectFunc: func(t *testing.T, payload []byte) {
				strPayload := string(payload)
				require.False(t, gjson.Get(strPayload, "success").Bool(), strPayload)
				require.Contains(t, gjson.Get(strPayload, "reason").String(), "bind parameter must be scalar", strPayload)
			},
		},
		{
			Name:      "query_json_timeformat_rowsFlatten",
			Topic:     "db/query",
			Payload:   []byte(`{"q": "select * from example where name = 'temp'", "format":"json", "tz":"UTC", "timeformat": "DEFAULT", "rowsFlatten": true }`),
			Subscribe: "db/reply",
			ExpectFunc: func(t *testing.T, payload []byte) {
				strPayload := string(payload)
				require.True(t, gjson.Get(strPayload, "success").Bool(), strPayload)
				require.Equal(t, "success", gjson.Get(strPayload, "reason").String(), strPayload)
				require.Equal(t, `["NAME","TIME","VALUE"]`, gjson.Get(strPayload, "data.columns").String(), strPayload)
				require.Equal(t, `["string","datetime","double"]`, gjson.Get(strPayload, "data.types").String(), strPayload)
				require.Equal(t, `temp`, gjson.Get(strPayload, "data.rows.0").String(), strPayload)
				require.Equal(t, "2024-01-15 04:10:59", gjson.Get(strPayload, "data.rows.1").String(), strPayload)
				require.Equal(t, 3.14, gjson.Get(strPayload, "data.rows.2").Float(), strPayload)
			},
		},
		{
			Name:      "query_json_transpose",
			Topic:     "db/query",
			Payload:   []byte(`{"q": "select * from example where name = 'temp'", "format":"json", "transpose": true }`),
			Subscribe: "db/reply",
			ExpectFunc: func(t *testing.T, payload []byte) {
				strPayload := string(payload)
				require.True(t, gjson.Get(strPayload, "success").Bool(), strPayload)
				require.Equal(t, "success", gjson.Get(strPayload, "reason").String(), strPayload)
				require.Equal(t, `["NAME","TIME","VALUE"]`, gjson.Get(strPayload, "data.columns").String(), strPayload)
				require.Equal(t, `["string","datetime","double"]`, gjson.Get(strPayload, "data.types").String(), strPayload)
				require.Equal(t, `["temp"]`, gjson.Get(strPayload, "data.cols.0").String(), strPayload)
				require.Equal(t, "[1705291859000000000]", gjson.Get(strPayload, "data.cols.1").String(), strPayload)
				require.Equal(t, `[3.14]`, gjson.Get(strPayload, "data.cols.2").String(), strPayload)
			},
		},
		{
			Name:      "query_json_timeformat_rowsArray",
			Topic:     "db/query",
			Payload:   []byte(`{"q": "select * from example where name = 'temp'", "format":"json", "tz":"UTC", "timeformat": "DEFAULT", "rowsArray": true }`),
			Subscribe: "db/reply",
			ExpectFunc: func(t *testing.T, payload []byte) {
				strPayload := string(payload)
				require.True(t, gjson.Get(strPayload, "success").Bool(), strPayload)
				require.Equal(t, "success", gjson.Get(strPayload, "reason").String(), strPayload)
				require.Equal(t, `["NAME","TIME","VALUE"]`, gjson.Get(strPayload, "data.columns").String(), strPayload)
				require.Equal(t, `["string","datetime","double"]`, gjson.Get(strPayload, "data.types").String(), strPayload)
				require.Equal(t, `temp`, gjson.Get(strPayload, "data.rows.0.NAME").String(), strPayload)
				require.Equal(t, `2024-01-15 04:10:59`, gjson.Get(strPayload, "data.rows.0.TIME").String(), strPayload)
				require.Equal(t, 3.14, gjson.Get(strPayload, "data.rows.0.VALUE").Float(), strPayload)
			},
		},
		{
			Name:      "query_simple_format=csv_reply",
			Topic:     "db/query",
			Payload:   []byte(`{"q": "select * from example where name = 'temp'", "format": "csv", "reply":"db/reply/123" }`),
			Subscribe: "db/reply/123",
			ExpectCSV: []string{"NAME,TIME,VALUE", "temp,1705291859000000000,3.14", "\n"},
		},
		{
			Name:      "query_simple_format=csv",
			Topic:     "db/query",
			Payload:   []byte(`{"q": "select * from example where name = 'temp'", "format": "csv" }`),
			Subscribe: "db/reply",
			ExpectCSV: []string{"NAME,TIME,VALUE", "temp,1705291859000000000,3.14", "\n"},
		},
		{
			Name:      "query_simple_format=csv_compress",
			Topic:     "db/query",
			Payload:   []byte(`{"q": "select * from example where name = 'temp'", "format": "csv", "compress":"gzip" }`),
			Subscribe: "db/reply",
			ExpectBin: compress([]byte("NAME,TIME,VALUE\ntemp,1705291859000000000,3.14\n\n")),
		},
		{
			Name:      "query_simple_format=csv_timeformat_default",
			Topic:     "db/query",
			Payload:   []byte(`{"q": "select * from example where name = 'temp'", "format": "csv", "tz": "UTC", "timeformat": "DEFAULT" }`),
			Subscribe: "db/reply",
			ExpectCSV: []string{"NAME,TIME,VALUE", "temp,2024-01-15 04:10:59,3.14", "\n"},
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

func TestMqttQueryFailures(t *testing.T) {
	defer func() {
		conn, err := spi.Connect(t.Context(), "sys")
		if err != nil {
			t.Fatalf("Test cleanup failed, connect error: %s", err.Error())
		}
		defer conn.Close()
		conn.ExecContext(t.Context(), `drop table mqtt_query_exec`)
	}()

	tests := []MqttTestCase{
		{
			Name:      "query_invalid_json",
			Topic:     "db/query",
			Payload:   []byte(`{"q":`),
			Subscribe: "db/reply",
			ExpectFunc: func(t *testing.T, payload []byte) {
				strPayload := string(payload)
				require.False(t, gjson.Get(strPayload, "success").Bool(), strPayload)
				require.Contains(t, gjson.Get(strPayload, "reason").String(), "unexpected EOF", strPayload)
			},
		},
		{
			Name:      "query_invalid_tz_custom_reply",
			Topic:     "db/query",
			Payload:   []byte(`{"q":"select 1","tz":"Invalid/Zone","reply":"db/reply/query-failure"}`),
			Subscribe: "db/reply/query-failure",
			ExpectFunc: func(t *testing.T, payload []byte) {
				strPayload := string(payload)
				require.False(t, gjson.Get(strPayload, "success").Bool(), strPayload)
				require.Contains(t, gjson.Get(strPayload, "reason").String(), "unknown time zone", strPayload)
			},
		},
		{
			Name:      "query_sql_error",
			Topic:     "db/query",
			Payload:   []byte(`{"q":"select * from missing_mqtt_query_table"}`),
			Subscribe: "db/reply",
			ExpectFunc: func(t *testing.T, payload []byte) {
				strPayload := string(payload)
				require.False(t, gjson.Get(strPayload, "success").Bool(), strPayload)
				require.NotEmpty(t, gjson.Get(strPayload, "reason").String(), strPayload)
			},
		},
		{
			Name:      "query_statement_success",
			Topic:     "db/query",
			Payload:   []byte(`{"q":"create tag table mqtt_query_exec (name varchar(20) primary key, time datetime basetime, value double)","reply":"db/reply/query-exec"}`),
			Subscribe: "db/reply/query-exec",
			ExpectFunc: func(t *testing.T, payload []byte) {
				strPayload := string(payload)
				require.True(t, gjson.Get(strPayload, "success").Bool(), strPayload)
				require.NotEmpty(t, gjson.Get(strPayload, "reason").String(), strPayload)
				require.False(t, gjson.Get(strPayload, "data").Exists(), strPayload)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			runMqttTest(t, &tt)
		})
	}
}

func TestWriteResponse(t *testing.T) {
	brokerUrl, err := url.Parse("tcp://" + mqttServerAddress)
	require.NoError(t, err)

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
		subAck, err := cm.Subscribe(t.Context(), &paho.Subscribe{
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
	c, err := autopaho.NewConnection(t.Context(), cfg)
	require.NoError(t, err)
	defer c.Disconnect(t.Context())
	readyWg.Wait()

	readyWg.Add(1)
	props := &paho.PublishProperties{}
	// props.ResponseTopic = "db/reply/123"
	props.User.Add("method", "insert")
	props.User.Add("format", "csv")
	props.User.Add("reply", "db/reply/123")
	c.Publish(t.Context(), &paho.Publish{
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
				Name:       "mqtt-write-csv-v5-time-tz-value",
				Topic:      "db/write/test_mqtt",
				Properties: map[string]string{"format": "csv", "timeformat": "2006-01-02 15:04:05", "tz": "Asia/Seoul", "header": "columns"},
				Payload:    []byte("TIME,VALUE,NAME\n2024-01-15 13:11:07,1.2345,csv4\n2024-01-15 13:11:08,2.3456,csv4"),
			},
			ExpectSql:   `select count(*) from test_mqtt where name = 'csv4'`,
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
				Payload: compress([]byte("csv5,1705291871000000000,1.2345\ncsv5,1705291872000000000,2.3456")),
			},
			ExpectSql:   `select count(*) from test_mqtt where name = 'csv5'`,
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

	creTable := `create tag table test_mqtt (
		name varchar(200) primary key,
		time datetime basetime,
		value double -- summarized,
		-- jsondata json,
		-- ival int,
		-- sval short
	) TAG_DUPLICATE_CHECK_DURATION=1;`
	req, _ := http.NewRequest(http.MethodGet, httpServerAddress+"/db/query?q="+url.QueryEscape(creTable), nil)
	rsp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, rsp.StatusCode)
	rsp.Body.Close()

	defer func() {
		dropTable := `drop table test_mqtt`
		req, _ := http.NewRequest(http.MethodGet, httpServerAddress+"/db/query?q="+url.QueryEscape(dropTable), nil)
		rsp, _ := http.DefaultClient.Do(req)
		require.Equal(t, http.StatusOK, rsp.StatusCode)
	}()

	for _, tt := range tests {
		vers := tt.Vers
		if len(vers) == 0 {
			vers = []uint{4, 5}
		}
		for n, ver := range vers {
			t.Run(tt.TC.Name, func(t *testing.T) {
				tt.TC.Ver = ver
				runMqttTest(t, &tt.TC)

				conn, err := spi.Connect(t.Context(), "sys")
				require.NoError(t, err)
				_, err = conn.ExecContext(t.Context(), "EXEC table_flush(test_mqtt)")
				if err != nil {
					t.Fatalf("Test %q failed, table_flush error: %s", tt.TC.Name, err.Error())
				}
				var count int
				result := conn.QueryRowContext(t.Context(), tt.ExpectSql)
				if result.Err() != nil {
					t.Fatalf("Test %q failed, query error: %s", tt.TC.Name, result.Err().Error())
				}
				err = result.Scan(&count)
				require.NoError(t, err)
				require.Equal(t, tt.ExpectCount*(n+1), count)
				conn.Close()
			})
		}
	}
}

// TestMqttWriteWithDatabaseParam verifies the MQTT v5 "db" user property support
// for multiple-database writes (machbase/neo#1484).
func TestMqttWriteWithDatabaseParam(t *testing.T) {
	dbName := fmt.Sprintf("MQTTWRITEDB%d", time.Now().UnixNano()%1000000)
	tableName := fmt.Sprintf("MQTT_WRITE_DB_T_%d", time.Now().UnixNano())

	doQuery := func(t *testing.T, sqlText string, db string) *http.Response {
		t.Helper()
		params := url.Values{"q": []string{sqlText}}
		if db != "" {
			params.Set("db", db)
		}
		req, _ := http.NewRequest(http.MethodGet, httpServerAddress+"/db/query?"+params.Encode(), nil)
		rsp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		rsp.Body.Close()
		return rsp
	}

	require.Equal(t, http.StatusOK, doQuery(t, "CREATE DATABASE IF NOT EXISTS "+dbName, "").StatusCode)
	t.Cleanup(func() {
		require.Equal(t, http.StatusOK, doQuery(t, "DROP DATABASE "+dbName+" CASCADE", "").StatusCode)
	})

	creTable := fmt.Sprintf(`create tag table %s (name varchar(200) primary key, time datetime basetime, value double)`, tableName)
	require.Equal(t, http.StatusOK, doQuery(t, creTable, "").StatusCode)
	t.Cleanup(func() {
		require.Equal(t, http.StatusOK, doQuery(t, "drop table "+tableName, "").StatusCode)
	})
	require.Equal(t, http.StatusOK, doQuery(t, creTable, dbName).StatusCode)

	runMqttTest(t, &MqttTestCase{
		Ver:        5,
		Name:       "mqtt-write-db-param",
		Topic:      "db/write/" + tableName,
		Properties: map[string]string{"db": dbName},
		Payload:    []byte(`[["mqtt-other-db", 1705291859000000000, 1.5]]`),
	})

	conn, err := spi.Connect(t.Context(), "sys")
	require.NoError(t, err)
	defer conn.Close()
	_, err = conn.ExecContext(t.Context(), "USE "+dbName)
	require.NoError(t, err)
	_, err = conn.ExecContext(t.Context(), "EXEC table_flush("+tableName+")")
	require.NoError(t, err)
	var count int
	require.NoError(t, conn.QueryRowContext(t.Context(), "select count(*) from "+tableName+" where name = 'mqtt-other-db'").Scan(&count))
	require.Equal(t, 1, count)
	_, err = conn.ExecContext(t.Context(), "USE MACHBASEDB")
	require.NoError(t, err)
	require.NoError(t, conn.QueryRowContext(t.Context(), "select count(*) from "+tableName+" where name = 'mqtt-other-db'").Scan(&count))
	require.Equal(t, 0, count)
}

// TestMqttWriteQualifiedTableNamePrecedence verifies that a "db.user.table"/
// "user.table" qualifier embedded in the MQTT topic takes precedence over the
// "db" v5 user property, for both insert and append, and that the qualifier
// works even over plain MQTT v3.1 (no user properties) topics.
func TestMqttWriteQualifiedTableNamePrecedence(t *testing.T) {
	db1 := fmt.Sprintf("MQTTWRITEQDB1%d", time.Now().UnixNano()%1000000)
	db2 := fmt.Sprintf("MQTTWRITEQDB2%d", time.Now().UnixNano()%1000000)
	tableName := fmt.Sprintf("MQTT_WRITE_Q_T_%d", time.Now().UnixNano())

	doQuery := func(t *testing.T, sqlText string, db string) *http.Response {
		t.Helper()
		params := url.Values{"q": []string{sqlText}}
		if db != "" {
			params.Set("db", db)
		}
		req, _ := http.NewRequest(http.MethodGet, httpServerAddress+"/db/query?"+params.Encode(), nil)
		rsp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		rsp.Body.Close()
		return rsp
	}

	for _, db := range []string{db1, db2} {
		require.Equal(t, http.StatusOK, doQuery(t, "CREATE DATABASE IF NOT EXISTS "+db, "").StatusCode)
		db := db // capture
		t.Cleanup(func() {
			require.Equal(t, http.StatusOK, doQuery(t, "DROP DATABASE "+db+" CASCADE", "").StatusCode)
		})
	}

	creTable := fmt.Sprintf(`create tag table %s (name varchar(200) primary key, time datetime basetime, value double)`, tableName)
	for _, db := range []string{"", db1, db2} {
		require.Equal(t, http.StatusOK, doQuery(t, creTable, db).StatusCode)
	}
	t.Cleanup(func() {
		require.Equal(t, http.StatusOK, doQuery(t, "drop table "+tableName, "").StatusCode)
	})

	// countIn counts rows with the given name in the given database, after
	// flushing the TAG table's index (needed for both plain insert and append,
	// since TAG tables only update their index lazily).
	countIn := func(t *testing.T, db string, name string) int {
		t.Helper()
		conn, err := spi.Connect(t.Context(), "sys")
		require.NoError(t, err)
		defer conn.Close()
		if db != "" {
			_, err = conn.ExecContext(t.Context(), "USE "+db)
			require.NoError(t, err)
		}
		_, err = conn.ExecContext(t.Context(), "EXEC table_flush("+tableName+")")
		require.NoError(t, err)
		var count int
		require.NoError(t, conn.QueryRowContext(t.Context(),
			"select count(*) from "+tableName+" where name = '"+name+"'").Scan(&count))
		return count
	}

	t.Run("insert_v3_topic_db_user_table", func(t *testing.T) {
		// Plain MQTT v3.1 topic (no user properties): db/write/<db1>.SYS.<table>
		runMqttTest(t, &MqttTestCase{
			Ver:     4,
			Name:    "mqtt-insert-v3-db-user-table",
			Topic:   "db/write/" + db1 + ".SYS." + tableName,
			Payload: []byte(`[["v3-path-qualified", 1705291859000000000, 1.5]]`),
		})
		require.Equal(t, 1, countIn(t, db1, "v3-path-qualified"))
		require.Equal(t, 0, countIn(t, "", "v3-path-qualified"))
	})

	t.Run("insert_v5_user_table_with_db_property", func(t *testing.T) {
		// db/write/SYS.<table> with the "db" user property -> db1
		runMqttTest(t, &MqttTestCase{
			Ver:        5,
			Name:       "mqtt-insert-v5-user-table-db-property",
			Topic:      "db/write/SYS." + tableName,
			Properties: map[string]string{"db": db1},
			Payload:    []byte(`[["v5-property-db", 1705291860000000000, 1.5]]`),
		})
		require.Equal(t, 1, countIn(t, db1, "v5-property-db"))
		require.Equal(t, 0, countIn(t, "", "v5-property-db"))
	})

	t.Run("insert_path_db_wins_over_property", func(t *testing.T) {
		// db/write/<db1>.SYS.<table> with "db" user property db2 -> db1 wins
		runMqttTest(t, &MqttTestCase{
			Ver:        5,
			Name:       "mqtt-insert-path-wins",
			Topic:      "db/write/" + db1 + ".SYS." + tableName,
			Properties: map[string]string{"db": db2},
			Payload:    []byte(`[["insert-path-wins", 1705291861000000000, 1.5]]`),
		})
		require.Equal(t, 1, countIn(t, db1, "insert-path-wins"))
		require.Equal(t, 0, countIn(t, db2, "insert-path-wins"))
	})

	t.Run("append_v3_topic_db_user_table", func(t *testing.T) {
		// Plain MQTT v3.1 topic (no user properties): db/append/<db1>.SYS.<table>
		runMqttTest(t, &MqttTestCase{
			Ver:     4,
			Name:    "mqtt-append-v3-db-user-table",
			Topic:   "db/append/" + db1 + ".SYS." + tableName,
			Payload: []byte(`["append-v3-path-qualified", 1705291862000000000, 1.5]`),
		})
		spi.FlushAppendWorkers(db1, "SYS", tableName)
		require.Equal(t, 1, countIn(t, db1, "append-v3-path-qualified"))
		require.Equal(t, 0, countIn(t, "", "append-v3-path-qualified"))
	})

	t.Run("append_v5_user_table_with_db_property", func(t *testing.T) {
		// db/append/SYS.<table> with the "db" user property -> db1
		runMqttTest(t, &MqttTestCase{
			Ver:        5,
			Name:       "mqtt-append-v5-user-table-db-property",
			Topic:      "db/append/SYS." + tableName,
			Properties: map[string]string{"db": db1},
			Payload:    []byte(`["append-v5-property-db", 1705291863000000000, 1.5]`),
		})
		spi.FlushAppendWorkers(db1, "SYS", tableName)
		require.Equal(t, 1, countIn(t, db1, "append-v5-property-db"))
		require.Equal(t, 0, countIn(t, "", "append-v5-property-db"))
	})

	t.Run("append_path_db_wins_over_property", func(t *testing.T) {
		// db/append/<db1>.SYS.<table> with "db" user property db2 -> db1 wins
		runMqttTest(t, &MqttTestCase{
			Ver:        5,
			Name:       "mqtt-append-path-wins",
			Topic:      "db/append/" + db1 + ".SYS." + tableName,
			Properties: map[string]string{"db": db2},
			Payload:    []byte(`["append-path-wins", 1705291864000000000, 1.5]`),
		})
		spi.FlushAppendWorkers(db1, "SYS", tableName)
		require.Equal(t, 1, countIn(t, db1, "append-path-wins"))
		require.Equal(t, 0, countIn(t, db2, "append-path-wins"))
	})
}

func TestMqttWriteFailures(t *testing.T) {
	at, _, err := jwtLogin("sys", "manager")
	require.NoError(t, err)

	creTable := `create tag table test_mqtt_failure (
		name varchar(200) primary key,
		time datetime basetime,
		value double
	) TAG_DUPLICATE_CHECK_DURATION=1;`
	req, _ := http.NewRequest(http.MethodGet, httpServerAddress+"/db/query?q="+url.QueryEscape(creTable), nil)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", at))
	rsp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, rsp.StatusCode)
	rsp.Body.Close()

	defer func() {
		dropTable := `drop table test_mqtt_failure`
		req, _ := http.NewRequest(http.MethodGet, httpServerAddress+"/db/query?q="+url.QueryEscape(dropTable), nil)
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", at))
		rsp, _ := http.DefaultClient.Do(req)
		require.Equal(t, http.StatusOK, rsp.StatusCode)
	}()

	tests := []MqttTestCase{
		{
			Name:       "mqtt-write-unsupported-format",
			Topic:      "db/write/test_mqtt_failure",
			Payload:    []byte(`[["bad-format",1705291859000000000,1.23]]`),
			Properties: map[string]string{"reply": "db/reply/write-failure", "format": "xml"},
			Subscribe:  "db/reply/write-failure",
			ExpectFunc: func(t *testing.T, payload []byte) {
				strPayload := string(payload)
				require.False(t, gjson.Get(strPayload, "success").Bool(), strPayload)
				require.Contains(t, gjson.Get(strPayload, "reason").String(), `unsupported format "xml"`, strPayload)
			},
		},
		{
			Name:       "mqtt-write-unsupported-compress",
			Topic:      "db/write/test_mqtt_failure",
			Payload:    []byte(`[["bad-compress",1705291859000000000,1.23]]`),
			Properties: map[string]string{"reply": "db/reply/write-failure", "compress": "zip"},
			Subscribe:  "db/reply/write-failure",
			ExpectFunc: func(t *testing.T, payload []byte) {
				strPayload := string(payload)
				require.False(t, gjson.Get(strPayload, "success").Bool(), strPayload)
				require.Contains(t, gjson.Get(strPayload, "reason").String(), `unsupported compress "zip"`, strPayload)
			},
		},
		{
			Name:       "mqtt-write-missing-table",
			Topic:      "db/write/",
			Payload:    []byte(`[]`),
			Properties: map[string]string{"reply": "db/reply/write-failure"},
			Subscribe:  "db/reply/write-failure",
			ExpectFunc: func(t *testing.T, payload []byte) {
				strPayload := string(payload)
				require.False(t, gjson.Get(strPayload, "success").Bool(), strPayload)
				require.Equal(t, "table is not specified", gjson.Get(strPayload, "reason").String(), strPayload)
			},
		},
		{
			Name:       "mqtt-write-invalid-gzip",
			Topic:      "db/write/test_mqtt_failure:json:gzip",
			Payload:    []byte("not-gzip"),
			Properties: map[string]string{"reply": "db/reply/write-failure"},
			Subscribe:  "db/reply/write-failure",
			ExpectFunc: func(t *testing.T, payload []byte) {
				strPayload := string(payload)
				require.False(t, gjson.Get(strPayload, "success").Bool(), strPayload)
				require.Contains(t, gjson.Get(strPayload, "reason").String(), "fail to unzip", strPayload)
			},
		},
		{
			Name:      "mqtt-write-unknown-column",
			Topic:     "db/write/test_mqtt_failure",
			Payload:   []byte(`{"reply":"db/reply/write-failure","data":{"columns":["NAME","MISSING","VALUE"],"rows":[["bad-column",1705291859000000000,1.23]]}}`),
			Subscribe: "db/reply/write-failure",
			ExpectFunc: func(t *testing.T, payload []byte) {
				strPayload := string(payload)
				require.False(t, gjson.Get(strPayload, "success").Bool(), strPayload)
				require.Contains(t, gjson.Get(strPayload, "reason").String(), `column "MISSING" not found`, strPayload)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			runMqttTest(t, &tt)
		})
	}
}

func TestAppend(t *testing.T) {
	jsonData := []byte(`[["my-append", 1705291859000000000, 1.2345], ["my-append", 1705291860000000000, 2.3456]]`)
	ndjsonData := []byte(`{"NAME":"my-append", "TIME":1705291859, "VALUE":1.2345}` + "\n" +
		`{"NAME":"my-append", "TIME":1705291860, "VALUE":2.3456}` + "\n")
	csvData := []byte("my-append,1705291859000000000,1.2345\nmy-append,1705291860000000000,2.3456")
	csvDataWithHeader := []byte("Value,time,NAme\n1.2345,1705291859000000000,my-append\n2.3456,1705291860000000000,my-append")
	jsonGzipData := compress(jsonData)
	csvGzipData := compress(csvData)
	tests := []MqttTestCase{
		{
			Name:       "db/append/example_v5",
			Topic:      "db/append/example",
			Payload:    jsonData,
			Ver:        uint(5),
			Properties: map[string]string{"AppendWorkerMaxIdleTimeout": "1s"},
		},
		{
			Name:    "db/append/example",
			Topic:   "db/append/example",
			Payload: jsonData,
			Ver:     uint(4),
		},
		{
			Name:       "db/write/example?method=append",
			Topic:      "db/write/example",
			Payload:    jsonData,
			Ver:        uint(5),
			Properties: map[string]string{"method": "append"},
		},
		{
			Name:    "db/append/example_json_v311",
			Topic:   "db/append/example:json",
			Payload: jsonData,
			Ver:     uint(4),
		},
		{
			Name:    "db/append/example_json_v5",
			Topic:   "db/append/example:json",
			Payload: jsonData,
			Ver:     uint(5),
		},
		{
			Name:    "db/append/example_json_gzip_v311",
			Topic:   "db/append/example:json:gzip",
			Payload: jsonGzipData,
			Ver:     uint(4),
		},
		{
			Name:    "db/append/example_json_gzip_v5",
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
			Name:       "append_csv_partial",
			Topic:      "db/write/example",
			Payload:    csvDataWithHeader,
			Ver:        uint(5),
			Properties: map[string]string{"method": "append", "format": "csv", "header": "columns", "flush": "true"},
		},
		{
			Name:    "db/append/example csv gzip",
			Topic:   "db/append/example:csv: gzip",
			Payload: csvGzipData,
			Ver:     uint(5),
		},
		{
			Name:       "db/write/example?format=ndjson&method=append",
			Topic:      "db/write/example",
			Payload:    ndjsonData,
			Ver:        uint(5),
			Properties: map[string]string{"method": "append", "format": "ndjson", "timeformat": "s", "flush": "true"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			runMqttTest(t, &tt)
			// TODO: how to ensure data is flushed before query?
			// - mqtt works asynchronously
			time.Sleep(1000 * time.Millisecond)

			conn, err := spi.Connect(t.Context(), "sys")
			if err != nil {
				t.Fatalf("Test %q failed, connect error: %s", tt.Name, err.Error())
			}
			defer conn.Close()
			retry := 0
		doRetry:
			_, err = conn.ExecContext(t.Context(), "EXEC table_flush(example)")
			if err != nil {
				t.Fatalf("Test %q failed, table_flush error: %s", tt.Name, err.Error())
			}
			var count int
			var tag = "my-append"
			result := conn.QueryRowContext(t.Context(), "select count(*) from example where name = ?", tag)
			if result.Err() != nil {
				t.Fatalf("Test %q failed, query error: %s", tt.Name, result.Err().Error())
			}
			err = result.Scan(&count)
			if err != nil {
				t.Fatalf("Test %q failed, scan error: %s", tt.Name, err.Error())
			}
			if count != 2 {
				if retry < 10 {
					retry++
					time.Sleep(1000 * time.Millisecond)
					goto doRetry
				}
				t.Logf("Test %q expect 2 rows, got %d", tt.Name, count)
				t.Fail()
			}
			conn.ExecContext(t.Context(), "delete from example where name = ?", tag)
			conn.ExecContext(t.Context(), "EXEC table_flush(example)")
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
	csvData := []byte("my-mqtt-tql,1705291859000000000,1.2345\nmy-mqtt-tql,1705291860000000000,2.3456")

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

				// TODO: how to ensure data is flushed before query?
				// - mqtt works asynchronously
				time.Sleep(1000 * time.Millisecond)

				conn, err := spi.Connect(t.Context(), "sys")
				if err != nil {
					t.Fatalf("Test %q failed, connect error: %s", tt.Name, err.Error())
				}
				defer conn.Close()
				retry := 0
			doRetry:
				_, err = conn.ExecContext(t.Context(), "EXEC table_flush(example)")
				if err != nil {
					t.Fatalf("Test %q failed, table_flush error: %s", tt.Name, err.Error())
				}
				var count int
				var tag = "my-mqtt-tql"
				conn.QueryRowContext(t.Context(), "select count(*) from example where name = ?", tag).Scan(&count)
				if count != 2 {
					if retry < 10 {
						retry++
						time.Sleep(1000 * time.Millisecond)
						goto doRetry
					}
					t.Logf("Test %q expect 2 rows, got %d", tt.Name, count)
					t.Fail()
				}
				_, err = conn.ExecContext(t.Context(), "delete from example where name = ?", tag)
				if err != nil {
					t.Fatalf("Test %q failed, delete error: %s", tt.Name, err.Error())
				}
				_, err = conn.ExecContext(t.Context(), "EXEC table_flush(example)")
				if err != nil {
					t.Fatalf("Test %q failed, table_flush error: %s", tt.Name, err.Error())
				}
			})
		}
	}
}

type mqttTestAuthServer struct {
	token    string
	allow    bool
	allowErr error
}

func (s *mqttTestAuthServer) ValidateClientToken(_ context.Context, token string) (string, bool, error) {
	s.token = token
	return "SYS", s.allow, s.allowErr
}

func (s *mqttTestAuthServer) ValidateClientCertificate(clientId string, certHash string) (bool, error) {
	return false, nil
}

func (s *mqttTestAuthServer) ValidateUserPublicKey(ctx context.Context, user string, publicKey ssh.PublicKey) (bool, error) {
	return false, nil
}

func (s *mqttTestAuthServer) ValidateUserPassword(ctx context.Context, user string, password string) (bool, string, error) {
	return false, "", nil
}

func (s *mqttTestAuthServer) ValidateUserOtp(user string, otp string) (bool, error) {
	return false, nil
}

func (s *mqttTestAuthServer) GenerateOtp(user string) (string, error) {
	return "", nil
}

func (s *mqttTestAuthServer) GenerateSnowflake() string {
	return ""
}

func (s *mqttTestAuthServer) ServerPrivateKeyPath() string {
	return "/path/to/private.key"
}

func TestNewMqttOptions(t *testing.T) {
	var started bool
	var stopped bool
	authSvc := &mqttTestAuthServer{allow: true}

	svr, err := NewMqtt(
		WithMqttAuthServer(authSvc, true),
		WithMqttMaxMessageSizeLimit(4096),
		WithMqttOnStarted(func() { started = true }),
		WithMqttOnStopped(func() { stopped = true }),
	)
	require.NoError(t, err)
	require.NotNil(t, svr)
	require.Equal(t, "db/reply", svr.defaultReplyTopic)
	require.True(t, svr.enableTokenAuth)
	require.Same(t, authSvc, svr.authServer)
	require.EqualValues(t, 4096, svr.broker.Options.Capabilities.MaximumPacketSize)
	require.Same(t, svr, svr.authHook.svr)

	svr.authHook.OnStarted()
	svr.authHook.OnStopped()
	require.True(t, started)
	require.True(t, stopped)

	t.Run("option_error", func(t *testing.T) {
		expected := errors.New("option failure")
		_, err := NewMqtt(func(s *mqttd) error { return expected })
		require.ErrorIs(t, err, expected)
	})
}

func TestMqttACLCheck(t *testing.T) {
	svr := &mqttd{restrictTopics: true}

	tests := []struct {
		name  string
		topic string
		write bool
		allow bool
	}{
		{name: "deny_subscribe_query", topic: "db/query", write: false, allow: false},
		{name: "deny_publish_reply", topic: "db/reply/abc", write: true, allow: false},
		{name: "deny_subscribe_tql", topic: "db/tql/script.tql", write: false, allow: false},
		{name: "deny_root_topic", topic: "db", write: true, allow: false},
		{name: "deny_wildcard_subscribe", topic: "db/#", write: false, allow: false},
		{name: "deny_publish_sys", topic: "$SYS/broker/load", write: true, allow: false},
		{name: "allow_write_query", topic: "db/query", write: true, allow: true},
		{name: "allow_normal_subscribe", topic: "db/reply/custom", write: false, allow: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.allow, svr.onACLCheck(nil, tt.topic, tt.write))
		})
	}
}

func TestAuthHookProvidesAndPacketEncode(t *testing.T) {
	hook := &AuthHook{}
	require.True(t, hook.Provides(mqtt.OnStarted))
	require.True(t, hook.Provides(mqtt.OnStopped))
	require.True(t, hook.Provides(mqtt.OnConnectAuthenticate))
	require.True(t, hook.Provides(mqtt.OnACLCheck))
	require.True(t, hook.Provides(mqtt.OnConnect))
	require.True(t, hook.Provides(mqtt.OnPublished))
	require.True(t, hook.Provides(mqtt.OnDisconnect))
	require.True(t, hook.Provides(mqtt.OnPacketEncode))
	require.False(t, hook.Provides(0))

	puback := packets.Packet{FixedHeader: packets.FixedHeader{Type: packets.Puback}, ReasonCode: 1}
	encoded := hook.OnPacketEncode(nil, puback)
	require.Equal(t, byte(0), encoded.ReasonCode)

	other := packets.Packet{FixedHeader: packets.FixedHeader{Type: packets.Puback}, ReasonCode: 2}
	encoded = hook.OnPacketEncode(nil, other)
	require.Equal(t, byte(2), encoded.ReasonCode)
	encoded = hook.OnPacketEncode(nil, packets.Packet{FixedHeader: packets.FixedHeader{Type: packets.Publish}, ReasonCode: 1})
	require.Equal(t, byte(1), encoded.ReasonCode)
}

func TestAuthHookOnConnectAuthenticate(t *testing.T) {
	client := &mqtt.Client{ID: "client-1"}
	pk := packets.Packet{Connect: packets.ConnectParams{Username: []byte("token-value")}}
	log := logging.GetLog("mqttd-test")

	t.Run("disabled", func(t *testing.T) {
		hook := &AuthHook{svr: &mqttd{log: log, enableTokenAuth: false}}
		require.True(t, hook.OnConnectAuthenticate(client, pk))
	})

	t.Run("missing_auth_server", func(t *testing.T) {
		hook := &AuthHook{svr: &mqttd{log: log, enableTokenAuth: true}}
		require.False(t, hook.OnConnectAuthenticate(client, pk))
	})

	t.Run("validate_true", func(t *testing.T) {
		authSvc := &mqttTestAuthServer{allow: true}
		hook := &AuthHook{svr: &mqttd{log: log, enableTokenAuth: true, authServer: authSvc}}
		require.True(t, hook.OnConnectAuthenticate(client, pk))
		require.Equal(t, "token-value", authSvc.token)
	})

	t.Run("validate_false", func(t *testing.T) {
		hook := &AuthHook{svr: &mqttd{log: log, enableTokenAuth: true, authServer: &mqttTestAuthServer{allow: false}}}
		require.False(t, hook.OnConnectAuthenticate(client, pk))
	})

	t.Run("validate_error", func(t *testing.T) {
		hook := &AuthHook{svr: &mqttd{log: log, enableTokenAuth: true, authServer: &mqttTestAuthServer{allowErr: errors.New("boom")}}}
		require.False(t, hook.OnConnectAuthenticate(client, pk))
	})

	t.Run("cert_verifier_bypassed_on_non_tls_listener", func(t *testing.T) {
		// e.g. the internal unix socket listener must not be rejected when
		// EnableTls is set for the TCP/WS listeners of the same broker.
		unixClient := &mqtt.Client{ID: "client-unix", Net: mqtt.ClientConnection{Listener: "mqtt-unix-0"}}
		hook := &AuthHook{svr: &mqttd{log: log, certVerifier: NewX509CertVerifier(nil)}}
		require.True(t, hook.OnConnectAuthenticate(unixClient, pk))
	})

	t.Run("cert_verifier_rejects_non_tls_conn_on_tls_listener", func(t *testing.T) {
		tlsClient := &mqtt.Client{ID: "client-tls", Net: mqtt.ClientConnection{Listener: "mqtt-tls-0"}}
		hook := &AuthHook{svr: &mqttd{log: log, certVerifier: NewX509CertVerifier(nil)}}
		require.False(t, hook.OnConnectAuthenticate(tlsClient, pk))
	})
}

func TestIsMqttTlsListener(t *testing.T) {
	require.True(t, isMqttTlsListener("mqtt-tls-0"))
	require.True(t, isMqttTlsListener("mqtt-wss-0"))
	require.False(t, isMqttTlsListener("mqtt-tcp-0"))
	require.False(t, isMqttTlsListener("mqtt-ws-0"))
	require.False(t, isMqttTlsListener("mqtt-unix-0"))
	require.False(t, isMqttTlsListener("mqtt2-ws"))
}

func TestLoadTlsConfigErrorsAndTcpHelper(t *testing.T) {
	require.NotPanics(t, func() {
		configureTcpConn(nil)
	})

	_, err := LoadTlsConfig("/path/does/not/exist.crt", "/path/does/not/exist.key", false, false)
	require.Error(t, err)
}
