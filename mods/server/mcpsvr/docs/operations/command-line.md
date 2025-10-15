# Machbase Neo Command line

## machbase-neo serve

Start machbase-neo server process.

### Flags

**General flags**
             
| flag             | desc                                                              |
|:-----------------|:----------------------------------------------------------------- |
| `--host`         | listening network addr (default: `127.0.0.1`) ex) `--host 0.0.0.0`                  |
| `-c`, `--config` | config file location ex) `--config /data/machbase-neo.conf`|
| `--pid`          | file path to save pid ex) `--pid /data/machbase-neo.pid`    |
| `--data`         | path to database (default: `./machbase_home`) ex) `--data /data/machbase`                 |
| `--file`         | path to files (default: `.`) ex) `--file /data/files`                       |
| `--backup-dir`   | path to the backup dir (default: `./backups`) ex) `--backup-dir /data/backups` |
| `--pref`         | path to preference directory path (default: `~/.config/machbase`)                                |
| `--preset`       | database preset `auto`, `fog`, `edge` (default: `auto`) ex) `--preset edge`    |

**Database Sessions flags**

Conceptually, if we divide machbase-neo into the API part (http, mqtt, etc.) that includes tql and the DBMS part, there have been no restrictions on traffic between the API and DBMS so far.

If 100 MQTT clients and 100 HTTP clients, a total of 200 clients, execute a db query "simultaneously", 200 sessions will be executed in the DBMS. If there is a tool that can control the traffic flow delivered to the DBMS, it would be possible to configure flexibly depending on the situation. Therefore, new flags that can be used in `machbase-neo serve` have been added.

| flag                     | desc                                                              |
|:-------------------------|:----------------------------------------------------------------- |
| `--max-open-conn`        | `< 0` : unlimited (default), `0` : `= CPU_count * factor`, `> 0` : specified max open connections |
| `--max-open-conn-factor` | used to calculate the number of the max open connections when `--max-open-conn` is 0. (default `2`). |
| `--max-open-query`       | `< 0` : unlimited, `0` (default) : `= CPU_count * factor`,  `> 0` : specified max open query iterations |
| `--max-open-query-factor`| used to calculate the number of the max open query iterations when `--max-open-query` is 0. (default `2`) |

- *--mach-open-conn* controls the number of connections (=sessions) that can be OPENED "simultaneously" between the API and DBMS. If this number is exceeded, it will wait at the API level.
  - `< 0` If a negative number (e.g., -1) is set, it operates without restrictions as in previous versions.
  - `0` If no settings are made by default, it is calculated as `the number of CPU cores * max-open-conn-factor`. The default max-open-conn-factor is 1.5.
  - `> 0` If a positive number is set, it operates according to the set value.

This setting value can be checked as a command `session limit` within `machbase-neo shell` and can be changed with `session limit --set=<num>`. Since the change is maintained only while the process is running, the startup script must be modified to change it permanently.

- *--mach-open-conn-factor* sets the factor value to calculate as `the number of CPU cores * factor` when `--mach-open-conn` described above is 0 (default). This value must be 0 or higher, and if it is 0 or negative, the default `2.0` is applied.

As example, if the number of CPU cores is 8 and the factor is 2.0, the open limit becomes 16, and if it is 0.5, the open limit becomes 4. If none of the two options described above are given, the default factor 2.0 is applied, and the open limit becomes 16.

**Http flags**

| flag                    | default     | desc                                                                      |
|:------------------------|:------------|:------------------------------------------------------------------------- |
| `--http-linger`         | `-1`        | HTTP socket option, `-1` means disable SO_LINGER, `>=0` means set SO_LINGER |
| `--http-readbuf-size`   | `0`         | HTTP socket read buffer size. `0` means use system default.                 |
| `--http-writebuf-size`  | `0`         | HTTP socket write buffer size. `0` means use system default.                |
| `--http-debug`          | `false`     | Enable HTTP Ddebug log                                                      |
| `--http-debug-latency`  | `"0"`       | Log HTTP requests that take longer than the specified duration to respond (e.g., "3s"). "0" means all request. |
| `--http-allow-statz`    |             | Allow source IPs (comma separated) to access `/db/statz` API. default allows only `127.0.0.1`. |

**Log flags**

| flag                    | default     | desc                                                                      |
|:------------------------|:------------|:------------------------------------------------------------------------- |
| `--log-filename`        | `-` (stdout)| log file path ex) `--log-filename /data/logs/machbase-neo.log`       |
| `--log-level`           | `INFO`      | log level. TRACE, DEBUG, INFO, WARN, ERROR ex) `--log-level INFO`    |
| `--log-append`          | `true`      | append existing log file.                   |
| `--log-rotate-schedule` | `@midnight` | time scheduled log file rotation            |
| `--log-max-size`        | `10`        | file max size in MB                         |
| `--log-max-backups`     | `1`         | maximum log file backups                    |
| `--log-max-age`         | `7`         | maximum days in backup files                | 
| `--log-compress`        | `false`     | gzip compress the backup files              |
| `--log-time-utc`        | `false`     | use UTC time for logging                    |

**Listener flags**

| flag             | default   | desc                            |
|:-----------------|:----------|-------------------------------- |
| `--shell-port`   | `5652`    | ssh listen port                 |
| `--mqtt-port`    | `5653`    | mqtt listen port                |
| `--mqtt-sock`    | `/tmp/machbase-neo-mqtt.sock`| mqtt unix socket |
| `--http-port`    | `5654`    | http listen port                |
| `--http-sock`    | `/tmp/machbase-neo.sock` | http unix socket |
| `--grpc-port`    | `5655`    | grpc listen port                |
| `--grpc-sock`    | `mach-grpc.sock` | grpc unix domain socket  |
| `--grpc-insecure`| `false`   | set `true` to use plain tcp socket, disable TLS |
| `--mach-port`    | `5656`    | machbase native listen port     |

> **📌 IMPORTANT**  
> Since the default of `--host` is the loopback address, it is not allowed to access machbase-neo from the remote hosts.  
> Set `--host <host-address>` or `--host 0.0.0.0` for accepting the network connections from remote clients.

If execute `machbase-neo serve` with no flags,

```sh
$ machbase-neo serve
```

it is equivalent with

```sh
$ machbase-neo serve --host 127.0.0.1 --data ./machbase_home --file . --preset auto
```

## machbase-neo shell

Start machbase-neo shell. It will start interactive mode shell if there are no other arguments.

**Flags**

| flag (long)       | default                | desc                                                             |
|:------------------|:-----------------------|:-----------------------------------------------------------------|
| `-s`, `--server`  | `tcp://127.0.0.1:5655` | machbase-neo's gRPC address. e.g. `-s unix://./mach-grpc.sock` e.g. `--server tcp://127.0.0.1:5655` |
| `--user`          | `sys`                  | user name. env: `NEOSHELL_USER`         |
| `--password`      | `manager`              | password. env: `NEOSHELL_PASSWORD`      |

When machbase-neo shell starts, it is looking for the user name and password from OS's environment variables `NEOSHELL_USER` and `NEOSHELL_PASSWORD`. Then if the flags `--user` and `--password` are provided, it will override the provided values instead of the environment variables.

### Precedence of username and password

#### Step 1: Command line flags

If `--user`, `--password` is provided? Use the given values

#### Step 2: Environment variables

If `$NEOSHELL_USER` (on windows `%NEOSHELL_USER%`) is set? Use the value as the user name.

If `$NEOSHELL_PASSWORD` (on windows `%NEOSHELL_PASSWORD%`) is set? Use the value as the password.

#### Step 3: Default

None of those are provided? Use default value `sys` and `manager`.

### Practical usage

For the security, use instant environment variables as below example.

```sh
$ NEOSHELL_PASSWORD='my-secret' machbase-neo shell --user sys
```

Be aware when you use `--password` flag, the secret can be exposed by simple `ps` command as like an example below.

```sh
$ machbase-neo shell --user sys --password manager
```

```sh
$ ps -aef |grep machbase-neo
  501 13551  3598   0  9:33AM ttys000    0:00.07 machbase-neo shell --user sys --password manager
```

**Run Query**
  
```sh
machbase-neo» select binary_signature from v$version;
┌────────┬─────────────────────────────────────────────┐
│ ROWNUM │ BINARY_SIGNATURE                            │
├────────┼─────────────────────────────────────────────┤
│      1 │ 8.0.2.develop-LINUX-X86-64-release-standard │
└────────┴─────────────────────────────────────────────┘
a row fetched.
```

**Create Table**

```sh
machbase-neo» create tag table if not exists example (name varchar(20) primary key, time datetime basetime, value double summarized);
executed.
```

**Schema Table**

```sh
machbase-neo» desc example;
┌────────┬───────┬──────────┬────────┐
│ ROWNUM │ NAME  │ TYPE     │ LENGTH │
├────────┼───────┼──────────┼────────┤
│      1 │ NAME  │ varchar  │     20 │
│      2 │ TIME  │ datetime │      8 │
│      3 │ VALUE │ double   │      8 │
└────────┴───────┴──────────┴────────┘
```

**Insert Table**

```sh
machbase-neo» insert into example values('tag0', to_date('2021-08-12'), 100);
a row inserted.
```

**Select Table**

```sh
machbase-neo» select * from example;
┌────────┬──────┬─────────────────────┬───────┐
│ ROWNUM │ NAME │ TIME(LOCAL)         │ VALUE │
├────────┼──────┼─────────────────────┼───────┤
│      1 │ tag0 │ 2021-08-12 00:00:00 │ 100   │
└────────┴──────┴─────────────────────┴───────┘
a row fetched.
```

**Drop Table**

```sh
machbase-neo» drop table example;
executed.
```

### Sub commands

#### explain

Syntax `explain [--full] <sql>`

Shows the execution plan of the sql.

```sh
machbase-neo» explain select * from example where name = 'tag.1';
 PROJECT
  TAG READ (RAW)
   KEYVALUE INDEX SCAN (_EXAMPLE_DATA_0)
    [KEY RANGE]
     * IN ()
   VOLATILE INDEX SCAN (_EXAMPLE_META)
    [KEY RANGE]
     *
```

#### export

```
  export [options] <table>
  arguments:
    table                    table name to read
  options:
    -o,--output <file>       output file (default:'-' stdout)
    -f,--format <format>     output format
                csv          csv format (default)
                json         json format
       --compress <method>   compression method [gzip] (default is not compressed)
       --[no-]heading        print header message (default:false)
       --[no-]footer         print footer message (default:false)
    -d,--delimiter           csv delimiter (default:',')
       --tz                  timezone for handling datetime
    -t,--timeformat          time format [ns|ms|s|<timeformat>] (default:'ns')
                             consult "help timeformat"
    -p,--precision <int>     set precision of float value to force round
```

#### import

```
  import [options] <table>
  arguments:
    table                 table name to write
  options:
    -i,--input <file>     input file, (default: '-' stdin)
    -f,--format <fmt>     file format [csv] (default:'csv')
       --compress <alg>   input data is compressed in <alg> (support:gzip)
       --no-header        there is no header, do not skip first line (default)
	   --charset          set character encoding, if input is not UTF-8
       --header           first line is header, skip it
       --method           write method [insert|append] (default:'insert')
       --create-table     create table if it doesn't exist (default:false)
       --truncate-table   truncate table ahead importing new data (default:false)
    -d,--delimiter        csv delimiter (default:',')
       --tz               timezone for handling datetime
    -t,--timeformat       time format [ns|ms|s|<timeformat>] (default:'ns')
                          consult "help timeformat"
       --eof <string>     specify eof line, use any string matches [a-zA-Z0-9]+ (default: '.')
```

#### show info

Display the server information.

```sh
machbase-neo» show info;
┌────────────────────┬─────────────────────────────┐
│ NAME               │ VALUE                       │
├────────────────────┼─────────────────────────────┤
│ build.version      │ v2.0.0                      │
│ build.hash         │ #c953293f                   │
│ build.timestamp    │ 2023-08-29T08:08:00         │
│ build.engine       │ static_standard_linux_amd64 │
│ runtime.os         │ linux                       │
│ runtime.arch       │ amd64                       │
│ runtime.pid        │ 57814                       │
│ runtime.uptime     │ 2h 30m 57s                  │
│ runtime.goroutines │ 45                          │
│ mem.sys            │ 32.6 MB                     │
│ mem.heap.sys       │ 19.0 MB                     │
│ mem.heap.alloc     │ 9.7 MB                      │
│ mem.heap.in-use    │ 13.0 MB                     │
│ mem.stack.sys      │ 1,024.0 KB                  │
│ mem.stack.in-use   │ 1,024.0 KB                  │
└────────────────────┴─────────────────────────────┘
```

#### show ports

Display the server's interface ports

```sh
machbase-neo» show ports;
┌─────────┬────────────────────────────────────────┐
│ SERVICE │ PORT                                   │
├─────────┼────────────────────────────────────────┤
│ grpc    │ tcp://127.0.0.1:5655                   │
│ grpc    │ unix:///database/mach-grpc.sock        │
│ http    │ tcp://127.0.0.1:5654                   │
│ mach    │ tcp://127.0.0.1:5656                   │
│ mqtt    │ tcp://127.0.0.1:5653                   │
│ shell   │ tcp://127.0.0.1:5652                   │
└─────────┴────────────────────────────────────────┘
```

#### show tables

Syntax: `show tables [-a]`

Display the table list. If flag `-a` is specified, the result includes the hidden tables.

```sh
machbase-neo» show tables;
┌────────┬────────────┬──────┬─────────────┬───────────┐
│ ROWNUM │ DB         │ USER │ NAME        │ TYPE      │
├────────┼────────────┼──────┼─────────────┼───────────┤
│      1 │ MACHBASEDB │ SYS  │ EXAMPLE     │ Tag Table │
│      2 │ MACHBASEDB │ SYS  │ TAG         │ Tag Table │
│      3 │ MACHBASEDB │ SYS  │ TAGDATA     │ Tag Table │
└────────┴────────────┴──────┴─────────────┴───────────┘
```

#### show table

Syntax `show table [-a] <table>`

Display the column list of the table. If flag `-a` is specified, the result includes the hidden columns.

```sh
machbase-neo» show table example -a;
┌────────┬───────┬──────────┬────────┬──────────┐
│ ROWNUM │ NAME  │ TYPE     │ LENGTH │ DESC     │
├────────┼───────┼──────────┼────────┼──────────┤
│      1 │ NAME  │ varchar  │    100 │ tag name │
│      2 │ TIME  │ datetime │     31 │ basetime │
│      3 │ VALUE │ double   │     17 │          │
│      4 │ _RID  │ long     │     20 │          │
└────────┴───────┴──────────┴────────┴──────────┘
```

#### show meta-tables

```sh
machbase-neo» show meta-tables;
┌────────┬─────────┬────────────────────────┬─────────────┐
│ ROWNUM │      ID │ NAME                   │ TYPE        │
├────────┼─────────┼────────────────────────┼─────────────┤
│      1 │ 1000020 │ M$SYS_TABLESPACES      │ Fixed Table │
│      2 │ 1000024 │ M$SYS_TABLESPACE_DISKS │ Fixed Table │
│      3 │ 1000049 │ M$SYS_TABLES           │ Fixed Table │
│      4 │ 1000051 │ M$TABLES               │ Fixed Table │
│      5 │ 1000053 │ M$SYS_COLUMNS          │ Fixed Table │
│      6 │ 1000054 │ M$COLUMNS              │ Fixed Table │
......
```

#### show virtual-tables

```sh
machbase-neo» show virtual-tables;
┌────────┬─────────┬─────────────────────────────────────────┬────────────────────┐
│ ROWNUM │      ID │ NAME                                    │ TYPE               │
├────────┼─────────┼─────────────────────────────────────────┼────────────────────┤
│      1 │      65 │ V$HOME_STAT                             │ Fixed Table (stat) │
│      2 │      93 │ V$DEMO_STAT                             │ Fixed Table (stat) │
│      3 │     227 │ V$SAMPLEBENCH_STAT                      │ Fixed Table (stat) │
│      4 │     319 │ V$TAGDATA_STAT                          │ Fixed Table (stat) │
│      5 │     382 │ V$EXAMPLE_STAT                          │ Fixed Table (stat) │
│      6 │     517 │ V$TAG_STAT                              │ Fixed Table (stat) │
......
```

#### show users

```sh
machbase-neo» show users;
┌────────┬───────────┐
│ ROWNUM │ USER_NAME │
├────────┼───────────┤
│      1 │ SYS       │
└────────┴───────────┘
a row fetched.
```

#### show license

```sh
 machbase-neo» show license;
┌────────┬──────────┬──────────────┬──────────┬────────────┬──────────────┬─────────────────────┐
│ ROWNUM │ ID       │ TYPE         │ CUSTOMER │ PROJECT    │ COUNTRY_CODE │ INSTALL_DATE        │
├────────┼──────────┼──────────────┼──────────┼────────────┼──────────────┼─────────────────────┤
│      1 │ 00000023 │ FOGUNLIMITED │ VUTECH   │ FORESTFIRE │ KR           │ 2024-04-22 15:56:14 │
└────────┴──────────┴──────────────┴──────────┴────────────┴──────────────┴─────────────────────┘
a row fetched.
```

#### session list

Syntax: `session list`

```sh
 machbase-neo» session list;
┌────┬───────────┬─────────┬────────────┬─────────┬─────────┬──────────┐
│ ID │ USER_NAME │ USER_ID │ STMT_COUNT │ CREATED │ LAST    │ LAST SQL │
├────┼───────────┼─────────┼────────────┼─────────┼─────────┼──────────┤
│ 25 │ SYS       │ 1       │          1 │ 1.667ms │ 1.657ms │ CONNECT  │
└────┴───────────┴─────────┴────────────┴─────────┴─────────┴──────────┘
```

#### session kill

Syntax `session kill <ID>`

#### session stat

Syntax: `session stat`

```sh
machbase-neo» session stat;
┌────────────────┬───────┐
│ NAME           │ VALUE │
├────────────────┼───────┤
│ CONNS          │ 1     │
│ CONNS_USED     │ 17    │
│ STMTS          │ 0     │
│ STMTS_USED     │ 20    │
│ APPENDERS      │ 0     │
│ APPENDERS_USED │ 0     │
│ RAW_CONNS      │ 1     │
└────────────────┴───────┘
```

#### desc

Syntax `desc [-a] <table>`

Describe table structure.

```sh
machbase-neo» desc example;
┌────────┬───────┬──────────┬────────┬──────────┐
│ ROWNUM │ NAME  │ TYPE     │ LENGTH │ DESC     │
├────────┼───────┼──────────┼────────┼──────────┤
│      1 │ NAME  │ varchar  │    100 │ tag name │
│      2 │ TIME  │ datetime │     31 │ basetime │
│      3 │ VALUE │ double   │     17 │          │
└────────┴───────┴──────────┴────────┴──────────┘
```

## machbase-neo restore

Syntax `machbase-neo restore --data <machbase_home_dir> <backup_dir>`

Restore database from backup.

```sh
$ machbase-neo restore --data <machbase home dir>  <backup dir>
```

## machbase-neo gen-config

Prints out default config template.

```
$ machbase-neo gen-config ↵

define DEF {
    LISTEN_HOST       = flag("--host", "127.0.0.1")
    SHELL_PORT        = flag("--shell-port", "5652")
    MQTT_PORT         = flag("--mqtt-port", "5653")
    HTTP_PORT         = flag("--http-port", "5654")
    GRPC_PORT         = flag("--grpc-port", "5655")
......
```