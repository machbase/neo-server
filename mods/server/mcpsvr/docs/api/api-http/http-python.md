# Machbase Neo HTTP Python Client

## Query

### GET CSV

```python
import requests
params = {"q":"select * from example", "format":"csv", "heading":"false"} 
response = requests.get("http://127.0.0.1:5654/db/query", params)
print(response.text)
```

## Write

### POST CSV

```python
import requests
csvdata = """temperature,1677033057000000000,21.1
humidity,1677033057000000000,0.53
"""
response = requests.post(
    "http://127.0.0.1:5654/db/write/example?heading=false", 
    data=csvdata, 
    headers={'Content-Type': 'text/csv'})
print(response.json())
```

## Example - matplotlib

**To write test data, use the command below from the [Write waves by shell](/neo/tutorials/shellscript-waves).**

```sh
sh gen_wave.sh | machbase-neo shell import --timeformat=s EXAMPLE
```

**Python code**

```python
import requests
import json
import datetime
import matplotlib.pyplot as plt
import numpy as np

url = "http://127.0.0.1:5654/db/query"
querystring = {"q":"select * from example order by time limit 200"} 
response = requests.request("GET", url, params=querystring)
data = json.loads(response.text)

sinTs, sinSeries, cosTs, cosSeries = [], [], [], []
for row in data["data"]["rows"]:
    ts = datetime.datetime.fromtimestamp(row[1]/1000000000)
    if row[0] == 'wave.cos':
        cosTs.append(ts)
        cosSeries.append(row[2])
    else:
        sinTs.append(ts)
        sinSeries.append(row[2])

plt.plot(sinTs, sinSeries, label="sin")
plt.plot(cosTs, cosSeries, label="cos")
plt.title("Tutorial Waves")
plt.legend()
plt.show()
```

## Example - pandas

### Load dataframe from table

- Load pandas dataframe from machbase-neo HTTP API.

```python
from urllib import parse
import pandas as pd

query_param = parse.urlencode({
    "q": "select * from example order by time limit 500",
    "format": "csv",
    "timeformat": "s",
})
df = pd.read_csv(f"http://127.0.0.1:5654/db/query?{query_param}")
df
```

### Write dataframe into table

- Write pandas dataframe into a tag table via machbase-neo HTTP API.

```python
import io, requests

stream = io.StringIO()
df.to_csv(stream, encoding='utf-8', header=False, index=False)
stream.seek(0)

file_upload_resp = requests.post(
    "http://127.0.0.1:5654/db/write/example?timeformat=s&method=append",
    headers={'Content-type':'text/csv'},
    data=stream )

print(file_upload_resp.json())
```

```python
{'success': True, 'reason': 'success, 500 record(s) appended', 'elapse': '2.288791ms'}
```

### Load CSV

Import pandas and urllib.

```py
from urllib import parse
import pandas as pd
```

Make query url for `"format": "csv"` option, then call `read_csv`.
Use `timeformat` to specify the precision of time data. `s`, `ms`, `us` and `ns`(default) are available.

```py
query_param = parse.urlencode({
    "q":"select * from example order by time limit 500",
    "format": "csv",
    "timeformat": "s",
})
df = pd.read_csv(f"http://127.0.0.1:5654/db/query?{query_param}")
df
```

### Load compressed CSV

Read gzip'ed CSV from HTTP API.

```py
from urllib import parse
import pandas as pd
import requests
import io
```

```py
query_param = parse.urlencode({
    "q":"select * from example order by time desc limit 1000",
    "format": "csv",
    "timeformat": "s",
    "compress": "gzip",
})
response = requests.get(f"http://127.0.0.1:5654/db/query?{query_param}", timeout=30, stream=True)
df = pd.read_csv(io.BytesIO(response.content))
df
```