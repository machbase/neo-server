# Machbase Neo HTTP C# Client

## Query

### GET CSV

```c#
using HttpClient client = new();
var q = System.Net.WebUtility.UrlEncode("select * from example");
var json = await client.GetStringAsync(
    "http://127.0.0.1:5654/db/query?q="+q
);
Console.Write(json);
```

## Write

### POST CSV

```c#
using HttpClient client = new();
var payload = new System.Net.Http.StringContent(
    @"temperature,1677033057000000000,21.1
    humidity,1677033057000000000,0.53",
    new System.Net.Http.Headers.MediaTypeHeaderValue("text/csv"));

var rsp = await client.PostAsync(
    "http://127.0.0.1:5654/db/write/example?heading=false", payload
);
Console.Write(rsp.ToString());
```