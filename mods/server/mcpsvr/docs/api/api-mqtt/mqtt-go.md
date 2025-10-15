# Machbase Neo MQTT Go Client

## Setup

### Import paho mqtt for Go

```go
import paho "github.com/eclipse/paho.mqtt.golang"
```

### Create project directory

```sh
mkdir mqtt_client && cd mqtt_client
```

## Publisher

### Client

```go
	opts := paho.NewClientOptions()
	opts.SetCleanSession(true)
	opts.SetConnectRetry(false)
	opts.SetAutoReconnect(false)
	opts.SetProtocolVersion(4)
	opts.SetClientID("machbase-mqtt-cli")
	opts.AddBroker("127.0.0.1:5653")
	opts.SetKeepAlive(30 * time.Second)

	client := paho.NewClient(opts)
```

### Connect (non-TLS)

Connect to machbase-neo via MQTT plain socket.

```go
	connectToken := client.Connect()
	connectToken.WaitTimeout(1 * time.Second)
	if connectToken.Error() != nil {
		panic(connectToken.Error())
	}
```

### Disconnect

```go
client.Disconnect(100)
```

### Publish

```go
	client.Publish("db/append/TAGDATA", 1, false, []byte(jsonStr))
```

## Full source code

```go
package main

import (
	"fmt"
	"sync"
	"time"

	paho "github.com/eclipse/paho.mqtt.golang"
)

func main() {
	wg := sync.WaitGroup{}

	// paho mqtt client options
	opts := paho.NewClientOptions()
	opts.SetCleanSession(true)
	opts.SetConnectRetry(false)
	opts.SetAutoReconnect(false)
	opts.SetProtocolVersion(4)
	opts.SetClientID("machbase-mqtt-cli")
	opts.AddBroker("127.0.0.1:5653")
	opts.SetKeepAlive(30 * time.Second)

	// connect to server with paho mqtt client
	client := paho.NewClient(opts)
	connectToken := client.Connect()
	connectToken.WaitTimeout(1 * time.Second)
	if connectToken.Error() != nil {
		panic(connectToken.Error())
	}

	client.Subscribe("db/reply/#", 1, func(_ paho.Client, msg paho.Message) {
		defer wg.Done()

		buff := msg.Payload()
		str := string(buff)
		fmt.Println("RECV", msg.Topic(), " :", str)
	})

	// check table existence
	jsonStr := `{ "q": "select count(*) from M$SYS_TABLES where name = 'TAGDATA'" }`
	wg.Add(1)
	client.Publish("db/query", 1, false, []byte(jsonStr))
	wg.Wait()

	// create table
	jsonStr = `{
			"q": "create tag table if not exists TAGDATA (name varchar(200) primary key, time datetime basetime, value double summarized, jstr varchar(80))"
		}`
	wg.Add(1)
	client.Publish("db/query", 1, false, []byte(jsonStr))
	wg.Wait()

	// insert
	jsonStr = `{
			"reply": "db/reply",
			"data": {
				"columns":["name", "time", "value"],
				"rows": [
					[ "my-car", 1670380342000000000, 32.1 ],
					[ "my-car", 1670380343000000000, 65.4 ],
					[ "my-car", 1670380344000000000, 76.5 ]
				]
			}
		}`
	// it is also possible to specify table in the topic like `db/write/TAGDATA`,
	// if both (topic and payload) ways used in a time, table name of payload will be taken by server
	wg.Add(1)
	client.Publish("db/write/TAGDATA", 1, false, []byte(jsonStr))
	wg.Wait()

	// append
	for i := 0; i < 100; i++ {
		// both forms are available
		// 1) append a single record: `[ columns... ]`
		// 2) append multiple records: `[ [columns...], [columns...] ]`
		jsonStr = fmt.Sprintf(`[ "my-car", %d, %.1f, "{\"speed\":\"%.1fkmh\",\"lat\":37.38906,\"lon\":127.12182}" ]`,
			time.Now().UnixNano(),
			float32(80+i),
			float32(80+i))
		client.Publish("db/append/TAGDATA", 1, false, []byte(jsonStr))
	}

	// select
	jsonStr = `{ "q":"select count(*) from TAGDATA" }`
	wg.Add(1)
	client.Publish("db/query", 1, false, []byte(jsonStr))
	wg.Wait()

	client.Unsubscribe("db/reply/#")
	// disconnect mqtt connection
	client.Disconnect(100)
}
```