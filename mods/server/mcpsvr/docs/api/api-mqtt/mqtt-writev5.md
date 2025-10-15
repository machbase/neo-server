# Machbase Neo MQTT v5 Write Guide

## Topic MQTT v5

In MQTT v5, user properties are custom key-value pairs that can be attached to each message. They offer greater flexibility compared to previous versions(MQTT v3.1/v3.1.1) and allow for additional metadata to be included with the message.

**Note: It is also possible to use MQTT v3.1 topic syntax with MQTT v5 without using custom properties.**
0
When using MQTT v5, the topic syntax can be simply `db/write/{table}`, and the following properties are supported:

| User Property  | Default  | Values                  |
|:---------------|:--------:|:------------------------|
| format         | `json`   | `csv`, `json`, `ndjson` |
| timeformat     | `ns`     | Time format: `s`, `ms`, `us`, `ns` |
| tz             | `UTC`    | Time Zone: `UTC`, `Local` and location spec |
| compress       |          | `gzip`                  |
| method         | `insert` | `insert`, `append`      |
| reply          |          | Topic to which the server sends the result messages |


**Additional properties for format=csv** 

| User Property  | Default  | Values                  |
|:---------------|:--------:|:------------------------|
| delimiter      |`,`       |                         |
| header         |          | `skip`, `columns`       |


**According to the semantics of append method, `header=columns` does not work with `method=append`.**

## APPEND method

Since MQTT is a connection-oriented protocol, a client program can continuously send data while maintaining the same MQTT session. 
This is the real benefit of using MQTT over HTTP for writing data.

In this example, we use `mosquitto_pub` just for demonstration.
Since it makes a connection to MQTT server and close when it finishes to publish a single message.
You will barely see performance gains against HTTP `write` api or some cases it may be worse.
Use this MQTT method only when a client can keep a connection relatively long time and send multiple messages.

### JSON

**PUBLISH multiple records**

The payload format in the example below is an array of tuples (an array of arrays in JSON).
It appends multiple records to the table through a single MQTT message.
It is also possible to publish a single tuple, as shown below. Machbase Neo accepts both types of payloads via MQTT.

- mqtt-data.json

```json
[
    [ "my-car", 1670380342000000000, 32.1 ],
    [ "my-car", 1670380343000000000, 65.4 ],
    [ "my-car", 1670380344000000000, 76.5 ]
]
```

- mosquitto_pub

```sh
mosquitto_pub -h 127.0.0.1 -p 5653 \
    -t db/write/EXAMPLE \
    -V 5 \
    -D PUBLISH user-property method append \
    -f ./mqtt-data.json
```

- JSH app

The following JSH app demonstrates how to publish multiple records to a Machbase Neo table using MQTT in JavaScript.
This code runs as a standalone JSH script and SCRIPT() function in a TQL script.

This example shows how to efficiently send multiple records to a Machbase Neo table using MQTT and JSH application.
The use of the append method and array payload allows for high-throughput data ingestion,
which is ideal for IoT and real-time data collection scenarios.

Here’s a step-by-step explanation for each main part of the code:

```js
// The script imports the required modules and creates an MQTT client
// configured to connect to the local MQTT broker at port 5653.
const system = require("@jsh/system");
const mqtt = require("@jsh/mqtt");
var conf = { serverUrls: ["tcp://127.0.0.1:5653"] };
var client = new mqtt.Client(conf);

// Sets up the publish options:
var pubOpt = {
    topic:"db/write/EXAMPLE", // Data will be written to the EXAMPLE table.
    qos:0,                    // Quality of Service level 0 (at most once).
    properties: {
        user: {
            method: "append", // "append" mode.
            timeformat: "ms", // timestamps are in milliseconds.
        },
    },
};

// Prepares an array of records to be written.
// Each record contains a name,
// a timestamp (in milliseconds), and a value.
ts = (new Date()).getTime();
var pubPayload = [
    [ "my-car", ts+0, 32.1 ],
    [ "my-car", ts+1, 65.4 ],
    [ "my-car", ts+2, 76.5 ],
];

client.onConnect = ()=>{
    // When the client connects to the broker,
    // it publishes the prepared payload to the specified topic
    // with the defined options.
    client.publish(pubOpt, JSON.stringify(pubPayload))
}

// The client connects to the broker (with a 3-second timeout),
// sends the data,
// and then disconnects after ensuring all messages have been sent.
client.connect({timeout:3000});
client.disconnect({waitForEmptyQueue: true, timeout:3000});
```

**PUBLISH single record**

- mqtt-data.json

```json
[ "my-car", 1670380345000000000, 87.6 ]
```

```sh
mosquitto_pub -h 127.0.0.1 -p 5653 \
    -t db/write/EXAMPLE \
    -V 5 \
    -D PUBLISH user-property method append \
    -f ./mqtt-data.json
```

**PUBLISH gzip JSON**

```sh
mosquitto_pub -h 127.0.0.1 -p 5653 \
    -t db/write/EXAMPLE \
    -V 5 \
    -D PUBLISH user-property method append \
    -D PUBLISH user-property compress gzip \
    -f mqtt-data.json.gz
```

### NDJSON

NDJSON (Newline Delimited JSON) is a format for streaming JSON data where each line is a valid JSON object. This is useful for processing large datasets or streaming data.
Each line should be a complete JSON object where all field names match the columns of the table.

- mqtt-nd.json

```json
{"NAME":"ndjson-data", "TIME":1670380342000000000, "VALUE":1.001}
{"NAME":"ndjson-data", "TIME":1670380343000000000, "VALUE":2.002}
```

```sh
mosquitto_pub -h 127.0.0.1 -p 5653 \
    -t db/write/EXAMPLE \
    -V 5 \
    -D PUBLISH user-property method append \
    -D PUBLISH user-property format ndjson \
    -f mqtt-nd.json
```

### CSV

- mqtt-data.csv

```csv
NAME,TIME,VALUE
my-car,1670380346,87.7
my-car,1670380347,98.6
my-car,1670380348,99.9
```

```sh
mosquitto_pub -h 127.0.0.1 -p 5653 \
    -t db/write/EXAMPLE \
    -V 5 \
    -D PUBLISH user-property format csv \
    -D PUBLISH user-property method append \
    -D PUBLISH user-property header skip \
    -D PUBLISH user-property timeformat s \
    -f mqtt-data.csv
```

The highlighted `header` `skip` option indicate that the first line is a header.

**PUBLISH gzip CSV**

- mqtt-data.csv.gz

```csv
NAME,TIME,VALUE
my-car,1670380346,87.7
my-car,1670380347,98.6
my-car,1670380348,99.9
```

```sh
mosquitto_pub -h 127.0.0.1 -p 5653 \
    -t db/write/EXAMPLE \
    -V 5 \
    -D PUBLISH user-property format csv \
    -D PUBLISH user-property method append \
    -D PUBLISH user-property header skip \
    -D PUBLISH user-property timeformat s \
    -D PUBLISH user-property compress gzip \
    -f mqtt-data.csv.gz
```

## INSERT method

It is strongly recommended using append method for the better performance through MQTT.
Refer to `insert` method only in situations where the order of data fields differs from the column order in the table or when not all columns are matched.

If the data has a different number of fields or a different order from the columns in the table,
use the `insert` method instead of the default `append` method.

### JSON

Since `db/write` works in `INSERT INTO table(...) VALUE(...)` SQL statement, it is required the columns in json payload.
The example of `data-write.json` is below.

- mqtt-data.json
```json
{
  "data": {
    "columns": ["name", "time", "value"],
    "rows": [
      [ "wave.pi", 1687481466000000000, 1.2345],
      [ "wave.pi", 1687481467000000000, 3.1415]
    ]
  }
}
```

The `method` option defaults to `insert`, so it can be omitted.

```sh
mosquitto_pub -h 127.0.0.1 -p 5653 \
    -t db/write/EXAMPLE \
    -V 5 \
    -D PUBLISH user-property method insert \
    -f mqtt-data.json
```

### NDJSON

This request message is equivalent that consists INSERT SQL statement as `INSERT into {table} (columns...) values (values...)`

- mqtt-nd.json

```json
{"NAME":"ndjson-data", "TIME":1670380342, "VALUE":1.001}
{"NAME":"ndjson-data", "TIME":1670380343, "VALUE":2.002}
```

The `method` option defaults to `insert`, so it can be omitted.

```sh
mosquitto_pub -h 127.0.0.1 -p 5653 \
    -t db/write/EXAMPLE \
    -V 5 \
    -D PUBLISH user-property method insert \
    -D PUBLISH user-property format ndjson \
    -D PUBLISH user-property timeformat s \
    -f mqtt-nd.json
```

### CSV

Insert methods with CSV data that has a different number or order of fields compared to the table columns is supported only in MQTT v5 using custom properties.

- mqtt-data.csv

```csv
VALUE,NAME,TIME
87.7,my-car,1670380346000
98.6,my-car,1670380347000
99.9,my-car,1670380348000
```

```sh
mosquitto_pub -h 127.0.0.1 -p 5653 \
    -t db/write/EXAMPLE \
    -V 5 \
    -D PUBLISH user-property format csv \
    -D PUBLISH user-property method insert \
    -D PUBLISH user-property header columns \
    -D PUBLISH user-property timeformat ms \
    -f mqtt-data.csv
```

## TQL

Topic `db/tql/{file.tql}` is for invoking TQL file.

When the data transforming is required for writing into the database, prepare the proper *tql* script and publish the data to the topic named `db/tql/{file.tql}`.

Please refer to the [As Writing API](../../tql/writing) for the writing data via MQTT and *tql*.


## Max message size

The maximum size of payload in a PUBLISH message is 256MB by MQTT specification. If a malicious or malfunctioning client sends large messages continuously it can consume all of network bandwidth and computing resource of server side then it may lead server to out of service status. It is good practice to set max message limit as just little more than what client applications demand. The default mqtt max message size is 1MB (`1048576`), it can be adjusted by command line flag like below or `MaxMessageSizeLimit` in the configuration file.

```sh
machbase-neo serve --mqtt-max-message 1048576
```