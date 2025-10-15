# Machbase Neo HTTP Go Client

## Query

### GET

Make a SQL query with URL escaping.

```go
q := url.QueryEscape("select count(*) from M$SYS_TABLES where name = 'TAGDATA'")
```

Call HTTP GET method.

```go
import (
	"fmt"
	"io"
	"net/http"
	"net/url"
)

func main() {
	client := http.Client{}
	q := url.QueryEscape("select count(*) from M$SYS_TABLES where name = 'TAGDATA'")
	rsp, err := client.Get("http://127.0.0.1:5654/db/query?q=" + q)
	if err != nil {
		panic(err)
	}

	body, err := io.ReadAll(rsp.Body)
	if err != nil {
		panic(err)
	}

	content := string(body)

	if rsp.StatusCode != http.StatusOK {
		panic(fmt.Errorf("ERR %s %s", rsp.Status, content))
	}

	fmt.Println(content)
}
```

### POST JSON

A client can request a JSON message containing a SQL query.

Make JSON content with a SQL query.

```go
queryJson := `{"q":"select count(*) from M$SYS_TABLES where name = 'TAGDATA'"}`
```

Call HTTP POST method with the Content-type.

```go
rsp, err := client.Post(addr, "application/json", bytes.NewBufferString(queryJson))
```

**Full source code**

```go
package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
)

func main() {
	addr := "http://127.0.0.1:5654/db/query"
	queryJson := `{"q":"select count(*) from M$SYS_TABLES where name = 'TAGDATA'"}`
	client := http.Client{}
	rsp, err := client.Post(addr, "application/json", bytes.NewBufferString(queryJson))
	if err != nil {
		panic(err)
	}

	body, err := io.ReadAll(rsp.Body)
	if err != nil {
		panic(err)
	}

	content := string(body)

	if rsp.StatusCode != http.StatusOK {
		panic(fmt.Errorf("ERR %s %s", rsp.Status, content))
	}

	fmt.Println(content)
}
```

### POST FormData

It is possible to send SQL query from HTML form data.

```go
data := url.Values{"q": {"select count(*) from M$SYS_TABLES where name = 'TAGDATA'"}}
rsp, err := client.Post(addr, "application/x-www-form-urlencoded", bytes.NewBufferString(data.Encode()))
```

**Full source code**

```go
package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

func main() {
	addr := "http://127.0.0.1:5654/db/query"
	data := url.Values{"q": {"select count(*) from M$SYS_TABLES where name = 'TAGDATA'"}}
	client := http.Client{}
	rsp, err := client.Post(addr, "application/x-www-form-urlencoded", bytes.NewBufferString(data.Encode()))
	if err != nil {
		panic(err)
	}

	body, err := io.ReadAll(rsp.Body)
	if err != nil {
		panic(err)
	}

	content := string(body)

	if rsp.StatusCode != http.StatusOK {
		panic(fmt.Errorf("ERR %s %s", rsp.Status, content))
	}

	fmt.Println(content)
}
```

## Write

### POST JSON

```go
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type WriteRsp struct {
	Success bool         `json:"success"`
	Reason  string       `json:"reason"`
	Elapse  string       `json:"elapse"`
	Data    WriteRspData `json:"data"`
}

type WriteRspData struct {
	AffectedRows uint64 `json:"affectedRows"`
}

func main() {
	addr := "http://127.0.0.1:5654/db/write/TAGDATA"

	rows := [][]any{
		{"my-car", time.Now().UnixNano(), 32.1},
		{"my-car", time.Now().UnixNano(), 65.4},
		{"my-car", time.Now().UnixNano(), 76.5},
	}
	columns := []string{"name", "time", "value", "jsondata"}
	writeReq := map[string]any{
		"data": map[string]any{
			"columns": columns,
			"rows":    rows,
		},
	}

	queryJson, _ := json.Marshal(&writeReq)
	client := http.Client{}
	rsp, err := client.Post(addr, "application/json", bytes.NewBuffer(queryJson))
	if err != nil {
		panic(err)
	}

	body, err := io.ReadAll(rsp.Body)
	if err != nil {
		panic(err)
	}

	content := string(body)

	if rsp.StatusCode != http.StatusOK {
		panic(fmt.Errorf("ERR %s %s", rsp.Status, content))
	}

	fmt.Println(content)
}
```

### POST CSV

```go
package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type WriteRsp struct {
	Success bool         `json:"success"`
	Reason  string       `json:"reason"`
	Elapse  string       `json:"elapse"`
	Data    WriteRspData `json:"data"`
}

type WriteRspData struct {
	AffectedRows uint64 `json:"affectedRows"`
}

func main() {
	addr := "http://127.0.0.1:5654/db/write/TAGDATA"

	rows := []string{
		fmt.Sprintf("my-car,%d,32.1", time.Now().UnixNano()),
		fmt.Sprintf("my-car,%d,65.4", time.Now().UnixNano()),
		fmt.Sprintf("my-car,%d,76.5", time.Now().UnixNano()),
	}

	client := http.Client{}
	rsp, err := client.Post(addr, "text/csv", bytes.NewBuffer([]byte(strings.Join(rows, "\n"))))
	if err != nil {
		panic(err)
	}

	body, err := io.ReadAll(rsp.Body)
	if err != nil {
		panic(err)
	}

	content := string(body)

	if rsp.StatusCode != http.StatusOK {
		panic(fmt.Errorf("ERR %s %s", rsp.Status, content))
	}

	fmt.Println(content)
}
```

## Example

**This example assumes the table below exists.**

```sql
CREATE TAG TABLE IF NOT EXISTS EXAMPLE (
    NAME VARCHAR(20) PRIMARY KEY,
    TIME DATETIME BASETIME,
    VALUE DOUBLE SUMMARIZED
);
```

If you are a Go programmer and prefer to write RESTful API client, this is the way to go.

### Code explains

Define data structure that represents the payload of write API.

```go
type WriteReq struct {
    Table string       `json:"table"`
    Data  WriteReqData `json:"data"`
}

type WriteReqData struct {
    Columns []string `json:"columns"`
    Rows    [][]any  `json:"rows"`
}
```

The API for writing data via HTTP is explained in [here](/neo/api-http/write) 
and it expects to receive JSON payload.

We can prepare payload like below code, so that write multiple records within a payload.
Assume `sin`, `cos` variables are properly initialized `float64` values.

```go
content, _ := json.Marshal(&WriteReq{
    Data: WriteReqData{
        Columns: []string{"name", "time", "value"},
        Rows: [][]any{
            {"wave.sin", ts.UTC().UnixNano(), sin},
            {"wave.cos", ts.UTC().UnixNano(), cos},
        },
    },
})
```

It will be encoded as JSON for writing API like below.


```json
{
    "data": {
        "columns":["name", "time", "value"],
        "rows": [
            [ "wave.sin", 1670380342000000000, 1.1 ],
            [ "wave.cos", 1670380343000000000, 2.2 ]
        ]
    }
}
```

Send it to server via http POST request.

```go
client := http.Client{}
rsp, err := client.Post("http://127.0.0.1:5654/db/write/EXAMPLE", 
    "application/json", bytes.NewBuffer(content))
```

Server replies `HTTP 200 OK` if it successfully writes data.

### Full source code

```go
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"time"
)

type WriteReq struct {
	Table string       `json:"table"`
	Data  WriteReqData `json:"data"`
}

type WriteReqData struct {
	Columns []string `json:"columns"`
	Rows    [][]any  `json:"rows"`
}

func main() {
	client := http.Client{}
	for ts := range time.Tick(500 * time.Millisecond) {
		delta := float64(ts.UnixMilli()%15000) / 15000
		theta := 2 * math.Pi * delta
		sin, cos := math.Sin(theta), math.Cos(theta)

		content, _ := json.Marshal(&WriteReq{
			Table: "EXAMPLE",
			Data: WriteReqData{
				Columns: []string{"name", "time", "value"},
				Rows: [][]any{
					{"wave.sin", ts.UTC().UnixNano(), sin},
					{"wave.cos", ts.UTC().UnixNano(), cos},
				},
			},
		})
		rsp, err := client.Post(
			"http://127.0.0.1:5654/db/write/example",
			"application/json",
			bytes.NewBuffer(content))
		if err != nil {
			panic(err)
		}
		if rsp.StatusCode != http.StatusOK {
			panic(fmt.Errorf("response %d", rsp.StatusCode))
		}
	}
}
```