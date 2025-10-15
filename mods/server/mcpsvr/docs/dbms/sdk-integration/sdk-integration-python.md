# Python

## Overview

Machbase ships a CPython extension named `machbaseAPI` that wraps the native CLI client distributed with the database. The package exports two classes:

- `machbaseAPI.machbaseAPI`: thin ctypes bindings for the native shared library.
- `machbaseAPI.machbase`: higher-level helper that exposes connection management, SQL execution, metadata lookup, and append protocol helpers.

The examples below target the `machbase` class, which is the entry point for application developers.

## Installation

### Requirements

- Python 3.8 or later with `pip`.
- A reachable Machbase server and credentials (default `SYS/MANAGER` on port `5656`).
- The Machbase shared library included in the wheel (Linux, Windows, macOS) or shipped alongside your source install.

### Install from PyPI

```bash
pip3 install machbaseAPI
```

If `pip3` is not on your PATH, use `python3 -m pip install machbaseAPI`.

### Verify the module

```bash
python3 - <<'PY'
from machbaseAPI.machbaseAPI import machbase
print('machbaseAPI import ok')
cli = machbase()
print('isConnected():', cli.isConnected())
PY
```

A successful run confirms that the package can be imported and instantiated.

## Quick Start

The snippet below connects to a local server, creates a sample table, inserts rows, reads them back, and closes the session.

```python
#!/usr/bin/env python3
import json
from machbaseAPI.machbaseAPI import machbase

def main():
    db = machbase()
    if db.open('127.0.0.1', 'SYS', 'MANAGER', 5656) == 0:
        raise SystemExit(db.result())

    try:
        rc = db.execute('drop table py_sample')
        print('drop table rc:', rc)
        print('drop table result:', db.result())

        ddl = (
            "create table py_sample ("
            "ts datetime,"
            "device varchar(40),"
            "value double"
            ")"
        )
        if db.execute(ddl) == 0:
            raise SystemExit(db.result())
        print('create table result:', db.result())

        for seq in range(3):
            sql = (
                "insert into py_sample values ("
                f"to_date('2024-01-0{seq+1}','YYYY-MM-DD'),"
                f"'sensor-{seq}',"
                f"{20.5 + seq}"
                ")"
            )
            if db.execute(sql) == 0:
                raise SystemExit(db.result())
            print('insert result:', db.result())

        if db.select('select * from py_sample order by ts') == 0:
            raise SystemExit(db.result())

        print('rows available:', db.count())
        while True:
            rc, payload = db.fetch()
            if rc == 0:
                break
            row = json.loads(payload)
            print('row:', row)

        db.selectClose()
    finally:
        if db.close() == 0:
            raise SystemExit(db.result())

if __name__ == '__main__':
    main()
```

## Result Handling

Most `machbase` methods return `1` on success and `0` on failure. After each call, use `db.result()` to read the JSON-formatted payload emitted by the server. The helper `db.count()` reports the row count buffered for the current result set. When iterating over `select()` results, call `db.fetch()` repeatedly until it returns `(0, '')`, then release resources with `db.selectClose()`.

## Supported API Matrix

| Class | API | Description | Return |
| -- | -- | -- | -- |
| `machbase` | `open(host, user, password, port)` | Connect to a Machbase server with default credentials and port. | `1` on success, `0` on failure |
| `machbase` | `openEx(host, user, password, port, conn_str)` | Extended connect with additional connection string attributes. | `1` or `0` |
| `machbase` | `close()` | Terminate the current session. | `1` or `0` |
| `machbase` | `isOpened()` | Check whether a handle has been opened. | `1` or `0` |
| `machbase` | `isConnected()` | Verify that the handle is connected to the server. | `1` or `0` |
| `machbase` | `getSessionId()` | Return the numeric session identifier issued by the server. | Session ID integer |
| `machbase` | `execute(sql, type=0)` | Run a SQL statement (all statements except `UPDATE`). | `1` or `0` |
| `machbase` | `schema(sql)` | Execute schema-related statements. | `1` or `0` |
| `machbase` | `tables()` | Fetch metadata for all tables. | `1` or `0` |
| `machbase` | `columns(table_name)` | Fetch column metadata for a specific table. | `1` or `0` |
| `machbase` | `column(table_name)` | Retrieve column layout using the low-level catalog call. | `1` or `0` |
| `machbase` | `statistics(table_name, user='SYS')` | Request table statistics via the CLI. | `1` or `0` |
| `machbase` | `select(sql)` | Execute a streaming `SELECT` or `DESC`. | `1` or `0` |
| `machbase` | `fetch()` | Pull the next row after `select()`. | `(rc, json_str)` |
| `machbase` | `selectClose()` | Close an open result set cursor. | `1` or `0` |
| `machbase` | `result()` | Return the latest JSON payload. | JSON string |
| `machbase` | `count()` | Return row count for the current buffer. | Integer |
| `machbase` | `checkBit()` | Report pointer width (32 or 64 bits). | `32` or `64` |
| `machbase` | `appendOpen(table_name, types)` | Begin append protocol with column type codes. | `1` or `0` |
| `machbase` | `appendData(table_name, types, values, format)` | Append rows using the active append session. | `1` or `0` |
| `machbase` | `appendDataByTime(table_name, types, values, format, times)` | Append rows with explicit epoch timestamps. | `1` or `0` |
| `machbase` | `appendFlush()` | Flush buffered append rows to disk. | `1` or `0` |
| `machbase` | `appendClose()` | Close the append session. | `1` or `0` |
| `machbase` | `append(table_name, types, values, format)` | Convenience wrapper that opens, appends, and closes. | `1` or `0` |
| `machbase` | `appendByTime(table_name, types, values, format, times)` | Convenience wrapper for time-aware append. | `1` or `0` |
| `machbaseAPI` | `get_library_path()` | Resolve the path of the bundled native client library. | String path |
| `machbaseAPI` | `machbaseAPI.openDB(...)` | Low-level open call (mirrors `machbase.open`). | Pointer or `None` |
| `machbaseAPI` | `machbaseAPI.openDBEx(...)` | Low-level extended open call. | Pointer or `None` |
| `machbaseAPI` | `machbaseAPI.closeDB(handle)` | Close a raw handle. | Integer status |
| `machbaseAPI` | `machbaseAPI.execDirect(handle, sql)` | Execute SQL without classification. | Integer status |
| `machbaseAPI` | `machbaseAPI.execSelect(handle, sql, type)` | Execute SQL and prepare buffered result. | Integer status |
| `machbaseAPI` | `machbaseAPI.execSchema(handle, sql)` | Execute schema statements. | Integer status |
| `machbaseAPI` | `machbaseAPI.execStatistics(handle, table, user)` | Request table statistics. | Integer status |
| `machbaseAPI` | `machbaseAPI.execAppendOpen(...)` | Start append protocol. | Integer status |
| `machbaseAPI` | `machbaseAPI.execAppendData(...)` | Append data rows. | Integer status |
| `machbaseAPI` | `machbaseAPI.execAppendDataByTime(...)` | Append data rows with timestamps. | Integer status |
| `machbaseAPI` | `machbaseAPI.execAppendFlush(handle)` | Flush append buffer. | Integer status |
| `machbaseAPI` | `machbaseAPI.execAppendClose(handle)` | Close append session. | Integer status |
| `machbaseAPI` | `machbaseAPI.getColumns(handle, table)` | Retrieve column metadata. | Integer status |
| `machbaseAPI` | `machbaseAPI.getIsConnected(handle)` | Check connection state flag. | Integer status |
| `machbaseAPI` | `machbaseAPI.getDataCount(handle)` | Return buffered row count. | Unsigned long |
| `machbaseAPI` | `machbaseAPI.getData(handle)` | Return pointer to JSON buffer. | `ctypes.c_char_p` |
| `machbaseAPI` | `machbaseAPI.getlAddr(handle)` | Low part of result pointer (64-bit). | Integer |
| `machbaseAPI` | `machbaseAPI.getrAddr(handle)` | High part of result pointer (64-bit). | Integer |
| `machbaseAPI` | `machbaseAPI.getSessionId(handle)` | Fetch server session id. | Unsigned long |
| `machbaseAPI` | `machbaseAPI.fetchRow(handle)` | Advance result set cursor. | Integer status |
| `machbaseAPI` | `machbaseAPI.selectClose(handle)` | Close select cursor. | Integer status |

## API Reference and Samples

Update host, port, username, and password values as needed in each script. Every snippet is standalone and can be executed with `python3 script.py`.

### Connection management

#### machbase.open(), machbase.isOpened(), machbase.isConnected(), machbase.getSessionId(), machbase.close()

```python
#!/usr/bin/env python3
from machbaseAPI.machbaseAPI import machbase

def main():
    db = machbase()
    print('isOpened before open:', db.isOpened())
    print('isConnected before open:', db.isConnected())

    if db.open('127.0.0.1', 'SYS', 'MANAGER', 5656) == 0:
        raise SystemExit(db.result())

    print('session id:', db.getSessionId())
    print('isOpened after open:', db.isOpened())
    print('isConnected after open:', db.isConnected())

    if db.close() == 0:
        raise SystemExit(db.result())

    print('isOpened after close:', db.isOpened())
    print('isConnected after close:', db.isConnected())

if __name__ == '__main__':
    main()
```

#### machbase.openEx()

```python
#!/usr/bin/env python3
from machbaseAPI.machbaseAPI import machbase

def main():
    db = machbase()
    conn_str = 'APP_NAME=python-demo'
    if db.openEx('127.0.0.1', 'SYS', 'MANAGER', 5656, conn_str) == 0:
        raise SystemExit(db.result())
    print('connected with openEx, session id:', db.getSessionId())
    if db.close() == 0:
        raise SystemExit(db.result())

if __name__ == '__main__':
    main()
```

### DML and result buffers

#### machbase.execute(), machbase.result(), machbase.count()

```python
#!/usr/bin/env python3
import json
from machbaseAPI.machbaseAPI import machbase

def main():
    db = machbase()
    if db.open('127.0.0.1', 'SYS', 'MANAGER', 5656) == 0:
        raise SystemExit(db.result())

    try:
        rc = db.execute('drop table py_exec_demo')
        print('drop table rc:', rc)
        print('drop table result:', db.result())

        ddl = 'create table py_exec_demo(id integer, note varchar(32))'
        if db.execute(ddl) == 0:
            raise SystemExit(db.result())
        print('create table result:', db.result())

        for idx in range(2):
            sql = f"insert into py_exec_demo values ({idx}, 'row-{idx}')"
            if db.execute(sql) == 0:
                raise SystemExit(db.result())
            print('insert result:', db.result())

        if db.execute('select * from py_exec_demo order by id') == 0:
            raise SystemExit(db.result())
        payload = db.result()
        print('select payload:', payload)
        rows = json.loads(payload)
        print('decoded rows:', rows)
        print('row count via count():', db.count())
    finally:
        if db.close() == 0:
            raise SystemExit(db.result())

if __name__ == '__main__':
    main()
```

### Streaming SELECT helpers

#### machbase.select(), machbase.fetch(), machbase.selectClose()

```python
#!/usr/bin/env python3
import json
from machbaseAPI.machbaseAPI import machbase

def main():
    db = machbase()
    if db.open('127.0.0.1', 'SYS', 'MANAGER', 5656) == 0:
        raise SystemExit(db.result())

    try:
        rc = db.execute('drop table py_select_demo')
        print('drop table rc:', rc)
        print('drop table result:', db.result())

        ddl = 'create table py_select_demo(id integer, value double)'
        if db.execute(ddl) == 0:
            raise SystemExit(db.result())
        print('create table result:', db.result())

        for idx in range(5):
            sql = f"insert into py_select_demo values ({idx}, {idx * 1.5})"
            if db.execute(sql) == 0:
                raise SystemExit(db.result())
            print('insert result:', db.result())

        if db.select('select id, value from py_select_demo order by id') == 0:
            raise SystemExit(db.result())

        print('buffered rows:', db.count())
        while True:
            rc, payload = db.fetch()
            if rc == 0:
                break
            print('fetched row:', json.loads(payload))

        db.selectClose()
    finally:
        if db.close() == 0:
            raise SystemExit(db.result())

if __name__ == '__main__':
    main()
```

### Schema helpers

#### machbase.schema()

```python
#!/usr/bin/env python3
from machbaseAPI.machbaseAPI import machbase

def main():
    db = machbase()
    if db.open('127.0.0.1', 'SYS', 'MANAGER', 5656) == 0:
        raise SystemExit(db.result())

    try:
        rc = db.schema('drop table py_schema_demo')
        print('schema drop rc:', rc)
        print('schema drop result:', db.result())

        ddl = 'create table py_schema_demo(name varchar(20), created datetime)'
        if db.schema(ddl) == 0:
            raise SystemExit(db.result())
        print('schema create result:', db.result())
    finally:
        if db.close() == 0:
            raise SystemExit(db.result())

if __name__ == '__main__':
    main()
```

### Metadata and statistics

#### machbase.tables(), machbase.columns(), machbase.column(), machbase.statistics()

```python
#!/usr/bin/env python3
from machbaseAPI.machbaseAPI import machbase

def main():
    db = machbase()
    if db.open('127.0.0.1', 'SYS', 'MANAGER', 5656) == 0:
        raise SystemExit(db.result())

    try:
        if db.tables() == 0:
            raise SystemExit(db.result())
        print('tables metadata:', db.result())

        if db.columns('PY_EXEC_DEMO') == 0:
            raise SystemExit(db.result())
        print('columns metadata:', db.result())

        if db.column('PY_EXEC_DEMO') == 0:
            raise SystemExit(db.result())
        print('column metadata:', db.result())

        if db.statistics('PY_EXEC_DEMO') == 0:
            raise SystemExit(db.result())
        print('statistics output:', db.result())
    finally:
        if db.close() == 0:
            raise SystemExit(db.result())

if __name__ == '__main__':
    main()
```

### Append protocol primitives

`appendOpen()`, `appendData()`, `appendFlush()`, and `appendClose()` stream rows efficiently. The example derives column type codes from `columns()` output before pushing data.

```python
#!/usr/bin/env python3
import json
import re
from machbaseAPI.machbaseAPI import machbase

def main():
    db = machbase()
    if db.open('127.0.0.1', 'SYS', 'MANAGER', 5656) == 0:
        raise SystemExit(db.result())

    try:
        rc = db.execute('drop table py_append_demo')
        print('drop table rc:', rc)
        print('drop table result:', db.result())

        ddl = 'create table py_append_demo(ts datetime, device varchar(32), value double)'
        if db.execute(ddl) == 0:
            raise SystemExit(db.result())
        print('create table result:', db.result())

        if db.columns('PY_APPEND_DEMO') == 0:
            raise SystemExit(db.result())
        column_payload = db.result()
        col_specs = [json.loads(item) for item in re.findall(r'\{[^}]+\}', column_payload)]
        types = [spec.get('type') for spec in col_specs]
        print('append column types:', types)

        if db.appendOpen('PY_APPEND_DEMO', types) == 0:
            raise SystemExit(db.result())

        rows = [
            ['2024-01-01 09:00:00', 'sensor-a', 21.5],
            ['2024-01-01 09:05:00', 'sensor-b', 22.1],
        ]
        if db.appendData('PY_APPEND_DEMO', types, rows) == 0:
            raise SystemExit(db.result())
        print('appendData result:', db.result())

        if db.appendFlush() == 0:
            raise SystemExit(db.result())
        print('appendFlush result:', db.result())

        if db.appendClose() == 0:
            raise SystemExit(db.result())
        print('appendClose result:', db.result())
    finally:
        if db.close() == 0:
            raise SystemExit(db.result())

if __name__ == '__main__':
    main()
```

### Convenience append helpers

#### machbase.append()

```python
#!/usr/bin/env python3
from machbaseAPI.machbaseAPI import machbase

def main():
    db = machbase()
    if db.open('127.0.0.1', 'SYS', 'MANAGER', 5656) == 0:
        raise SystemExit(db.result())

    try:
        db.execute('drop table py_append_auto')
        db.result()
        ddl = 'create table py_append_auto(ts datetime, tag varchar(16), reading double)'
        if db.execute(ddl) == 0:
            raise SystemExit(db.result())
        db.result()

        types = ['6', '5', '20']
        values = [
            ['2024-01-01 10:00:00', 'node-1', 30.0],
            ['2024-01-01 10:01:00', 'node-1', 30.5],
        ]
        if db.append('PY_APPEND_AUTO', types, values) == 0:
            raise SystemExit(db.result())
        print('append() result:', db.result())
    finally:
        if db.close() == 0:
            raise SystemExit(db.result())

if __name__ == '__main__':
    main()
```

#### machbase.appendDataByTime(), machbase.appendByTime()

```python
#!/usr/bin/env python3
from machbaseAPI.machbaseAPI import machbase

def main():
    db = machbase()
    if db.open('127.0.0.1', 'SYS', 'MANAGER', 5656) == 0:
        raise SystemExit(db.result())

    try:
        db.execute('drop table py_append_time')
        db.result()
        ddl = 'create table py_append_time(ts datetime, tag varchar(16), reading double)'
        if db.execute(ddl) == 0:
            raise SystemExit(db.result())
        db.result()

        types = ['6', '5', '20']
        rows = [
            ['2024-01-01 11:00:00', 'node-2', 40.1],
            ['2024-01-01 11:01:00', 'node-2', 40.7],
        ]
        epoch_times = [1704106800, 1704106860]

        if db.appendOpen('PY_APPEND_TIME', types) == 0:
            raise SystemExit(db.result())
        if db.appendDataByTime('PY_APPEND_TIME', types, rows, 'YYYY-MM-DD HH24:MI:SS', epoch_times) == 0:
            raise SystemExit(db.result())
        print('appendDataByTime result:', db.result())
        db.appendClose()

        if db.appendByTime('PY_APPEND_TIME', types, rows, 'YYYY-MM-DD HH24:MI:SS', epoch_times) == 0:
            raise SystemExit(db.result())
        print('appendByTime result:', db.result())
    finally:
        if db.close() == 0:
            raise SystemExit(db.result())

if __name__ == '__main__':
    main()
```

### Diagnostics

#### machbase.checkBit()

```python
#!/usr/bin/env python3
from machbaseAPI.machbaseAPI import machbase

def main():
    db = machbase()
    print('client pointer width:', db.checkBit())

if __name__ == '__main__':
    main()
```

### Low-level bindings

The lower-level `machbaseAPI` class exposes the raw ctypes bindings and the helper `get_library_path()`.

```python
#!/usr/bin/env python3
from machbaseAPI import machbaseAPI
from machbaseAPI.machbaseAPI import get_library_path

def main():
    print('native library path:', get_library_path())
    api = machbaseAPI.machbaseAPI()
    print('openDB argtypes:', api.clib.openDB.argtypes)

if __name__ == '__main__':
    main()
```

Use the low-level surface only when you need direct access to the C layer; the `machbase` helper covers day-to-day database tasks.
