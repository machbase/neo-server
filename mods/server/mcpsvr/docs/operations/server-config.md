# Machbase Neo Config file

## Create a new config file

Execute machbase-neo with `gen-config` and save its output as default config file

```sh
machbase-neo gen-config > ./machbase-neo.conf
```

Edit generated config so that customize settings, then start machbase-neo with `--config <path>` or `-c <path>` option to direct where machbase-neo read config.

```sh
machbase-neo serve --config ./machbase-neo.conf
```

## Database directory

The default value of `DataDir` is `${execDir()}/machbase_home` which is a sub-directory of where `machbase-neo` executable file is.

Change it to new path where you want to store the database files. If the folder is new and has no database files, machbase-neo will create a new database automatically.

## Preference directory

The default value of `PrefDir` is `prefDir("machbase")` which is `$HOME/.config/machbase`.

## Listeners

| Listener                  | Config                    | default                 |
|:--------------------------|:--------------------------|:------------------------|
| SSH Shell                 | Shell.Listeners           | `tcp://127.0.0.1:5652`  |
| MQTT                      | Mqtt.Listeners            | `tcp://127.0.0.1:5653` `unix://${tempDir()}/machbase-neo-mqtt.sock`  |
| HTTP                      | Http.Listeners            | `tcp://127.0.0.1:5654` `unix://${tempDir()}/machbase-neo.sock`  |
| gRPC                      | Grpc.Listeners            | `tcp://127.0.0.1:5655` `unix://${execDir()}/mach-grpc.sock` |
| Machbase native           | Machbase.PORT_NO          | `5656`                  |
|                           | Machbase.BIND_IP_ADDRESS  | `127.0.0.1`             |

> **ℹ️ Info**  
> Machbase native port `5656` is used for native clients such as JDBC and ODBC.  
> JDBC, ODBC drivers can be found from Machbase home page.

## Config References

Syntax of config file adopts the HCL syntax.

### functions

Several functions are supported for the value of config item.

- `flag(A, B)` : Get value of command line flag 'A'. if not specified, apply B as default value.
- `env(A, B)` : get value of Environment variable 'A'. if not specified, apply B as default value
- `execDir()` : Get directory path where executable file is.
- `tempDir()` : Get system temp dir path.
- `userDir()` : Get user's home directory, On Linux and macOS, it returns the $HOME environment variable.
- `prefDir(subdir)` : Ger user's preference directory, On Linux and macOS, it returns the real path of $HOME/.config/{subdir}

> **ℹ️ Combine env() and flag()**  
> It is general practice for seeking user's setting that check command line flag first then find Environment variable and finally apply default value if both are not specified.  
> We can write value `flag("--my-var", env("MY_VAR", "myvalue"))` for this use case

### define DEF

This section is for the default values. the variables in this section are referred in other section. Users can define their own variables and even change the command line flags. As example below, `LISTEN_HOST` is taken value from `--host` flag of command line, but take `"127.0.0.1"` as default if `--host` flag is not provided.

If change `"127.0.0.1"` to `"192.168.1.10"`, the default value will be changed.

If change `"--host"` to `"--bind"` for example, command line flag will be changed. From then you can use `machbase-neo serve --bind <ip_addr>` instead of `machbase-neo serve --host <ip_addr>`.

```hcl
define DEF {
    LISTEN_HOST       = flag("--host", "127.0.0.1")
    SHELL_PORT        = flag("--shell-port", "5652")
    MQTT_PORT         = flag("--mqtt-port", "5653")
    HTTP_PORT         = flag("--http-port", "5654")
    GRPC_PORT         = flag("--grpc-port", "5655")
    GRPC_SOCK         = flag("--grpc-sock", "${execDir()}/mach-grpc.sock")
    MACH_PORT         = flag("--mach-port", "5656")
}
```

### define VARS

This section defines commonly used variables. 

```hcl
define VARS {
    PREF_DIR              = flag("--pref", prefDir("machbase"))
    DATA_DIR              = flag("--data", "${execDir()}/machbase_home")
    FILE_DIR              = flag("--file", "${execDir()}")
    UI_DIR                = flag("--ui", "")
    MACH_LISTEN_HOST      = flag("--mach-listen-host", DEF_LISTEN_HOST)
    MACH_LISTEN_PORT      = flag("--mach-listen-port", DEF_MACH_PORT)
    SHELL_LISTEN_HOST     = flag("--shell-listen-host", DEF_LISTEN_HOST)
    SHELL_LISTEN_PORT     = flag("--shell-listen-port", DEF_SHELL_PORT)
    GRPC_LISTEN_HOST      = flag("--grpc-listen-host", DEF_LISTEN_HOST)
    GRPC_LISTEN_PORT      = flag("--grpc-listen-port", DEF_GRPC_PORT)
    GRPC_LISTEN_SOCK      = flag("--grpc-listen-sock", DEF_GRPC_SOCK)
    HTTP_LISTEN_HOST      = flag("--http-listen-host", DEF_LISTEN_HOST)
    HTTP_LISTEN_PORT      = flag("--http-listen-port", DEF_HTTP_PORT)
    MQTT_LISTEN_HOST      = flag("--mqtt-listen-host", DEF_LISTEN_HOST)
    MQTT_LISTEN_PORT      = flag("--mqtt-listen-port", DEF_MQTT_PORT)
    MQTT_MAXMESSAGE       = flag("--mqtt-max-message", 1048576) // 1MB

    HTTP_ENABLE_TOKENAUTH = flag("--http-enable-token-auth", false)
    MQTT_ENABLE_TOKENAUTH = flag("--mqtt-enable-token-auth", false)
    MQTT_ENABLE_TLS       = flag("--mqtt-enable-tls", false)

    HTTP_ENABLE_WEBUI     = flag("--http-enable-web", true)
    HTTP_DEBUG_MODE       = flag("--http-debug", false)

    EXPERIMENT_MODE       = flag("--experiment", false)

    MACHBASE_ENABLE_SIGHANDLER = flag("--machbase-enable-sighandler", false)
    MACHBASE_INIT_OPTION       = flag("--machbase-init-option", 2)

    CREATEDB_SCRIPT_FILES  = flag("--createdb-script-files", "")
}
```

### logging config

| Key                         | Type      | Desc                                                     |
|:----------------------------|:----------|----------------------------------------------------------|
| Console                     | bool      | print out log message on console                         |
| Filename                    | string    | log file path `-` for stdout, ex) /logs/machbase-neo.log |
| DefaultPrefixWidth          | int       | alignment width of log prefix                            |
| DefaultEnableSourceLocation | bool      | enable logging source filename and line number           |
| DefaultLevel                | string    | `TRACE`, `DEBUG`, `INFO`, `WARN`, `ERROR`                |
| Levels                      | array     | array of Level object                                    |
| Append                      | bool      | append log file, if it exists                            |
| RotateSchedule              | string    | schedule to rotate log file ex) "@midnight"              |
| MaxSize                     | int       | max log file size in MB                                  |
| MaxBackups                  | int       | max number of backup files                               |
| MaxAge                      | int       | max days to keep the backup files                        |
| Compress                    | bool      | compress the backup files                                |
| UTC                         | bool      | Use UTC time for logging                                 |

- Level object

| Key                         | Type      | Desc                                                     |
|:----------------------------|:----------|----------------------------------------------------------|
| Pattern                     | string    | glob pattern form logger's name                          |
| Level                       | string    | log level for the logger                                 |

```hcl
module "machbase.com/neo-logging" {
    name = "neolog"
    config {
        Console                     = false
        Filename                    = flag("--log-filename", "-")
        Append                      = flag("--log-append", true)
        RotateSchedule              = flag("--log-rotate-schedule", "@midnight")
        MaxSize                     = flag("--log-max-size", 10)
        MaxBackups                  = flag("--log-max-backups", 1)
        MaxAge                      = flag("--log-max-age", 7)
        Compress                    = flag("--log-compress", false)
        UTC                         = flag("--log-time-utc", false)
        DefaultPrefixWidth          = 16
        DefaultEnableSourceLocation = flag("--log-source-location", false)
        DefaultLevel                = flag("--log-level", "INFO")
        Levels = [
            { Pattern="neo*", Level="TRACE" },
            { Pattern="http-log", Level="DEBUG" },
        ]
    }
}
```

### server config

This section is for the database server consists of multiple parts those will be explained in section by section.

#### MachbaseHome

| Key                         | Type      | Desc                                                     |
|:----------------------------|:----------|----------------------------------------------------------|
| MachbaseHome                | string    | directory path where database files are stored           |

#### Machbase

Machbase core properties are here, please refer to Machbase Manual Property section for details.

#### Shell

This allows remote access machbase-neo shell via ssh. Since default `LISTEN_HOST` is `"127.0.0.1"` the ssh access only available from same host machine. Set `"0.0.0.0"` or exact IP address of host machine to allow remote access.

> **⚠️ Security**  
> Before allow remote access, it is strongly recommended to change `SYS`'s default password from `manager` to your own.

| Key                         | Type               | Desc                                                     |
|:----------------------------|:-------------------|----------------------------------------------------------|
| Listeners                   | array of string    | listening addresses (ex: `tcp://127.0.0.1:5652`, `tcp://0.0.0.0:5652`)|
| IdleTimeout                 | duration           | server will close the ssh connection if there is no activity for the specified time |

#### Grpc

machbase-neo's gRPC listening address and size limit of messages are configured.

| Key                         | Type               | Desc                                                     |
|:----------------------------|:-------------------|----------------------------------------------------------|
| Listeners                   | array of string    | listening addresses                                       |
| MaxRecvMsgSize              | int                | maximum message size in MB that server can receive       |
| MaxSendMsgSize              | int                | maximum message size in MB                               |

#### Http

server's HTTP listener config.

| Key                         | Type               | Desc                                                     |
|:----------------------------|:-------------------|----------------------------------------------------------|
| Listeners                   | array of string    | listening addresses                                      |
| EnableTokenAuth             | bool               | enable token based authentication (default `false`)      |
| EnableWebUI                 | bool               | enable web user interface (default `true`)               |

#### Mqtt

| Key                         | Type               | Desc                                                     |
|:----------------------------|:-------------------|----------------------------------------------------------|
| Listeners                   | array of string    | listening addresses                                       |
| MaxMessageSizeLimit         | int                | maximum size limit of payload in a PUBLISH (default 1048576 = 1MB) |
| EnableTokenAuth             | bool               | enable token based authentication (default `false`)      |
| EnableTls                   | bool               | enable TLS for the TCP listeners (default `false`)       |

### neo-server config

```hcl
module "machbase.com/neo-server" {
    name = "neosvr"
    config {
        PrefDir          = VARS_PREF_DIR
        DataDir          = VARS_DATA_DIR
        FileDirs         = [ VARS_FILE_DIR ]
        ExperimentMode   = VARS_EXPERIMENT_MODE
        CreateDBScriptFiles = [ VARS_CREATEDB_SCRIPT_FILES ]
        Machbase         = {
            HANDLE_LIMIT     = 2048
            PORT_NO          = VARS_MACH_LISTEN_PORT
            BIND_IP_ADDRESS  = VARS_MACH_LISTEN_HOST
        }
        Shell = {
            Listeners        = [ "tcp://${VARS_SHELL_LISTEN_HOST}:${VARS_SHELL_LISTEN_PORT}" ]
            IdleTimeout      = "5m"
        }
        Grpc = {
            Listeners        = [ 
                "unix://${VARS_GRPC_LISTEN_SOCK}",
                "tcp://${VARS_GRPC_LISTEN_HOST}:${VARS_GRPC_LISTEN_PORT}",
            ]
            MaxRecvMsgSize   = 4
            MaxSendMsgSize   = 4
        }
        Http = {
            Listeners        = [ "tcp://${VARS_HTTP_LISTEN_HOST}:${VARS_HTTP_LISTEN_PORT}" ]
            WebDir           = VARS_UI_DIR
            EnableTokenAuth  = VARS_HTTP_ENABLE_TOKENAUTH
            DebugMode        = VARS_HTTP_DEBUG_MODE
            EnableWebUI      = VARS_HTTP_ENABLE_WEBUI
        }
        Mqtt = {
            Listeners           = [ "tcp://${VARS_MQTT_LISTEN_HOST}:${VARS_MQTT_LISTEN_PORT}"]
            EnableTokenAuth     = VARS_MQTT_ENABLE_TOKENAUTH
            EnableTls           = VARS_MQTT_ENABLE_TLS
            MaxMessageSizeLimit = VARS_MQTT_MAXMESSAGE
        }
        Jwt = {
            AtDuration = flag("--jwt-at-expire", "5m")
            RtDuration = flag("--jwt-rt-expire", "60m")
        }
        MachbaseInitOption       = VARS_MACHBASE_INIT_OPTION
        EnableMachbaseSigHandler = VARS_MACHBASE_ENABLE_SIGHANDLER
    }
}