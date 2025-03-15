package test

import (
	"fmt"
	"sync"
	"testing"
	"time"

	paho "github.com/eclipse/paho.mqtt.golang"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func newMqttClient(t *testing.T) paho.Client {
	t.Helper()

	cfg := paho.NewClientOptions()
	cfg.SetCleanSession(true)
	cfg.SetConnectRetry(false)
	cfg.SetAutoReconnect(false)
	cfg.SetProtocolVersion(4)
	cfg.SetClientID("machbase-test-cli")
	cfg.AddBroker("127.0.0.1:5653")
	cfg.SetKeepAlive(6 * time.Second)
	cfg.SetUsername("user")
	cfg.SetPassword("pass")

	//// connect mqtt server
	client := paho.NewClient(cfg)
	require.NotNil(t, client)

	//// connect mqtt server
	result := client.Connect()
	ok := result.WaitTimeout(time.Second)
	if result.Error() != nil {
		t.Logf("CONNECT: %s", result.Error())
	}
	require.True(t, ok)
	require.NoError(t, result.Error())
	return client
}

var mqttTestWg sync.WaitGroup
var mqttTestFunc func(reply string) (string, bool)

func mqttTestSubscriber(t *testing.T) func(_ paho.Client, msg paho.Message) {
	return func(_ paho.Client, msg paho.Message) {
		defer func() {
			mqttTestFunc = nil
			mqttTestWg.Done()
		}()
		buff := msg.Payload()
		if mqttTestFunc != nil {
			if msg, pass := mqttTestFunc(string(buff)); !pass {
				t.Log("RECV:", string(buff))
				t.Log(msg)
				t.Fail()
			}
		}
	}
}

func TestMqttWithSysUser(t *testing.T) {
	t.Skip("skip mqtt test, because it is timed out on CI")
	client := newMqttClient(t)

	//// subscribe to reply topic
	client.Subscribe("db/reply", 1, mqttTestSubscriber(t))

	//// create table
	jsonStr := `{
			"q": "create tag table if not exists sample (name varchar(200) primary key, time datetime basetime, value double summarized, jsondata json)"
		}`
	mqttTestFunc = func(reply string) (string, bool) {
		vSuccess := gjson.Get(reply, "success")
		vReason := gjson.Get(reply, "reason")
		require.True(t, vSuccess.Bool(), fmt.Sprintf("RECV: %v", reply))
		require.Equal(t, "Created successfully.", vReason.String())
		return "table creation test failed", true
	}
	mqttTestWg.Add(1)
	client.Publish("db/query", 1, false, []byte(jsonStr))
	mqttTestWg.Wait()

	//// check table exists
	jsonStr = `{ "q": "select count(*) from M$SYS_TABLES where name = 'SAMPLE'" }`
	mqttTestFunc = func(reply string) (string, bool) {
		vSuccess := gjson.Get(reply, "success")
		vReason := gjson.Get(reply, "reason")
		require.True(t, vSuccess.Bool())
		require.Equal(t, "success", vReason.String(), fmt.Sprintf("RECV: %v", reply))
		vCount := gjson.Get(reply, "data.rows.0.0")
		return "table existence test failed", vCount.Int() == 1
	}
	mqttTestWg.Add(1)
	client.Publish("db/query", 1, false, []byte(jsonStr))
	mqttTestWg.Wait()

	//// append
	jsonStr = `[
			[ "sample.tag", 1670380344000000000, 1.0001, "{\"name\":\"Abc\"}" ],
			[ "sample.tag", 1670380345000000000, 2.0002, "{\"name\":\"Def\"}" ]
	]`
	mqttTestFunc = func(reply string) (string, bool) {
		vSuccess := gjson.Get(reply, "success")
		vReason := gjson.Get(reply, "reason")
		require.True(t, vSuccess.Bool(), fmt.Sprintf("RECV: %v", reply))
		require.Equal(t, "success", vReason.String())
		return "append test failed", true
	}
	// mqtt append does not have reply
	token := client.Publish("db/append/sample", 1, false, []byte(jsonStr))
	token.Wait()

	//// insert
	jsonStr = `{
		"data": {
			"columns":["name", "time", "value"],
			"rows": [
				[ "sample.tag", 1670380342000000000, 1.0001 ],
				[ "sample.tag", 1670380343000000000, 2.0002 ]
			]
		}
	}`
	mqttTestFunc = func(reply string) (string, bool) {
		vSuccess := gjson.Get(reply, "success")
		vReason := gjson.Get(reply, "reason")
		require.True(t, vSuccess.Bool(), fmt.Sprintf("RECV: %v", reply))
		require.Equal(t, "success", vReason.String())
		return "insert test failed", true
	}
	token = client.Publish("db/write/sample", 1, false, []byte(jsonStr))
	token.Wait()

	//// insert with influx lineprotocol
	// linestr := `sample.tag name="guage",value=3.003 1670380345000000`
	mqttTestFunc = func(reply string) (string, bool) {
		vSuccess := gjson.Get(reply, "success")
		vReason := gjson.Get(reply, "reason")
		require.True(t, vSuccess.Bool(), fmt.Sprintf("RECV: %v", reply))
		require.Equal(t, "success", vReason.String())
		return "lineprotocol test failed", true
	}
	token = client.Publish("metrics/sample", 1, false, []byte(lineProtocolData))
	token.Wait()

	//// wait until receive all replied messages from server
	client.Disconnect(100)

	// reconnect
	client = newMqttClient(t)
	require.NotNil(t, client)
	//// subscribe to reply topic
	client.Subscribe("db/reply", 1, mqttTestSubscriber(t))

	//// select
	mqttTestFunc = func(reply string) (string, bool) {
		vSuccess := gjson.Get(reply, "success")
		vReason := gjson.Get(reply, "reason")
		require.True(t, vSuccess.Bool(), fmt.Sprintf("RECV: %v", reply))
		require.Equal(t, "success", vReason.String())
		vRows := gjson.Get(reply, "data.rows")
		require.Equal(t, 4, len(vRows.Array()), fmt.Sprintf("RECV: %v", reply))

		require.Equal(t, "sample.tag", gjson.Get(reply, "data.rows.0.0").String())
		require.Equal(t, int64(1670380342000000000), gjson.Get(reply, "data.rows.0.1").Int())
		require.Equal(t, 1.0001, gjson.Get(reply, "data.rows.0.2").Float())

		require.Equal(t, "sample.tag", gjson.Get(reply, "data.rows.1.0").String())
		require.Equal(t, int64(1670380343000000000), gjson.Get(reply, "data.rows.1.1").Int())
		require.Equal(t, 2.0002, gjson.Get(reply, "data.rows.1.2").Float())

		require.Equal(t, "sample.tag", gjson.Get(reply, "data.rows.2.0").String())
		require.Equal(t, int64(1670380344000000000), gjson.Get(reply, "data.rows.2.1").Int())

		require.Equal(t, "sample.tag", gjson.Get(reply, "data.rows.3.0").String())
		require.Equal(t, int64(1670380345000000000), gjson.Get(reply, "data.rows.3.1").Int())

		return "select test failed", true
	}
	mqttTestWg.Add(1)
	client.Publish("db/query", 1, false, []byte(`{"q":"select name, time, value from sample where name = 'sample.tag'"}`))
	mqttTestWg.Wait()

	//// drop table
	mqttTestFunc = func(reply string) (string, bool) {
		vSuccess := gjson.Get(reply, "success")
		vReason := gjson.Get(reply, "reason")
		require.True(t, vSuccess.Bool(), fmt.Sprintf("RECV: %v", reply))
		require.Equal(t, "Dropped successfully.", vReason.String())
		return "table drop test failed", true
	}
	jsonStr = `{ "q": "drop table sample" }`
	mqttTestWg.Add(1)
	client.Publish("db/query", 1, false, []byte(jsonStr))
	mqttTestWg.Wait()

	//// wait until receive all replied messages from server
	client.Disconnect(100)
	time.Sleep(time.Second * 1)
}

func TestMqttWithDemoUser(t *testing.T) {
	t.Skip("skip mqtt test, because it is timed out on CI")
	var jsonStr string

	client := newMqttClient(t)

	//// subscribe to reply topic
	client.Subscribe("db/reply", 1, mqttTestSubscriber(t))

	//// create user
	jsonStr = `{ "q": "CREATE USER demo IDENTIFIED BY demo" }`
	mqttTestFunc = func(reply string) (string, bool) {
		vReason := gjson.Get(reply, "reason")
		if vReason.String() == "Created successfully." ||
			vReason.String() == "MACH-ERR 2082 User (DEMO) already exists." {
			// accept as success
			return "create user ok", true
		} else {
			t.Fail()
			return "create user failed", false
		}
	}
	mqttTestWg.Add(1)
	client.Publish("db/query", 1, false, []byte(jsonStr))
	mqttTestWg.Wait()

	//// create table DEMO.sample
	jsonStr = `{
			"q": "create tag table if not exists DEMO.sample (name varchar(200) primary key, time datetime basetime, value double summarized, json json)"
		}`
	mqttTestFunc = func(reply string) (string, bool) {
		vSuccess := gjson.Get(reply, "success")
		vReason := gjson.Get(reply, "reason")
		require.True(t, vSuccess.Bool(), fmt.Sprintf("RECV: %v", reply))
		require.Equal(t, "Created successfully.", vReason.String())
		return "table creation test failed", true
	}
	mqttTestWg.Add(1)
	client.Publish("db/query", 1, false, []byte(jsonStr))
	mqttTestWg.Wait()

	//// append DEMO.sample
	jsonStr = `[
			[ "sample.tag", 1670380344000000000, 1.0001, "{\"name\":\"Abc\"}" ],
			[ "sample.tag", 1670380345000000000, 2.0002, "{\"name\":\"Def\"}" ]
		]`
	mqttTestFunc = func(reply string) (string, bool) {
		vSuccess := gjson.Get(reply, "success")
		vReason := gjson.Get(reply, "reason")
		require.True(t, vSuccess.Bool(), fmt.Sprintf("RECV: %v", reply))
		require.Equal(t, "success", vReason.String())
		return "append test failed", true
	}
	// mqtt append does not have reply
	token := client.Publish("db/append/DEMO.sample", 1, false, []byte(jsonStr))
	token.Wait()

	//// insert
	jsonStr = `{
				"reply": "db/reply",
				"data": {
					"columns":["name", "time", "value"],
					"rows": [
						[ "sample.tag", 1670380342000000000, 1.0001 ],
						[ "sample.tag", 1670380343000000000, 2.0002 ]
					]
				}
			}`
	mqttTestFunc = func(reply string) (string, bool) {
		vSuccess := gjson.Get(reply, "success")
		vReason := gjson.Get(reply, "reason")
		require.True(t, vSuccess.Bool(), fmt.Sprintf("RECV: %v", reply))
		require.Equal(t, "success, 2 record(s) inserted", vReason.String())
		return "insert test failed", true
	}
	mqttTestWg.Add(1)
	client.Publish("db/write/demo.sample", 1, false, []byte(jsonStr))
	mqttTestWg.Wait()

	//// wait until receive all replied messages from server
	client.Disconnect(100)

	flushTable("DEMO.sample")

	// reconnect
	client = newMqttClient(t)
	require.NotNil(t, client)
	//// subscribe to reply topic
	client.Subscribe("db/reply", 1, mqttTestSubscriber(t))

	//// select
	mqttTestFunc = func(reply string) (string, bool) {
		vSuccess := gjson.Get(reply, "success")
		vReason := gjson.Get(reply, "reason")
		require.True(t, vSuccess.Bool(), fmt.Sprintf("RECV: %v", reply))
		require.Equal(t, "success", vReason.String())
		vRows := gjson.Get(reply, "data.rows")
		require.Equal(t, 4, len(vRows.Array()), fmt.Sprintf("RECV: %v", reply))

		require.Equal(t, "sample.tag", gjson.Get(reply, "data.rows.0.0").String())
		require.Equal(t, int64(1670380342000000000), gjson.Get(reply, "data.rows.0.1").Int())
		require.Equal(t, 1.0001, gjson.Get(reply, "data.rows.0.2").Float())

		require.Equal(t, "sample.tag", gjson.Get(reply, "data.rows.1.0").String())
		require.Equal(t, int64(1670380343000000000), gjson.Get(reply, "data.rows.1.1").Int())
		require.Equal(t, 2.0002, gjson.Get(reply, "data.rows.1.2").Float())

		require.Equal(t, "sample.tag", gjson.Get(reply, "data.rows.2.0").String())
		require.Equal(t, int64(1670380344000000000), gjson.Get(reply, "data.rows.2.1").Int())

		require.Equal(t, "sample.tag", gjson.Get(reply, "data.rows.3.0").String())
		require.Equal(t, int64(1670380345000000000), gjson.Get(reply, "data.rows.3.1").Int())

		return "select test failed", true
	}
	mqttTestWg.Add(1)
	client.Publish("db/query", 1, false, []byte(`{"q":"select name, time, value from demo.sample where name = 'sample.tag'"}`))
	mqttTestWg.Wait()

	//// drop table DEMO.sample
	jsonStr = `{
			"q": "drop table DEMO.sample"
		}`
	mqttTestFunc = func(reply string) (string, bool) {
		vSuccess := gjson.Get(reply, "success")
		vReason := gjson.Get(reply, "reason")
		require.True(t, vSuccess.Bool(), fmt.Sprintf("RECV: %v", reply))
		require.Equal(t, "Dropped successfully.", vReason.String())
		return "drop user table failed", true
	}
	mqttTestWg.Add(1)
	client.Publish("db/query", 1, false, []byte(jsonStr))
	mqttTestWg.Wait()

	//// drop user DEMO
	jsonStr = `{ "q": "DROP USER demo" }`
	mqttTestFunc = func(reply string) (string, bool) {
		vSuccess := gjson.Get(reply, "success")
		vReason := gjson.Get(reply, "reason")
		require.True(t, vSuccess.Bool(), fmt.Sprintf("RECV: %v", reply))
		require.Equal(t, "Dropped successfully.", vReason.String())
		return "drop user failed", true
	}
	mqttTestWg.Add(1)
	client.Publish("db/query", 1, false, []byte(jsonStr))
	mqttTestWg.Wait()

	client.Disconnect(100)
	time.Sleep(time.Second * 1)
}
