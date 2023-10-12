
# User Interface API

All user interface API are authorizing client with JWT based authentication.

## LOGIN

### POST /web/api/login

- `LoginReq`
```json
{
    "loginName": "id",
    "password": "password"
}
```

- `LoginRsp`
```json
{
    "success": true,
    "accessToken": "jwt access token",
    "refreshToken": "jwt refresh token",
    "reason": "string",
    "elapse": "string",
    "server": {
        "version": "v2.0.0"
    }
}
```

### POST /web/api/relogin

- `ReLoginReq`

```json
{
    "refreshToken": "refresh token that was issued by 'login'"
}
```

- `ReLoginRsp`

```json
{
    "success": true,
    "accessToken": "jwt access token",
    "refreshToken": "jwt refresh token",
    "reason": "string",
    "elapse": "string",
    "server": {
        "version": "v2.0.0"
    }
}
```

### POST /web/api/logout

- `LogoutReq`:

```json
{
    "refreshToken": "refresh token that was issued by 'login'"
}
```

### GET /web/api/check

- `LoginCheckRsp`
```json
{
    "success": true,
    "reason": "string",
    "elapse": "string",
    "experimentMode": true,
    "recents": [{"ReferenceGroup, omitempty"}],
    "references":[{"ReferenceGroup, omitempty"}],
    "shells": [{"ShellDefinition, omitempty"}]
}
```

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
    "target": "brower link target, omitempty"
}
```

- type: `url`, `wrk`, `tql`, `sql`
- address: if address has prefix `serverfiel://<path>` it points a server side file, 
  otherwise external web url that starts with `https://`


- `ShellDefinition`
```json
{
    "id": "shell definition id (uuid)",
    "type": "type",
    "icon": "icon name, omitempty",
    "label": "display name",
    "theme": "theme name, omitempty",
    "command": "terminal shell command, omitempty",
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

## SHELL & TERMINAL

### ws:///web/api/term/:term_id/data

Web socket for terminal

### POST /web/api/term/:term_id/windowsize

Change terminal size

`TerminalSize`

```json
{
    "rows": 24,
    "cols": 80
}
```

### GET /web/api/shell/:id

Returns `ShellDefinition` for the given id

### POST /web/api/shell/:id

Update the `ShellDefinition` of the given id

### GET /web/api/shell/:id/copy

Returns `ShellDefinition` for a new copy of the shell of the given id

### DELETE /web/api/shell/:id

Delete the shell of the given id

```json
{
    "success": true,
    "reason": "success of error message",
    "elapse": "time represents in text"
}
```

## SERVER EVENTS

### ws:/web/api/console/:console_id/data

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
|                | `ping.tick`      | any integer number, server will repond with the same number that client sends |
| `log`          | `log.level`      | log level `TRACE`, `DEBUG`, `INFO`, `WARN`, `ERROR`|
|                | `log.message`    | log message        |


## TQL & Worksapce

**Content-types of TQL**

| Header <br/>`Content-Type` | Header <br/>`X-Chart-Type` |          Content            |
|:--------------------------:| :-------------------------:| :-------------------------- |
| text/html                  | "echart"                   | Full HTML (echart) <br/>ex) It may be inside of `<iframe>`|
| text/html                  | -                          | Full HTML <br/>ex) It may be inside of `<iframe>` |
| text/csv                   | -                          | CSV                         |
| text/markdown              | -                          | Markdown                    |
| application/json           | "echart"                   | JSON (echart data)          |
| application/json           | -                          | JSON                        |
| application/xhtml+xml      | -                          | HTML Element, ex) `<div>...</div>` |


### GET /web/api/tql/*path

Run the tql of the path, refer the section of 'Content-types of TQL' for the response

### POST /web/api/tql/*path

Run the tql of the path, refer the section of 'Content-types of TQL' for the response

### POST /web/api/tql

Post tql script as content payload, server will response the execution result.
refer the section of 'Content-types of TQL' for the response

### POST /web/api/md

Post markdown as content payload, sever will response the rendering result in xhtml

## GET,POST /web/machbase

It works as same as `/db/query` API, the difference is the way of authentication.
The `/db/query` authorize the client by API Token (for client applications).
But `/web/machbase` checks JWT (for human interactions).


## GET /web/api/tables?showall=false

Return table list

- `showall` returns includes all hidden tables if set `true`

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

## GET /web/api/tables/:table/tags?name=prefix

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

## GET /web/api/tables/:table/:tag/stat

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

## GET,POST /web/api/files/*path

- file types and content-type

| file type | Content-Type             |
|:----------|:-------------------------|
| .sql      | text/plain               |
| .tql      | text/plain               |
| .taz      | application/json         |
| .wrk      | application/json         |
| unknown   | application/octet-stream |

### GET /web/api/files/*path

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

### POST /web/api/files/*path

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

### PUT /web/api/files/*path

Rename(move) a file (or a directory).

`RenameReq`

```json
{
    "destination": "target path",
}
```

This api returns status code `200 OK` if the operation has done successfully.


### DELETE /web/api/files/*path

Delete the file at the `path`, if the path is pointing a directory and is not empty, it will return error.


## GET /web/api/license

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

## POST /web/api/license

Install license file

## Deprecated

- GET,POST /web/api/chart
