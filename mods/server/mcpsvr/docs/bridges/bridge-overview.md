# Machbase Neo Bridge and Subscriber

## Bridge

### Register a bridge

register sqlite connection

```
bridge add -t sqlite sqlitedb file:/data/sqlite.db;
```

### List registered bridges

```
bridge list
┌──────────┬────────┬────────────────────────┐
│ NAME     │ TYPE   │ CONNECTION             │
├──────────┼────────┼────────────────────────┤
│ sqlitedb │ sqlite │ file:/data/sqlite.db   │
└──────────┴────────┴────────────────────────┘
```

### Execute commands on the bridge

```
bridge exec sqlitedb CREATE TABLE IF NOT EXISTS example(id INTEGER NOT NULL PRIMARY KEY, name TEXT, age TEXT, address TEXT, UNIQUE(name));
```

### Query command on the bridge

`bridge query` command is only works with "SQL" type bridges

```
bridge query sqlitedb select * from example;

┌────┬────────┬─────┬───────────────┐
│ ID │ NAME   │ AGE │ ADDRESS       │
├────┼────────┼─────┼───────────────┤
│  1 │ hong_1 │ 20  │ address for 1 │
│  2 │ hong_2 │ 20  │ address for 2 │
│  3 │ hong_3 │ 20  │ address for 3 │
└────┴────────┴─────┴───────────────┘
```

### Utilize a bridge in *tql* with `SQL()`

`SQL()` takes `bridge()` option with "SQL" type bridge and execute the given SQL statement.

```js
SQL(bridge("sqlitedb"), `select * from example`)
CSV()
```

### Utilize a bridge in *tql* `SCRIPT()`

You can access database-type bridges from `SCRIPT()` using `$.db({bridge:"name"})`, as shown in the example below.

Support for accessing bridged databases in JavaScript using `$.db({bridge:"name"})` has been available since version 8.0.27.

```js
SCRIPT({
    err = $.db({bridge:"mem"})
     .query("select company, employee, created_on from mem_example")
     .forEach( function(fields){
        $.yield(fields[0], fields[1], fields[2]);
     })
    if (err !== undefined) {
        console.error("result", ret);
    }
})
CSV()
```

### Copy data to other database

This example demonstrates how to copy data from Machbase to an SQLite bridge.

**Bridge**

Define a `sqlite` bridge with the following details:

- Type: `SQLite`
- Connection string: `file:///tmp/sqlite.db`

**SQL**

Create the `example` table in the SQLite database located at "/tmp/sqlite.db".

```sql
--env: bridge=sqlite
CREATE TABLE IF NOT EXISTS example (
    NAME TEXT,
    TIME DATETIME,
    VALUE REAL
);
-- env: reset
```

**TQL**

The TQL script below executes a `SELECT` statement using the `SQL()` function to retrieve the required data, and then writes the data into the SQLite database using the `INSERT()` function with `bridge("sqlite")` as the first argument.

```js
SQL(`select name, time, value from example where name = 'my-car'`)
INSERT(bridge("sqlite"), "name", "time", "value", table("example"))
```

## Subscriber

The purpose of a *subscriber* is connecting to an external message broker system, receiving streaming messages, ingesting messages by *tql* script.

Currently machbase-neo supports connecting to the external MQTT brokers, and it will support also NATS and Kafka with the future releases.

A simple use case is that make a bridge to the external MQTT broker, and define a subscriber with 1) the bridge, 2) a topic of the MQTT broker and 3) *tql* script path. Then machbase-neo works as MQTT client and whenever it receives messages, it passes them to the specified *tql* script.

```mermaid
flowchart RL
    external-system --PUBLISH--> machbase-neo
    machbase-neo --SUBSCRIBE--> external-system
    subgraph machbase-neo
        direction RL
        bridge --> subscriber
        subscriber["Subscriber
                    TQL"] --Write--> machbase
        machbase[("machbase
                    engine")]
    end
    subgraph external-system
        direction RL
        client["Client"] --PUBLISH--> mqtt[["MQTT
                                            Broker"]]
    end
```

### Register a subscriber

Register subscribers.

**Syntax:** `subscriber add [options] <name> <bridge> <topic> <tql-path>`

- options
    - `--autostart` makes the subscriber will start automatically when machbase-neo starts. If the subscriber is not *autostart* mode, you can make it start and stop manually by `subscriber start <name>` and `subscriber stop <name>` commands.
    - `--qos <int>` if the bridge is MQTT type, it specifies the QoS level of the subscription to the topic. It supports `0`, `1` and the default is `0` if it is not specified.
    - `--queue <string>` if the bridge is NATS type, it specifies the Queue Group.

- `<name>` subscriber's name
- `<bridge>` specify pre-defined bridge, it should be a type of the broker
- `<topic>` topic to subscribe
- `<tql-path>` the *tql* script that handles the received message

### Subscriber Status

**Syntax:** `subscriber list`

- `STOP`
- `RUNNING`

### Subscriber Start/Stop

**Syntax:** `subscriber [start | stop] <name>`

### Remove subscriber

**Syntax:** `subscriber del <name>`