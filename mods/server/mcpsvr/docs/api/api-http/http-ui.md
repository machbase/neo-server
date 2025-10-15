# Machbase Neo HTTP User Interface API

These user interface API validates the requests from clients with JWT based authentication.

## User Authenticate

### Login

**POST `/web/api/login`**

**Request:**
```json
{
    "loginName": "sys",
    "password": "manager"
}
```

**Response:**
```json
{
    "success": true,
    "accessToken": "jwt-access-token",
    "refreshToken": "jwt-refresh-token",
    "reason": "string",
    "elapse": "string",
    "server": { "version": "v1.2.3" }
}
```

### Refresh token

**POST `/web/api/relogin`**

**Request:**
```json
{
    "refreshToken": "refresh token that was issued with 'login'"
}
```

**Response:**
```json
{
    "success": true,
    "accessToken": "jwt access token",
    "refreshToken": "jwt refresh token",
    "reason": "string",
    "elapse": "string",
    "server": { "version": "v1.2.3" }
}
```

### Logout

**POST `/web/api/logout`**

- `LogoutReq`:

```json
{
    "refreshToken": "refresh token that was issued with 'login'"
}
```

### Status

**GET `/web/api/check`**

Validates current  token status.

- `LoginCheckRsp`
```json
{
    "success": true,
    "reason": "string",
    "elapse": "string",
    "experimentMode": false,
    "server": {"version":"v1.2.3"},
    "shells": [{"ShellDefinition"}]
}
```

- `ShellDefinition`
```json
{
    "id": "shell definition id (uuid)",
    "type": "type",
    "icon": "icon name",
    "label": "display name",
    "theme": "theme name",
    "command": "terminal shell command",
    "attributes": [
        { "removable": true },
        { "cloneable": true },
        { "editable": true }
    ]
}
```

- types

| type | description      |
|:-----| :------------    |
| sql  | sql editor       |
| tql  | tql editor       |
| wrk  | workspace editor |
| taz  | tag analyzer     |
| term | terminal         |

## Database

### Execute SQL

**GET,POST `/web/machbase`**

It works as same as `/db/query` API, the only difference is the way of authentication.
The `/db/query` authorize the client applications by API Token, while `/web/machbase` validates JWT for user interactions.

### List tables

**GET `/web/api/tables?showall=false&name=pattern`**

Return table list

- `showall` returns includes all hidden tables if set `true`
- `name` table name filtering pattern, the pattern can be a glob (includes `?` or `*`) or prefix (which has no `?` and `*`)

```json
{
    "success": true,
    "reason": "success or other message",
    "elapse": "elapse time in string format",
    "data": {
        "columns": ["ROWNUM", "DB", "USER", "NAME", "TYPE"],
        "types": ["int32", "string", "string", "string", "string"],
        "rows":[
            [1, "MACHBASE", "SYS", "TABLENAME", "TAG TABLE"],
        ]
    }
}
```

### List tags

**GET `/web/api/tables/:table/tags?name=prefix`**

Returns tag list of the table

- `name` returns only tags those name starts with the given prefix

```json
{
    "success": true,
    "reason": "success or other message",
    "elapse": "elapse time in string format",
    "data": {
        "columns": ["ROWNUM", "NAME"],
        "types": ["int32", "string"],
        "rows":[
            [1, "temperature"],
        ]
    }
}
```

### Tag stat

**GET `/web/api/tables/:table/:tag/stat`**

Returns the stat of tag of the table

```json
{
    "success": true,
    "reason": "success or other message",
    "elapse": "elapse time in string format",
    "data": {
        "columns": ["ROWNUM", "NAME", "ROW_COUNT", "MIN_TIME", "MAX_TIME",
			"MIN_VALUE", "MIN_VALUE_TIME", "MAX_VALUE", "MAX_VALUE_TIME", "RECENT_ROW_TIME"],
        "types": ["int32", "string", "int64", "datetime", "datetime","double", 
            "datetime", "double", , "datetime",, "datetime"],
        "rows":[
            ["...omit...."],
        ]
    }
}
```

## Shell & Terminal

### Data channel

**`ws:///web/api/term/:term_id/data`**

Web socket for terminal

### Window size

**POST `/web/api/term/:term_id/windowsize`**

Change terminal size

`TerminalSize`

```json
{ "rows": 24, "cols": 80 }
```

### Get Shell Definition

**GET `/web/api/shell/:id`**

Returns `ShellDefinition` for the given id

### Update Shell Definition

**POST `/web/api/shell/:id`**

Update the `ShellDefinition` of the given id

### Make copy of Shell

**GET `/web/api/shell/:id/copy`**

Returns `ShellDefinition` for a new copy of the shell of the given id

### Delete Shell Definition 

**DELETE `/web/api/shell/:id`**

Delete the shell of the given id

```json
{
    "success": true,
    "reason": "success of error message",
    "elapse": "time represents in text"
}
```

## Server events

### Event channel

**`ws:/web/api/console/:console_id/data`**

Web socket for the bi-directional messages

- message type

```json
{
    "type": "type(see below)",
    "ping": {
        "tick": 1234
    },
    "log": {
        "level": "INFO",
        "message": "log message"
    }
}
```

| type           |  fields          | description        |
|:---------------| :----------------| :------------------|
| `ping`         |                  | ping message       |
|                | `ping.tick`      | any integer number, server will respond with the same number that client sends |
| `log`          | `log.level`      | log level `TRACE`, `DEBUG`, `INFO`, `WARN`, `ERROR`|
|                | `log.message`    | log message        |
|                | `log.repeat`     | count, if the same message repeats more than two times in serial |


## TQL & Workspace

**Content-types of TQL**

| Header <br/>`Content-Type` | Header <br/>`X-Chart-Type` |          Content            |
|:--------------------------:| :-------------------------:| :-------------------------- |
| text/html                  | "echart", "geomap"         | Full HTML <br/>ex) It may be inside of `<iframe>`|
| text/html                  | -                          | Full HTML <br/>ex) It may be inside of `<iframe>` |
| text/csv                   | -                          | CSV                         |
| text/markdown              | -                          | Markdown                    |
| application/json           | "echart", "geomap"         | JSON (echart or geomap data)|
| application/json           | -                          | JSON                        |
| application/xhtml+xml      | -                          | HTML Element, ex) `<div>...</div>` |


### Run tql file

**GET `/web/api/tql/*path`**

Run the tql of the path, refer the section of 'Content-types of TQL' for the response

**POST `/web/api/tql/*path`**

Run the tql of the path, refer the section of 'Content-types of TQL' for the response

### Run tql script

**POST `/web/api/tql`**

Post tql script as content payload, server will response the execution result.
refer the section of 'Content-types of TQL' for the response.

If the request has a `$` named query parameter, it will be taken as the tql script,
and the payload will be treated as data. This `$` query parameter is available since v8.0.17.

### Markdown rendering

**POST `/web/api/md`**

Post markdown as content payload, sever will response the rendering result in xhtml

## File management

### Content-Type

File types and content-type

| file type | Content-Type             |
|:----------|:-------------------------|
| .sql      | text/plain               |
| .tql      | text/plain               |
| .taz      | application/json         |
| .wrk      | application/json         |
| unknown   | application/octet-stream |

### Read file

**GET `/web/api/files/*path`**

Returns the content of the file if the path is pointing a file.

Returns Dir entries if the path is pointing a directory.

- `Entry`

```json
{
    "isDir": true,
    "name": "name",
    "content": "bytes array, if the entry is a file",
    "children": [{"SubEntry, if the entry is a directory"}],
}
```

- `SubEntry`

```json
{
    "isDir": true,
    "name": "name",
    "type": "type",
    "size": 1234,
    "lastModifiedUnixMillis": 169384757
}
```

### Write file

**POST `/web/api/files/*path`**

- if the `path` points a file, it will write the payload content into the file.

- if the `path` is a directory and request with no content, it will create a empty directory.
  and returns the `Entry` of the directory

- if the `path` is a directory and payload is json of `GitCloneReq`,
  it will clone the remote git repository to the `path` and returns `Entry` of the directory.

`GitCloneReq`

```json
{
    "command": "clone",
    "url": "https://github.com/machbase/neo-samples.git"
}
```

- `command` : `clone`, `pull`

### Rename/move file

**PUT `/web/api/files/*path`**

Rename(move) a file (or a directory).

`RenameReq`

```json
{
    "destination": "target path",
}
```

This api returns status code `200 OK` if the operation has done successfully.


### Remove file

**DELETE `/web/api/files/*path`**

Delete the file at the `path`, if the path is pointing a directory and is not empty, it will return error.

## Key management

### List Key

**GET `/web/api/keys/:id`**

Return key info list

`response`

```json
{
    "success": true,
    "reason": "success",
    "data": [
        {
            "idx": 0,
            "id": "eleven",
            "notBefore": 1713171461,
            "notAfter": 2028531461
        }
    ],
    "elapse": "131.9µs"
}
```
### Generate Key

**POST `/web/api/keys`**

generate key
- `name` is required
- `notAfter` is expiration date

**Request:**
```json
{
    "name": "eleven",
    "notBefore": 0,
    "notAfter": 0
}
```

**Response:**
```json
{
    "success": true,
    "reason": "success",
    "elapse": "5.4961ms",
    "certificate": "-----BEGIN CERTIFICATE-----\nXXXXXXXXXXXXXXXXXX\n-----END CERTIFICATE-----\n",
    "privateKey": "-----BEGIN EC PRIVATE KEY-----\nXXXXXXXXXXXXXXXX\n-----END EC PRIVATE KEY-----\n",
    "token": "eleven:b:XXXXXXXXXXXXXXXXX"
}
```

### Delete Key

**DELETE `/web/api/keys/:id`**

Delete the key of the given id

`response`

```json
{
    "success": true,
    "reason": "success",
    "elapse": "112.8µs"
}
```

## Ssh Key

### List Ssh Key

**GET `/web/api/sshkeys`**

Return ssh-key info list

`response`

```json
{
    "data": [
        {
            "keyType": "ssh-rsa",
            "fingerprint": "f08h89fhf0dkv0v0v9c9x0cx9v9",
            "comment": "example@machbase.com"
        }
    ],
    "elapse": "67.6µs",
    "reason": "success",
    "success": true
}
```

### Generate Ssh Key

**POST `/web/api/sshkeys`**

**Use public key authentication with SSH**   

Adding the public key to machbase-neo server makes it possible to execute any `machbase-neo shell` command without prompt and entering password.

**Request:**
```json
{
    "key": "your publickey"
}
```

**Response:**
```json
{
    "elapse": "138.801µs",
    "reason": "success",
    "success": true
}
```

### Delete Ssh Key

**DELETE `/web/api/sshkeys/:fingerprint`**

Delete the ssh-key of the given fingerprint   

`response`
```json
{
    "elapse": "198.8µs",
    "reason": "success",
    "success": true
}
```

## Timer

### Get Timer

**GET `/web/api/timers/:name`**

Return timer info

- state: `RUNNING`, `STARTING`, `STOP`, `STOPPING`,`FAILED`, `UNKNWON`

`response`

```json
{
    "success": true,
    "reason": "success",
    "data": [
        {
            "name": "ELEVEN",
            "type": "TIMER",
            "state": "STOP", 
            "task": "timer.tql",
            "schedule": "0 30 * * * *"
        }
    ],
    "elapse": "92.1µs"
}
```

### List Timer

**GET `/web/api/timers`**

Return timer info list
- state: `RUNNING`, `STARTING`, `STOP`, `STOPPING`,`FAILED`, `UNKNWON`

`response`

```json
{
    "success": true,
    "reason": "success",
    "data": [
        {
            "name": "ELEVEN",
            "type": "TIMER",
            "state": "STOP",
            "task": "timer.tql",
            "schedule": "0 30 * * * *"
        },
        {
            "name": "TWELVE",
            "type": "TIMER",
            "state": "RUNNING",
            "task": "timer2.tql",
            "schedule": "1 30 * * * *"
        }
    ],
    "elapse": "92.1µs"
}
```

### Add Timer

**POST `/web/api/timers`**

Add Timer
- `name`, `autoStart`, `schedule`, `path` is required  

Timer `schedule`
- `0 30 * * * *`           Every hour on the half hour
- `@every 1h30m`           Every hour thirty
- `@daily`                 Every day

**Request:**
```json
{
    "name":"eleven",
    "autoStart":false,
    "schedule":"@every 10s",
    "path":"timer.tql"
}
```

**Response:**
```json
{
    "success": true,
    "reason": "success",
    "elapse": "4.9658ms"
}
```

### Start Timer

**POST `/web/api/timers/:name/state`**

Start Timer
- `state` is required

**Request:**
```json
{
    "state":"start",
}
```

**Response:**
```json
{
    "success": true,
    "reason": "success",
    "elapse": "822.601µs"
}
```

### Stop Timer

**POST `/web/api/timers/:name/state`**

Stop Timer
- `state` is required

**Request:**
```json
{
    "state":"stop",
}
```

**Response:**
```json
{
    "success": true,
    "reason": "success",
    "elapse": "26.2µs"
}
```

### Update Timer

**PUT `/web/api/timers/:name`**

Update Timer
- `autoStart`, `schedule`, `path` 

**Request:**
```json
{
    "audoStart" : true,
    "schedule":"@every 5s",
    "path":"timer.tql"
}
```

**Response:**
```json
{
    "elapse": "459.6µs",
    "reason": "success",
    "success": true
}
```

### Delete Timer

**DELETE `/web/api/timers/:name`**

Delete Timer

`Response`
```json
{
    "success": true,
    "reason": "success",
    "elapse": "4.8664ms"
}
```

## Bridge

### List Bridge

**GET `/web/api/bridges`**

Return bridge info list

`response`

```json
{
    "success": true,
    "reason": "success",
    "data": [
        {
            "name": "pg",
            "type": "postgres",
            "path": "host=127.0.0.1 port=5432 user=postgres password=1234 dbname=bridgedb sslmode=disable"
        }
    ],
    "elapse": "1.328301ms"
}
```
### Add Bridge

**POST `/web/api/bridges`**

Add Bridge
- `name`, `type`, `path` is required
- supported bridges `SQLite`, `PostgreSql`, `Mysql`, `MSSQL`, `MQTT`

**Request:**
```json
{
    "name":"pg",
    "type":"postgres", // sqlite, postgres, mysql, mssql, mqtt
    "path":"host=127.0.0.1 port=5432 user=postgres password=1234 dbname=bridgedb sslmode=disable"
}
```

**Response:**
```json
{
    "success": true,
    "reason": "success",
    "elapse": "193.499µs"
}
```

### Exec Bridge

**POST `/web/api/bridges/:name/state`**

Exec Bridge
- `state`, `command` is required

**Request:**
```json
{
    "state":"exec",
    "command":"CREATE TABLE IF NOT EXISTS pg_example(id SERIAL PRIMARY KEY,company VARCHAR(50) UNIQUE NOT NULL,employee  INT,discount REAL,plan FLOAT(8),code UUID,valid BOOL, memo TEXT, created_on TIMESTAMP NOT NULL)"
}
```

**Response:**
```json
{
    "success": true,
    "reason": "success",
    "elapse": "217.4µs"
}
```

### Query Bridge

**POST `/web/api/bridges/:name/state`**

Query Bridge
- `state`, `command` is required

**Request:**
```json
{
    "state":"query",
    "command":"select * from pg_example"
}
```

**Response:**
```json
{
    "success": true,
    "reason": "success",
    "column": [
        "id",
        "company",
        "employee",
        "discount",
        "plan",
        "code",
        "valid",
        "memo",
        "created_on"
    ],
    "rows": [
        [
            2,
            "test-company",
            10,
            1.234,
            2.3456,
            "c2d29867-3d0b-d497-9191-18a9d8ee7830",
            true,
            "test memo",
            "2023-08-09T14:20:00+09:00"
        ],
        [
            3,
            "test-company2",
            10,
            1.234,
            2.3456,
            null,
            null,
            null,
            "2023-08-09T14:20:00+09:00"
        ]
    ],
    "elapse": "53.015905ms"
}
```

### Test Bridge

**POST `/web/api/bridges/:name/state`**

Test Bridge

**Request:**
```json
{
    "state":"test",
}
```

**Response:**
```json
{
    "success": true,
    "reason": "success",
    "elapse": "331.1µs"
}
```

### Delete Bridge

**DELETE `/web/api/bridges/:name`**

Delete the bridge of the given name

`response`

```json
{
    "success": true,
    "reason": "success",
    "elapse": "112.8µs"
}
```

## Subscriber

### Get Subscriber

**GET `/web/api/subscribers/:name`**

Return subscriber info
- state: `RUNNING`, `STARTING`, `STOP`, `STOPPING`,`FAILED`, `UNKNWON`
- `autoStart`, `queue`, `Qos` field is omitempty

`response`

```json
{
    "data": [
        {
            "name": "NATS_SUBR",
            "type": "SUBSCRIBER",
            "autoStart": true,  // omitempty
            "state": "RUNNING", 
            "task": "db/append/EXAMPLE:csv",
            "bridge": "my_nats",
            "topic": "iot.sensor",
            "queue":"", // omitempty
            "QoS":""    // omitempty
        }
    ],
    "elapse": "253.4µs",
    "reason": "success",
    "success": true
}
```

### List Subscriber

**GET `/web/api/subscribers`**

Return subscriber info list
- state: `RUNNING`, `STARTING`, `STOP`, `STOPPING`,`FAILED`, `UNKNWON`
- `autoStart`, `queue`, `Qos` field is omitempty

`response`

```json
{
    "data": [
        {
            "name": "NATS_SUBR",
            "type": "SUBSCRIBER",
            "autoStart": true,  // omitempty
            "state": "RUNNING",
            "task": "db/append/EXAMPLE:csv",
            "bridge": "my_nats",
            "topic": "iot.sensor",
            "queue":"", // omitempty
            "QoS":""    // omitempty
        },
        {
            "name": "NATS_SUBR2",
            "type": "SUBSCRIBER",
            "autoStart": true,  // omitempty
            "state": "STARTING",
            "task": "db/insert/EXAMPLE2:csv",
            "bridge": "my_nats2",
            "topic": "iot.sensor2",
            "queue":"", // omitempty
            "QoS":""    // omitempty
        }
    ],
    "elapse": "253.4µs",
    "reason": "success",
    "success": true
}
```

### Add Subscribers

**POST `/web/api/subscribers`**

Add Subscriber   
- `autostart`:   '--autostart' makes the subscriber starts along with machbase-neo starts. Ommit this to start/stop manually.   
- `name` 'nats_subr' the name of the subscriber.   
- `bridge` 'my_nats' the name of the bridge that the subscriber is going to use.   
- `topic` 'iot.sensor' subject name to subscribe. it should be in NATS subject syntax.   
- `task` 'db/append/EXAMPLE:csv' writing descriptor, it means the incoming data is in CSV format and writing data into the table EXAMPLE in append mode.   
- `autostart` makes the subscriber will start automatically when machbase-neo starts. If the subscriber is not autostart mode, you can make it start and stop manually by subscriber start <name> and subscriber stop <name> commands.
- `QoS` <int> if the bridge is MQTT type, it specifies the QoS level of the subscription to the topic. It supports 0, 1 and the default is 0 if it is not specified.
- `queue` <string> if the bridge is NATS type, it specifies the Queue Group.

nats-bridge manual https://docs.machbase.com/neo/bridges/31.nats/

**Request:**
```json
{
    "name":"nats_subr",
    "autoStart":true,
    "bridge":"my_nats",
    "topic":"iot.sensor",
    "task":"db/append/EXAMPLE:csv",
    "QoS": "",  // mqtt bridge option 0 or 1 ( default 0 )
    "queue": "" // nats birdge option
}
```

**Response:**
```json
{
    "elapse": "260µs",
    "reason": "success",
    "success": true
}
```

### Start Subscriber

**POST `/web/api/subscribers/:name/state`**

- `state` is required

**Request:**
```json
{
    "state":"start",
}
```

**Response:**
```json
{
    "elapse": "166.1µs",
    "reason": "success",
    "success": true
}
```

### Stop Subscriber

**POST `/web/api/subscribers/:name/state`**

- `state` is required

**Request:**
```json
{
    "state":"stop",
}
```

**Response:**
```json
{
    "elapse": "54.2µs",
    "reason": "success",
    "success": true
}
```

### Delete Subscriber

**DELETE `/web/api/subscribers/:name`**

Delete the subscriber of the given name

`response`

```json
{
    "elapse": "77.1µs",
    "reason": "success",
    "success": true
}
```

## Backup

### Get Backup

**GET `/web/api/backup/archives`**

Return backup list
- default backup dir `$MACHBASE_HOME/dbs/backup`
- machbase-neo serve `--backup-dir={path}` required

`response`

```json
{
    "data": [
        {
            "path": "example_backup1",
            "isMount": true,
            "mountName": "backup1"
        },
        {
            "path": "example_backup2",
            "isMount": false
        }
    ],
    "elapse": "6.562299ms",
    "reason": "success",
    "success": true
}
```

### DB Backup

**POST `/web/api/backup/archive`**

backup database</br>
- **Full backup**:   Backup of entire data
- **Incremental backup**:   Backup of the data added after the full or previous incremental backup
- **Time Duration backup**:   Backup of data for a specific period

`request`

**Full Backup:**
```json
{
    "type":"database", // database or table
    "tableName":"",
    "duration":{
        "type":"full",
        "after":"",
        "from":"",
        "to":""
    },
    "path":"example_backup1" 
    // "path":"/home/neo/backups/example_backup1" 
}
```

**Incremental Backup:**
```json
{
    "type":"database", // database or table
    "tableName":"",
    "duration":{
        "type":"incremental",
        "after":"{previous_backup_dir}",
        "from":"",
        "to":""
    },
    "path":"example_backup1" 
    // "path":"/home/neo/backups/example_backup1" 
}
```

**Time Backup:**
```json
{
    "type":"database", // database or table
    "tableName":"",
    "duration":{
        "type":"time",
        "after":"",
        "from":"2024-08-01 00:00:00",
        "to":"2024-08-02 23:59:59"
    },
    "path":"example_backup1" 
    // "path":"/home/neo/backups/example_backup1" 
}
```

**Table Backup:**
```json
{
    "type":"table", // database or table
    "tableName":"example",
    "duration":{
        "type":"full",
        "after":"",
        "from":"",
        "to":""
    },
    "path":"example_backup1" 
    // "path":"/home/neo/backups/example_backup1" 
}
```

`response`
```json
{
    "success": true,
    "reason": "success",
    "elapse": "231.3µs"
}
```

### Status Backup

**GET `/web/api/backup/archive/status`**

return backup status</br>

`response`
```json
{
    "data": {
        "type": "database",
        "tableName": "",
        "duration": {
            "type": "full",
            "after": "",
            "from": "",
            "to": ""
        },
        "path": "/home/neo/neo-server/tmp/machbase_home/dbs/example_backup1",
    },
    "elapse": "1.1µs",
    "reason": "success",
    "success": true
}
```

## Mount

### Mount List

**GET `/web/api/backup/mounts`**

Return mount list

`response`

```json
{
    "data": [
        {
            "name": "machbase_backup_19700101090000_20240726104832_15",
            "path": "backup1",
            "tbsid": 23,
            "scn": 15,
            "mountdb": "MOUNT_BACKUP1",
            "dbBeginTime": "1970-01-01 09:00:00",
            "dbEndTime": "2024-07-26 10:48:32",
            "backupBeginTime": "2024-07-26 10:48:32",
            "backupEndTime": "2024-07-26 10:48:34",
            "flag": 0
        }
    ],
    "elapse": "424.3µs",
    "reason": "success",
    "success": true
}
```

### DB Mount

**POST `/web/api/backup/mounts/:name`**

database mount
- `:name` mount name
- `path` backup database path (`Absolute Path`, `Relative Path` available)

**Request:**
```json
{
    "path":"example_backup1" // Relative Path
    // "path":"/home/machbase/machbase_home/dbs/example_backup1" // Absolute Path
}
```

**Response:**
```json
{
    "elapse": "46.8694ms",
    "reason": "success",
    "success": true
}
```

### DB Unmount

**DELETE `/web/api/backup/mounts/:name`**

database unmount
- `:name` unmount name

`response`
```json
{
    "elapse": "46.8694ms",
    "reason": "success",
    "success": true
}
```

## Package

### Search

**GET `/web/api/pkgs/search?name=pkg_name&possibles=10`**

`Query Parameter`
 - `name` is package name, required
 - `possible` is search count ( possible=0, all search )


`response`

```json
{
    "success": true,
    "reason": "success",
    "data":{},
    "elapse": "547.1µs"
}
```
### Sync

**GET `/web/api/pkgs/sync`**

Package sync 

`response`

```json
{
    "success": true,
    "reason": "success",
    "elapse": "30.9144ms"
}
```

### Install

**GET `/web/api/pkgs/insall/:name`**

 - `:name` is install package name, required

`response`
```json
{
    "success": true,
    "reason": "success",
    "data":{}, // omitempty
    "log":"",
    "elapse": "23.1491ms"
}
```

### Uninstall

**GET `/web/api/pkgs/uninsall/:name`**

 - `:name` is uninstall package name, required

`response`
```json
{
    "success": true,
    "reason": "success",
    "data":{}, // omitempty
    "log":"",
    "elapse": "88.4133ms"
}
```

## Others

### References

**GET `/web/api/refs/*path`**

- `ReferenceGroup`
```json
{
    "label": "group name",
    "items":[{"ReferenceItem"}]
}
```

- `ReferenceItem`
```json
{
    "type": "type",
    "title": "display title",
    "address": "url address",
    "target": "browser link target"
}
```

- type: `url`, `wrk`, `tql`, `sql`
- address: if address has prefix `serverfile://<path>` it points a server side file, 
  otherwise external web url that starts with `https://`


### SQL statements splitter

**POST `/web/api/splitter/sql`**

```json
{
    "success": true,
    "reason": "success or error reason",
    "elapse": "elapse time",
    "data": {
        "statements": [
            {
                "text": "-- env: bridge=sqlite",
                "beginLine": 1,
                "endLine": 1,
                "isComment": true,
                "env": {
                    "bridge": "sqlite",
                    "error": "if there are syntax error in `-- env: bridge=database`"
                }
            },
            {
                "text": "select * from table",
                "beginLine": 2,
                "endLine": 2,
                "isComment": false,
                "env": {
                    "bridge": "sqlite",
                    "error": "if there are syntax error in `-- env: bridge=database`"
                }
            },
            {
                "text": "-- comment",
                "beginLine": 3,
                "endLine": 3,
                "isComment": true,
                "env": {}
            }
        ]
    }
}
```

### License info

**GET `/web/api/license`**

```json
{
    "success": true,
    "reason": "success or error reason",
    "elapse": "elapse time",
    "data": {
        "id": "license id",
        "type": "type",
        "customer": "customer",
        "project": "project",
        "countryCode": "country code",
        "installDate": "installation date",
        "issueDate": "license issue date"
    }
}
```

### License install

**POST `/web/api/license`**

Install license file