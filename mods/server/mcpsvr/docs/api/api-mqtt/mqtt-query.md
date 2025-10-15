# Machbase Neo MQTT Query

The database query topic for MQTT is `db/query`. Send a query request to this topic, and the server will respond with the result to the `db/reply` topic or the topic specified in the `reply` field of the request.

## Query JSON

| param       | default | description                   |
|:----------- |---------|:----------------------------- |
| **q**       | _n/a_   | SQL query string              |
| reply       | db/reply| The topic where to receive the result of query |
| format      | json    | Result data format: json, csv, box |
| timeformat  | ns      | Time format: s, ms, us, ns    |
| tz          | UTC     | Time Zone: UTC, Local and location spec |
| compress    | _no compression_   | compression method: gzip      |
| rownum      | false   | including rownum: true, false |
| heading     | true    | showing heading: true, false  |
| precision   | -1      | precision of float value, -1 for no round, 0 for int |s

**More Parameters in `format=json`**

Those options are available only when `format=json`

| param       | default | description                   |
|:----------- |---------|:----------------------------- |
| transpose   | false   | produce cols array instead of rows. |
| rowsFlatten | false   | reduce the array dimension of the *rows* field in the JSON object. |
| rowsArray   | false   | produce JSON that contains only array of object for each record.  |

A basic query example shows the client subscribe to `db/reply/#` and publish a query request to `db/query` with *reply* field `db/reply/my_query` so that it can identify the individual reply from multiple messages.

```json
{
    "q": "select name,time,value from example limit 5",
    "format": "csv",
    "reply": "db/reply/my_query"
}
```

## Client Examples

### JSH app

In this example, you will learn how to subscribe to a reply topic,
send an SQL query request, and receive the result over MQTT.

1. **Subscribe to the Reply Topic**  
   The client first subscribes to a specific reply topic, such as `db/reply/my_query`.
   This topic is where the server will send the query result.

2. **Publish the SQL Query Request**  
   The client then publishes a message to the `db/query` topic.
   The message includes the SQL query (`q`),
   the desired result format (`format`),
   and the reply topic (`reply`) where the result should be sent.

3. **Receive and Process the Response**  
   When the server processes the query,
   it sends the result to the specified reply topic.
   The client receives this message and prints the result.

Below is the complete code example:

```js
const process = require("@jsh/process");
const mqtt = require("@jsh/mqtt");

const topicReply = "db/reply/my_query";
const topicQuery = "db/query";
try {
    var conf = { serverUrls: ["tcp://127.0.0.1:5653"] };
    var client = new mqtt.Client(conf);
    client.onConnect = () => {
        client.subscribe({subscriptions:[{topic:topicReply, qos: 1}]})
    }
    var received = false
    client.onMessage = (msg) => {
        console.log('---- reply ----')
        console.log(msg.payload.string());
        received = true
    }

    client.connect( {timeout: 1000} );
    client.publish({topic:topicQuery, qos: 1}, JSON.stringify({
        q: `select name,time,value from example limit 5`,
        format: 'csv',
        reply: topicReply,
    }))
    do {
        process.sleep(100);
    } while(!received)
    client.unsubscribe({topics:[topicReply]})
    client.disconnect({timeout:1000});
} catch (e) {
    console.error("Error:", e.message);
}
```

### Node.js Client

```sh
npm install mqtt --save
```

```js
const mqtt = require("mqtt");

const client = mqtt.connect("mqtt://127.0.0.1:5653", {
    clean: true,
    connectTimeout: 3000,
    autoUseTopicAlias: true,
    protocolVersion: 5,
});

const sqlText = "SELECT time,value FROM example "+
    "where name = 'neo_cpu.percent' limit 3";

client.on("connect", () => {
    client.subscribe("db/reply/#", (err) => {
        if (!err) {
            const req = {
                q: sqlText,
                format: "box",
                precision: 1,
                timeformat: "15:04:05",
            };
            client.publish("db/query", JSON.stringify(req));
        }
    });
});

client.on("message", (topic, message) => {
    console.log(message.toString());
    client.end();
});
```

```sh
$ node main.js

+----------+-------+
| TIME     | VALUE |
+----------+-------+
| 05:46:19 | 69.4  |
| 05:46:22 | 26.4  |
| 05:46:25 | 42.8  |
+----------+-------+
```

### Go client

**Define data structure for response**

```go
type Result struct {
	Success bool       `json:"success"`
	Reason  string     `json:"reason"`
	Elapse  string     `json:"elapse"`
	Data    ResultData `json:"data"`
}

type ResultData struct {
	Columns []string `json:"columns"`
	Types   []string `json:"types"`
	Rows    [][]any  `json:"rows"`
}
```

**Subscribe 'db/reply'**

```go
client.Subscribe("db/reply", 1, func(_ paho.Client, msg paho.Message) {
    buff := msg.Payload()
    result := Result{}
    if err := json.Unmarshal(buff, &result); err != nil {
        panic(err)
    }
    if !result.Success {
        fmt.Println("RECV: query failed:", result.Reason)
        return
    }
    if len(result.Data.Rows) == 0 {
        fmt.Println("Empty result")
        return
    }
    for i, rec := range result.Data.Rows {
        // do something for each record
        name := rec[0].(string)
        ts := time.Unix(0, int64(rec[1].(float64)))
        value := float64(rec[2].(float64))
        fmt.Println(i+1, name, ts, value)
    }
})
```

**Publish 'db/query'**

```go
jsonStr := `{ "q": "select * from EXAMPLE order by time desc limit 5" }`
client.Publish("db/query", 1, false, []byte(jsonStr))
```