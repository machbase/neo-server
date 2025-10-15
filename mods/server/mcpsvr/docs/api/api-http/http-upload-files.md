# Machbase Neo HTTP Upload Files

A client can upload arbitrary files to Machbase-Neo via HTTP *multipart/form-data* encoding.
The attached files are stored in the specified directory,
and the database keeps the metadata of the file as a JSON string in the column.

Post the column and value in *multipart/form-data* encoding,
where the name of each part should be the column name and the value should be a string representation of the data.
Attach the file to a `JSON` type column with the `X-Store-Dir` header to specify the directory
where the file should be stored.
If the specified directory in `X-Store-Dir` does not exist, the server will create it automatically.
The file content will be stored in the directory with a generated unique name in UUID format.

## Upload file

This demo assumes the `EXAMPLE` table has already been created:

```sql
CREATE TAG TABLE EXAMPLE(
    NAME     VARCHAR(80)  primary key,
    TIME     DATETIME basetime,
    VALUE    DOUBLE,
    EXTDATA  JSON
)
```

**HTTP:**

This example uses Visual Studio Code's REST Client extension.

~~~
```http
POST http://127.0.0.1:5654/db/write/EXAMPLE
Content-Type: multipart/form-data; boundary=----Boundary7MA4YWxkTrZu0gW

------Boundary7MA4YWxkTrZu0gW
Content-Disposition: form-data; name="NAME"

camera-1
------Boundary7MA4YWxkTrZu0gW
Content-Disposition: form-data; name="TIME"

now
------Boundary7MA4YWxkTrZu0gW
Content-Disposition: form-data; name="VALUE"

0
------Boundary7MA4YWxkTrZu0gW
Content-Disposition: form-data; name="EXTDATA"; filename="image_file.png"
X-Store-Dir: /tmp/store
Content-Type: image/png

< /data/image_file.png
------Boundary7MA4YWxkTrZu0gW--
```
~~~

**cURL:**

This example uses `curl` with the `-F` option to make a POST request in *multipart/form-data* encoding.

```sh
curl -X POST 'http://127.0.0.1:5654/db/write/EXAMPLE' \
    -F 'NAME=camera-1' \
    -F 'TIME=now' \
    -F 'VALUE=0' \
    -F 'EXTDATA=@./data/image_file.png;headers="X-Store-Dir: /tmp/store"'
```

### X-Store-Dir

The uploaded file content is stored in the directory specified by the `X-Store-Dir` header,
and the file name is based on the `ID` of the response.
The `X-Store-Dir` header can be included in the part's header, as shown in the example above,
or it can be specified as a top-level header.

**`${data}`**

You can use the `${data}` variable in the `X-Store-Dir` path to represent the database home directory. This directory is specified by the `--data` flag when launching the machbase-neo process, or it defaults to the sub-directory `machbase_home`  under the directory where the machbase-neo executable resides if the `--data` flag is not used. Please refer to the document about [command line flags](/neo/operations/command-line/).

For example, if you set `X-Store-Dir: ${data}/store`, the uploaded file will be saved in `some/path/to/machbase_home/store/file_name_is_ID_of_the_response`.

### Response Message
Once the file is successfully uploaded, the server responds with the stored file information as shown below.

- ID : Unique id assigned by the server
- FN : Original file name
- SZ : File size
- CT : Content-Type
- SD : Stored directory path in the server side

```json
{
  "success": true,
  "reason": "success, 1 record(s) inserted",
  "elapse": "3.772042ms",
  "data": {
    "files": {
      "EXTDATA": {
        "ID": "1ef8a87f-96bd-6576-9ff5-972fa7638db8",
        "FN": "image_file.png",
        "SZ": 12692,
        "CT": "image/png",
        "SD": "/tmp/store"
      }
    }
  }
}
```

The `EXTDATA` column in the above example can be accessed as a normal JSON format "string" data 
which contains the meta information of the uploaded file.

```sql
SELECT EXTDATA FROM EXAMPLE
WHERE NAME = 'camera-1' 
```

Or, use JSON path:

```sql
SELECT EXTDATA FROM EXAMPLE
WHERE NAME = 'camera-1'
AND EXTDATA->'$.FN' = 'image_file.png';
```

The examples that use the SELECT query with the `/db/query` API:

**HTTP:**
~~~
```http
GET http://127.0.0.1:5654/db/query
  ?q=select EXTDATA from EXAMPLE where NAME = 'camera-1'
```
~~~

**cURL:**
```sh
curl -o - 'http://127.0.0.1:5654/db/query' \
  --data-urlencode "q=select EXTDATA from EXAMPLE\
    where NAME = 'camera-1'"
```

Using json path condition with `->` notation.

**HTTP:**
~~~
```http
GET http://127.0.0.1:5654/db/query
  ?q=select EXTDATA from EXAMPLE where NAME = 'camera-1' and EXTDATA->'$.FN' = 'image_file.png'
```
~~~

**cURL:**
```sh
curl -o - 'http://127.0.0.1:5654/db/query' \
  --data-urlencode "q=select EXTDATA from EXAMPLE\
    where NAME = 'camera-1' \
    and EXTDATA->'$.FN' = 'image_file.png'"
```

The `EXTDATA` column contains the file information as shown below.

```json
{
  "data": {
    "columns": [ "EXTDATA" ],
    "types": [ "string" ],
    "rows": [
      [
        "{\"ID\":\"1ef8a87f-96bd-6576-9ff5-972fa7638db8\",\"FN\":\"image_file.png\",\"SZ\":12692,\"CT\":\"image/png\",\"SD\":\"/tmp/store\"}"
      ]
    ]
  },
  "success": true,
  "reason": "success",
  "elapse": "843.666µs"
}
```

Use JSON path to extract specific fields from the JSON type column:

**HTTP:**
~~~
```http
GET http://127.0.0.1:5654/db/query
    ?format=ndjson
    &q=SELECT NAME, TIME, EXTDATA->'$.ID' as FID  FROM EXAMPLE WHERE NAME = 'camera-1'
```
~~~

**cURL:**
```sh
curl -o - http://127.0.0.1:5654/db/query \
  --data-urlencode "format=ndjson"   \
  --data-urlencode "q=SELECT NAME, TIME, EXTDATA->'$.ID' as FID  \
    FROM EXAMPLE WHERE NAME = 'camera-1'"
```

```json
{"NAME":"camera-1","TIME":1728950208158594000,"FID":"1ef8a87f-96bd-6576-9ff5-972fa7638db8"}
{"NAME":"camera-1","TIME":1728953137384133000,"FID":"1ef8a8ec-b602-6cac-8fb1-ac9c0c1b981b"}
```

## Read content of the file

The file content can be accessed by the query API:

`http://{server_address}/db/query/file/{table}/{column}/{ID}`

If the table is TAG table, set `tag` parameter to improve the server response time:

`http://{server_address}/db/query/{tag_table}/{column}/{ID}?tag=camera-1`

If the table is a LOG table, the `ID` is generated based on the time when the record is inserted. For a TAG table, the `ID` is derived from the base time column of the record.

Since the `ID` contains timestamp information, Machbase-Neo uses it in the `TIME between A and B` clause for TAG tables or `_ARRIVAL_TIME between A and B` for LOG tables to narrow the search range.
Note that the timestamp in the `ID` is *NOT* exactly the same as the base time or `_ARRIVAL_TIME` of a LOG table, it is still useful for improving search performance.

### HTTP GET

**HTTP:**
~~~
```http
GET http://127.0.0.1:5654/db/query/file/EXAMPLE/EXTDATA/1ef8a87f-96bd-6576-9ff5-972fa7638db8
```
~~~

**cURL:**
```sh
curl -o ./img-download.png \
    http://127.0.0.1:5654/db/query/file/EXAMPLE/EXTDATA/1ef8a87f-96bd-6576-9ff5-972fa7638db8
```

### Use &lt;img&gt; in HTML

```html
<html>
<body>
<img src="http://127.0.0.1:5654/db/query/file/EXAMPLE/EXTDATA/1ef8a87f-96bd-6576-9ff5-972fa7638db8">
</body>
</html>
```

If the table is a TAG table and the tag name is known, use the `tag` query parameter to improve query performance:

```html
<html>
<body>
<img src="http://127.0.0.1:5654/db/query/file/EXAMPLE/EXTDATA/1ef8a87f-96bd-6576-9ff5-972fa7638db8?tag=camera-1">
</body>
</html>
```

## Examples

### Javascript

Uploading file in Javascript.

```js
const request = require('request');
const fs = require('fs');

let req = {
    method: 'POST',
    url: 'http://127.0.0.1:5654/db/write/EXAMPLE',
    headers: {"X-Store-Dir": "/tmp/store"},
    formData: {
        NAME: 'camera-1',
        TIME: 'now',
        VALUE: 0,
        EXTDATA:  fs.createReadStream('./image_file.png'), 
    },
};

request(req, function(err, res, body){
    if (err) { console.log(err);
    } else { console.log(body); }
})
```

### Python

Uploading file in Python.

```python
import requests

# Define the URL to which the file will be uploaded
url = 'http://127.0.0.1:5654/db/write/EXAMPLE'

# Path to the image file
file_path = './image_file.png'

# Open the image file in binary mode
with open(file_path, 'rb') as file:
    headers = {'X-Store-Dir': '/tmp/store'}
    data = {'NAME': 'camera-1', 'TIME': 'now', 'VALUE': 0}
    files = {'EXTDATA': ('filename.png', file, 'image/png')}

    # Send the POST request
    response = requests.post(url, data=data, files=files, headers=headers)
    
    # Print the response from the server
    print(response.status_code)
    print(response.text)
```