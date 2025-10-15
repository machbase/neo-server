# Machbase Neo Bridge - NATS

NATS Bridge enables machbase-neo to send and receive message to/from NATS server (https://nats.io).

## Register a bridge to NATS server

Register a bridge

```
bridge add -t nats my_nats server=nats://127.0.0.1:3000 name=client-name;
```

A NATS bridge just defines how machbase-neo can connect to the external MQTT broker, See the subscriber section below to get it to receive messages.

Available connect options. Please refer to the official documents of NATS for the detail.

| Option           | Description                          |
| :-----------     | :---------------------------------   |
| `Server`         | server address, If the broker has redundant access points, use multiple "broker" options |
| `Name`           | Name is an optional name label which will be sent to the server on CONNECT to identify the client. |
| `NoRandomize`    | NoRandomize configures whether we will randomize the server pool. |
| `NoEcho`         | NoEcho configures whether the server will echo back messages that are sent on this connection if we also have matching subscriptions. Note this is supported on servers >= version 1.2. Proto 1 or greater. |
| `Verbose`        | Verbose signals the server to send an OK ack for commands successfully processed by the server. |
| `Pedantic`       | Pedantic signals the server whether it should be doing further validation of subjects. |
| `AllowReconnect` | AllowReconnect enables reconnection logic to be used when we encounter a disconnect from the current server. |
| `MaxReconnect`   | MaxReconnect sets the number of reconnect attempts that will be tried before giving up. |
| `ReconnectWait`  | ReconnectWait sets the time to backoff after attempting a reconnect to a server that we were already connected to previously. |
| `Timeout`        | Timeout sets the timeout for a Dial operation on a connection. |
| `PingInterval`   | PingInterval is the period at which the client will be sending ping commands to the server, disabled if 0 or negative. (ex: `PingInterval=2m`) |
| `User`           | User sets the username to be used when connecting to the server. |
| `Password`       | Password sets the password to be used when connecting to a server. |
| `Token`          | Token sets the token to be used when connecting to a server. |
| `RetryOnFailedConnect` | sets the connection in reconnecting state right away if it can't connect to a server in the initial set. |
| `SkipHostLookup` | SkipHostLookup skips the DNS lookup for the server hostname. (ex: `SkipHostLookup=true`) |

## Receive messages - Subscriber

Let's make an example that receives messages from NATS server and storing the data into database utilizing bridge and subscriber.

### 1. Run NATS server

If you need to install NATS server, please refere https://nats.io. The installation in standalone mode is straightforward.

```sh
$ nats-server
[61052] 2021/10/28 16:53:38.003205 [INF] Starting nats-server
[61052] 2021/10/28 16:53:38.003329 [INF]   Version:  2.6.1
[61052] 2021/10/28 16:53:38.003333 [INF]   Git:      [not set]
[61052] 2021/10/28 16:53:38.003339 [INF]   Name:     NDUP6JO4T5LRUEXZUHWXMJYMG4IZAJDNWETTA4GPJ7DKXLJUXBN3UP3M
[61052] 2021/10/28 16:53:38.003342 [INF]   ID:       NDUP6JO4T5LRUEXZUHWXMJYMG4IZAJDNWETTA4GPJ7DKXLJUXBN3UP3M
[61052] 2021/10/28 16:53:38.004046 [INF] Listening for client connections on 0.0.0.0:4222
[61052] 2021/10/28 16:53:38.004683 [INF] Server is ready
...
```

### 2. Register a bridge for the NATS

Open machbase-neo shell, execute `bridge add...` command.

```
bridge add -t nats my_nats server=nats://127.0.0.1:4222 name=demo;
```

It defines the way how machbase-neo can connect to the NATS server.

```
┌──────────┬──────────┬──────────────────────────────────────────┐
│ NAME     │ TYPE     │ CONNECTION                               │
├──────────┼──────────┼──────────────────────────────────────────┤
│ my_nats  │ nats     │ server=nats://127.0.0.1:4222 name=demo   │
└──────────┴──────────┴──────────────────────────────────────────┘
```

### 3-A. Subscriber with writing descriptor

Open machbase-neo shell to add a new subscriber which makes a pipeline between the bridge and database table.

```
subscriber add --autostart nats_subr my_nats iot.sensor db/append/EXAMPLE:csv;
```

Execute `subscriber list` to confirm.

```
┌───────────┬─────────┬────────────┬───────────────────────┬───────────┬─────────┐
│ NAME      │ BRIDGE  │ TOPIC      │ DESTINATION           │ AUTOSTART │ STATE   │
├───────────┼─────────┼────────────┼───────────────────────┼───────────┼─────────┤
│ NATS_SUBR │ my_nats │ iot.sensor │ db/append/EXAMPLE:csv │ true      │ RUNNING │
└───────────┴─────────┴────────────┴───────────────────────┴───────────┴─────────┘
```

It specifies...
- `--autostart` makes the subscriber starts along with machbase-neo starts. Omit this to start/stop manually.
- `nats_subr` the name of the subscriber.
- `my_nats` the name of the bridge that the subscriber is going to use.
- `iot.sensor` subject name to subscribe. it should be in NATS subject syntax.
- `db/append/EXAMPLE:csv` writing descriptor, it means the incoming data is in CSV format and writing data into the table `EXAMPLE` in *append* mode.

The place of writing description can be replaced with a file path of *TQL* script. We will see an example later.

The syntax of writing descriptor is ...

```
db/{method}/{table_name}:{format}:{compress}?{options}
```

**method**

There are two methods `append` and `write`. The `append` is recommended on the stream environment like NATS.

- `append` writing data in append mode
- `write` writing data with INSERT sql statement

**table_name**

Specify the destination table name, case insensitive.

**format**

- `json` (default)
- `csv`

**compress**

Currently `gzip` is supported, If `:{compress}` part is omitted, it means the data is not compressed.

**options**

The writing description can contain an optional question-mark-separated URL-encoded parameters.

| Name          | Default      | Description                                                    |
| :------------ | :----------- | :------------------------------------------------------------- |
| `timeformat`  | `ns`         | Time format: s, ms, us, ns                                     |
| `tz`          | `UTC`        | Time Zone: UTC, Local and location spec                        |
| `delimiter`   | `,`          | CSV delimiter, ignored if content is not csv                   |
| `heading`     | `false`      | If CSV contains header line, set `true` to skip the first line |

Please refer to the subscriber's pending message limit on [nats.io](https://docs.nats.io/running-a-nats-service/nats_admin/slow_consumers#client-configuration)

Examples)

- `db/append/EXAMPLE:csv?timeformat=s&heading=true`
- `db/write/EXAMPLE:csv:gzip?timeformat=s`
- `db/append/EXAMPLE:json?timeformat=2&pendingMsgLimit=1048576`

#### NATS client application

Let's make a simple Go application which sends mulitple records of data in CSV to the subject `iot.sensor` on the NATS server.

```go
package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/nats-io/nats.go"
)

func main() {
    // connect to the NATS server
	opts := nats.GetDefaultOptions()
	opts.Servers = []string{"nats://127.0.0.1:4222"}
	conn, err := opts.Connect()
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	tick := time.Now()

    // make CSV data
    lines := []string{}
	for i := 0; i < 10; i++ {
        // NAME,TIME,VALUE
		line := fmt.Sprintf("hello-nats,%d,3.1415", tick.Add(time.Duration(i)).UnixNano())
		lines = append(lines, line)
	}
	reqData := []byte(strings.Join(lines, "\n"))

	// A) request-respond model
	if rsp, err := conn.Request("iot.sensor", reqData, 100*time.Millisecond); err != nil {
		panic(err)
	} else {
		fmt.Println("RESP:", string(rsp.Data))
	}
	// B) fire-and-forget model
	// if err := conn.Publish("iot.sensor", reqData); err != nil {
	// 	panic(err)
	// }
}
```

When you run this program, it will send 10 lines of CSV data to the `iot.sensor` subject on the NATS server, the subscriber `nats_subr` receives the data and writes it into the table `EXAMPLE`.

```sh
$ go run nats_pub.go ↵
RESP: {"success":true,"reason":"10 records appended","elapse":"2.186209ms"}
```

### 3-B. Subscriber with TQL

#### Data writing TQL script

Let's make a tql script that receives CSV data and writing it to the table `example` and save the file as `test.tql`.

```js
CSV(payload())
MAPVALUE(1, parseTime(value(1), "ns"))
MAPVALUE(2, parseFloat(value(2)))
APPEND( table("example") )
```

Open machbase-neo shell to add a new subscriber which makes a pipeline between the bridge and TQL script.

```
subscriber add --autostart nats_subr my_nats iot.sensor /test.tql;
```

It specifies...
- `--autostart` makes the subscriber starts along with machbase-neo starts.
- `nats_subr` the name of the subscriber
- `my_nats` the name of the bridge that the subscriber is going to use
- `iot.sensor` subject name to subscribe. it supports NATS subject syntax.
- `/test.tql` the tql file path which will receive the incoming data.

Execute `subscriber list` to confirm.

```
┌───────────┬─────────┬────────────┬─────────────┬───────────┬─────────┐
│ NAME      │ BRIDGE  │ TOPIC      │ DESTINATION │ AUTOSTART │ STATE   │
├───────────┼─────────┼────────────┼─────────────┼───────────┼─────────┤
│ NATS_SUBR │ my_nats │ test.topic │ /test.tql   │ true      │ RUNNING │
└───────────┴─────────┴────────────┴─────────────┴───────────┴─────────┘
```

#### NATS client application

Then run the same NATS client application as above example code.

The source code of the above example can be found in [Github](https://github.com/machbase/neo-server/tree/main/examples/go/nats_pub).