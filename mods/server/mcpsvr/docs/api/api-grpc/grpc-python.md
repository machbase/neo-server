# Machbase Neo gRPC Python Client

## Setup

### Python gRPC

Install gRPC compiler for Python.

```sh
pip3 install grpcio grpcio-tools
```

### Download `machrpc.proto` and generate code

Make a working directory.

```sh
mkdir machrpc-py && cd machrpc-py
```

Download proto file.

```sh
curl -o machrpc.proto https://raw.githubusercontent.com/machbase/neo-server/main/api/proto/machrpc.proto
```

Compile proto file into Python.

```sh
python3 -m grpc_tools.protoc \
    -I . \
    --python_out=. \
    --grpc_python_out=. \
    ./machrpc.proto
```

As result, it generates two python files `machrpc_pb2.py` and `machrpc_pb2_grpc.py`

## Query

### Type `any` converter

The machbase-neo gRPC is relying on `"google/protobuf/any.proto` package for its data types.
It is required to define a type conversion function.

The function below is convert protobuf any type to proper python data types.

### Convert protobuf.any value to python data type

```python
from google.protobuf.any_pb2 import Any
import google.protobuf.timestamp_pb2 as pb_ts
import google.protobuf.wrappers_pb2 as pb_wp
import time
from datetime import datetime

def convpb(v):
    if v.type_url == "type.googleapis.com/google.protobuf.StringValue":
        r = pb_wp.StringValue()
        v.Unpack(r)
        return r.value
    elif v.type_url == "type.googleapis.com/google.protobuf.Timestamp":
        r = pb_ts.Timestamp()
        v.Unpack(r)
        dt = datetime.fromtimestamp(r.seconds)
        return dt.strftime('%Y-%m-%d %H:%M:%S')
    elif v.type_url == "type.googleapis.com/google.protobuf.DoubleValue":
        r = pb_wp.DoubleValue()
        v.Unpack(r)
        return str(r.value)
```

### Connect

Import gRPC runtime package and generated files.

```python
import grpc
import machrpc_pb2_grpc
import machrpc_pb2
```

Make gRPC channel to server then create a machbase-neo API stub.

```python
channel = grpc.insecure_channel('127.0.0.1:5655')
mach_stub = machrpc_pb2_grpc.MachbaseStub(channel)
```

### Execute query

Run SQL query with the stub.

```python
sqlText = "select * from example order by time limit 10"
rsp = mach_stub.Query(machrpc_pb2.QueryRequest(sql=sqlText))
```

### Get columns info of result set

We can get columns meta information of result rows after executing a query.

```python
cols = mach_stub.Columns(rsp.rowsHandle)
if cols.success:
    header = ['RowNum']
    for c in cols.columns:
        header.append(f"{c.name}({c.type})  ")
    print('   '.join(header))
```

### Fetch results

Retrieve the result records by calling `Fetch`.

```python
nrow = 0
while True:
    fetch = mach_stub.RowsFetch(rsp.rowsHandle)
    if fetch.hasNoRows:
        break
    nrow+=1
    line = []
    line.append(str(nrow))
    for i, c in enumerate(cols.columns):
        v = fetch.values[i]
        if c.type == "string":
            line.append(convpb(v))
        elif c.type == "datetime":
            line.append(convpb(v))
        elif c.type == "double":
            line.append(convpb(v))
        else:
            line.append(f"unknown {str(v)}")
    print('     '.join(line))
_ = mach_stub.RowsClose(rsp.rowsHandle)
```
 
**Rows must be Closed**: It is important to close rows by calling `RowsClose(handle)`.

## Append

### Import

```python
import grpc
import machrpc_pb2 as mach
import machrpc_pb2_grpc as machrpc
import numpy as np 
import time
import google.protobuf.wrappers_pb2 as pb_wp
from google.protobuf.any_pb2 import Any
```

### `Any` type converters for protocol buffer

```python
def AnyString(str: str):
    pbstr = pb_wp.StringValue()
    pbstr.value = str
    anystr = Any()
    anystr.Pack(pbstr)
    return anystr

def AnyInt64(iv: int):
    pbint = pb_wp.Int64Value()
    pbint.value = iv
    anyint = Any()
    anyint.Pack(pbint)
    return anyint

def AnyFloat(fv: float):
    pbfloat = pb_wp.DoubleValue()
    pbfloat.value = fv
    anyfloat = Any()
    anyfloat.Pack(pbfloat)
    return anyfloat
```

### Generate values

```python
sample_rate = 100
start_time = 0
end_time = 1000

timeseries = np.arange(start_time, end_time, 1/sample_rate)
frequency = 3
ts = time.time_ns()

data = list[list[Any]]()
for i, t in enumerate(timeseries):
    nanot = ts + int(t*1000000000)
    value = np.sin(2 * np.pi * frequency * t)
    data.append([AnyString("python.value"), AnyInt64(nanot), AnyFloat(value)])
```

### Connect to server

```python
channel = grpc.insecure_channel('127.0.0.1:5655')
mach_stub = machbase_proto_pb2_grpc.MachbaseStub(channel)
```

### Prepare new appender

```python
appender = stub.Appender(mach.AppenderRequest(tableName="example"))
```

### Streaming writing data

```python
def ToStream(rows: list[list[Any]]):
    for row in rows:
        yield mach.AppendData(handle = appender.handle, params = row)

stub.Append(ToStream(data))
```