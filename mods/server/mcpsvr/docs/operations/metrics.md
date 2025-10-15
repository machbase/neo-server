# Machbase Neo Metrics

> **BETA Warning**  
> The features described in this document are subject to change and may be updated in future releases.

The metrics are provided in 1 minute, 5 minutes and 15 minutes sampling periods.

## HTTP API

To retrieve the metrics via the RESTful API, use the endpoint:

```
http://127.0.0.1:5654/debug/statz?interval=[1m|5m|15m]&format=[json|html]
```

This endpoint allows you to specify the interval for which you want to gather metrics, choosing from 1 minute, 5 minutes, or 15 minutes. Please note that this endpoint is only accessible from the same machine (localhost) by default.

By default, the output format is JSON. If `format=html` is specified, the response will be in an HTML table.

## TQL with CHART

The example below shows how to render machbase-neo's HTTP latency distribution in a chart. It uses `FAKE( statz(period, metrics...) )` SRC function, and then makes time-value pairs for input of the `CHART()`.

```js
FAKE(statz("15m", 
    "machbase:http:latency_p50",
    "machbase:http:latency_p90",
    "machbase:http:latency_p99"
))
MAPVALUE(1, list(value(0), value(1)))
MAPVALUE(2, list(value(0), value(2)))
MAPVALUE(3, list(value(0), value(3)))
CHART(
    size("600px", "300px"),
    chartJSCode({
        function yformatter(val, idx){
            if (val > 1000000000)   { return `${val/1000000000} s`; }
            else if (val > 1000000) { return `${val/1000000} ms`; } 
            else if (val > 1000)    { return `${val/1000} µs`; }
            return `${val} ns`
        }
    }),
    chartOption({
        animation: false,
        yAxis: { type: "value", axisLabel:{ formatter: yformatter }},
        xAxis: { type: "time", axisLabel:{ rotate: -90 }},
        series: [
            {type: "line", data: column(3), areaStyle:{}, smooth:false, name: "p99"},
            {type: "line", data: column(2), areaStyle:{}, smooth:false, name: "p90"},
            {type: "line", data: column(1), areaStyle:{}, smooth:false, name: "p50"},
        ],
        tooltip: { trigger: "axis", valueFormatter: yformatter },
        legend: {}
    })
)
```

## Metrics

All metrics are based on the selected sampling period, which can be one of the following: 1 minute (`1m`), 5 minutes (`5m`), or 15 minutes (`15m`).

### HTTP

| Metric                      |  Description                                                |
|:----------------------------|:------------------------------------------------------------|
| `machbase:http:count`       |  Total number of HTTP requests                              |
| `machbase:http:latency_p50` |  HTTP response latency at the 50th percentile, median (ns.) |
| `machbase:http:latency_p90` |  HTTP response latency at the 90th percentile (ns.)         |
| `machbase:http:latency_p99` |  HTTP response latency at the 99th percentile (ns.)         |
| `machbase:http:recv_bytes`  |  Total size of HTTP request payloads                        |
| `machbase:http:send_bytes`  |  Total size of HTTP response payloads                       |
| `machbase:http:status_1xx`  |  Number of HTTP responses with 1xx status codes             |
| `machbase:http:status_2xx`  |  Number of HTTP responses with 2xx status codes             |
| `machbase:http:status_3xx`  |  Number of HTTP responses with 3xx status codes             |
| `machbase:http:status_4xx`  |  Number of HTTP responses with 4xx status codes             |
| `machbase:http:status_5xx`  |  Number of HTTP responses with 5xx status codes             |

### MQTT

| Metric                        | Description                                   |
|:------------------------------|:----------------------------------------------|
| `machbase:mqtt:recv_bytes`    | total number of bytes received (bytes)        |
| `machbase:mqtt:send_bytes`    | total number of bytes sent (bytes)            |
| `machbase:mqtt:recv_pkts`     | the total number of publish messages received |
| `machbase:mqtt:send_pkts`     | total number of messages of any type sent     |
| `machbase:mqtt:recv_msgs`     | total number of publish messages received     |
| `machbase:mqtt:send_msgs`     | total number of publish messages sent         |
| `machbase:mqtt:drop_msgs`     | total number of publish messages dropped to slow subscriber  |
| `machbase:mqtt:retained`      | total number of retained messages active on the broker       |
| `machbase:mqtt:subscriptions` | total number of subscriptions active on the broker           |
| `machbase:mqtt:clients`       | total number of connected and disconnected clients with a persistent session currently connected and registered  |
| `machbase:mqtt:clients_connected`      | number of currently connected clients  |
| `machbase:mqtt:clients_disconnected`   | total number of persistent clients (with clean session disabled) that are registered at the broker but are currently disconnected  |
| `machbase:mqtt:inflight`               | the number of messages currently in-flight          |
| `machbase:mqtt:inflight_dropped`       | the number of inflight messages which were dropped  |

### TQL

| Metric                                         | Description                                     |
|:-----------------------------------------------|:------------------------------------------------|
| `machbase:tql:cache:count_[avg\|max\|min]`     | Number of items in the TQL cache                |
| `machbase:tql:cache:data_size_[avg\|max\|min]` | Total size of the TQL cache (bytes)             |
| `machbase:tql:cache:evictions`                 | Number of items evicted from the TQL cache      |
| `machbase:tql:cache:insertions`                | Number of new items inserted into the TQL cache |
| `machbase:tql:cache:hits`                      | Number of cache hits in the TQL cache           |
| `machbase:tql:cache:misses`                    | Number of cache misses in the TQL cache         |

### Database Sessions

| Metric                                             | Description                         |
|:---------------------------------------------------|:------------------------------------|
| `machbase:session:append:count`                    | Total number of appenders used      |
| `machbase:session:append:in_use`                   | Number of appenders currently open  |
| `machbase:session:conn:count`                      | Total number of connections used    |
| `machbase:session:conn:in_use`                     | Number of connections currently open|
| `machbase:session:stmt:count`                      | Total number of statements used     |
| `machbase:session:stmt:in_use`                     | Number of statements currently open |
| `machbase:session:conn:use_time_[avg\|max\|min]`   | Connection usage time (ns.)         |
| `machbase:session:conn:wait_time_[avg\|max\|min]`  | Wait time for fetch iteration limit (ns.)                  |
| `machbase:session:query:count`                     | Total number of queries (only those using fetch iteration) |
| `machbase:session:query:exec_time_[avg\|max\|min]` | Execution time of prepared statements (ns.)                |
| `machbase:session:query:fetch_time_[avg\|max\|min]`| Fetch time (ns.)                                           |
| `machbase:session:query:wait_time_[avg\|max\|min]` | Wait time for iteration limit (ns.)                        |
| `machbase:session:query:hwm:elapse`                | High Water Marked Query total elapsed time (ns.)           |
| `machbase:session:query:hwm:exec_time`             | High Water Marked Query's statement preparation time (ns.) |
| `machbase:session:query:hwm:fetch_time`            | High Water Marked Query's fetch time (ns.)                 |
| `machbase:session:query:hwm:wait_time`             | High Water Marked Query's iteration limit wait time  (ns.) |
| `machbase:session:query:hwm:sql_args`              | High Water Marked Query's SQL bind variables ([]string)    |
| `machbase:session:query:hwm:sql_text`              | High Water Marked Query's SQL text (string)                |

### Go

| Metric                             | Description                          |
|:-----------------------------------|:-------------------------------------|
| `go:heap_in_use_[avg\|max\|min]`   | Heap usage (bytes)                   |
| `go:cgo_call_[avg\|max\|min]`      | Number of CGO function calls         |
| `go:goroutine_[avg\|max\|min]`     | Number of goroutines                 |