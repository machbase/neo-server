package test

import (
	"sync"
	"testing"
	"time"

	paho "github.com/eclipse/paho.mqtt.golang"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestMqttClient(t *testing.T) {
	cfg := paho.NewClientOptions()
	cfg.SetCleanSession(true)
	cfg.SetConnectRetry(false)
	cfg.SetAutoReconnect(false)
	cfg.SetProtocolVersion(4)
	cfg.SetClientID("machbase-test-cli")
	cfg.AddBroker("127.0.0.1:4083")
	cfg.SetKeepAlive(3 * time.Second)
	cfg.SetUsername("user")
	cfg.SetPassword("pass")

	//// connect mqtt server
	client := paho.NewClient(cfg)
	require.NotNil(t, client)

	result := client.Connect()
	ok := result.WaitTimeout(time.Second)
	if result.Error() != nil {
		t.Logf("CONNECT: %s", result.Error())
	}
	require.True(t, ok)
	require.Nil(t, result.Error())

	tableExistsQuery := false
	tableExists := false

	//// subscribe to reply topic
	wg := sync.WaitGroup{}
	client.Subscribe("db/reply", 1, func(_ paho.Client, msg paho.Message) {
		buff := msg.Payload()
		str := string(buff)
		t.Logf("RECV: %v", str)
		vSuccess := gjson.Get(str, "success")
		require.True(t, vSuccess.Bool())

		if tableExistsQuery {
			vCount := gjson.Get(str, "data.rows.0.0")
			tableExists = vCount.Int() > 0
		}
		wg.Done()
	})

	//// check table exists
	jsonStr := `{ "q": "select count(*) from M$SYS_TABLES where name = 'SAMPLE'" }`
	wg.Add(1)
	tableExistsQuery = true
	client.Publish("db/query", 1, false, []byte(jsonStr))
	wg.Wait()

	tableExistsQuery = false
	//// drop table
	if tableExists {
		jsonStr = `{ "q": "drop table sample" }`
		wg.Add(1)
		client.Publish("db/query", 1, false, []byte(jsonStr))
		wg.Wait()
	}

	//// create table
	jsonStr = `{
		"q": "create tag table sample (name varchar(200) primary key, time datetime basetime, value double summarized, jsondata json)"
	}`
	wg.Add(1)
	client.Publish("db/query", 1, false, []byte(jsonStr))
	wg.Wait()

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
	wg.Add(1)
	client.Publish("db/write/sample", 1, false, []byte(jsonStr))
	wg.Wait()

	//// insert with influx lineprotocol
	// lineprotocol doesn't require reply message
	// linestr := `sample.tag name="guage",value=3.003 1670380345000000`
	client.Publish("metrics/sample", 1, false, []byte(lineProtocolData))

	//// select
	wg.Add(1)
	client.Publish("db/query", 1, false, []byte(`{"q":"select * from sample"}`))
	wg.Wait()

	//// wait until receive all replied messages from server
	client.Disconnect(100)
	time.Sleep(time.Second * 1)
}
