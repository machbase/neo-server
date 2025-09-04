
define DEF {
    LISTEN_HOST       = flag("--host", "127.0.0.1")
    SHELL_PORT        = flag("--shell-port", "5652")
    MQTT_PORT         = flag("--mqtt-port", "5653")
    MQTT_SOCK         = flag("--mqtt-sock", "${tempDir()}/machbase-neo-mqtt.sock")
    HTTP_PORT         = flag("--http-port", "5654")
    HTTP_SOCK         = flag("--http-sock", "${tempDir()}/machbase-neo.sock")
    GRPC_PORT         = flag("--grpc-port", "5655")
    GRPC_SOCK         = flag("--grpc-sock", "${execDir()}/mach-grpc.sock")
    GRPC_INSECURE     = flag("--grpc-insecure", false)
    MACH_PORT         = flag("--mach-port", "5656")
}

define VARS {
    PREF_DIR          = flag("--pref", prefDir("machbase"))
    DATA_DIR          = flag("--data", "${execDir()}/machbase_home")
    FILE_DIR          = flag("--file", "${execDir()}")
    BACKUP_DIR        = flag("--backup-dir", "${execDir()}/backups")
    UI_DIR            = flag("--ui", "")
    MACH_LISTEN_HOST  = flag("--mach-listen-host", DEF_LISTEN_HOST)
    MACH_LISTEN_PORT  = flag("--mach-listen-port", DEF_MACH_PORT)
    SHELL_LISTEN_HOST = flag("--shell-listen-host", DEF_LISTEN_HOST)
    SHELL_LISTEN_PORT = flag("--shell-listen-port", DEF_SHELL_PORT)
    GRPC_LISTEN_HOST  = flag("--grpc-listen-host", DEF_LISTEN_HOST)
    GRPC_LISTEN_PORT  = flag("--grpc-listen-port", DEF_GRPC_PORT)
    GRPC_LISTEN_SOCK  = flag("--grpc-listen-sock", DEF_GRPC_SOCK)
    HTTP_LISTEN_HOST  = flag("--http-listen-host", DEF_LISTEN_HOST)
    HTTP_LISTEN_PORT  = flag("--http-listen-port", DEF_HTTP_PORT)
    HTTP_LISTEN_SOCK  = flag("--http-listen-sock", DEF_HTTP_SOCK)
    MQTT_LISTEN_HOST  = flag("--mqtt-listen-host", DEF_LISTEN_HOST)
    MQTT_LISTEN_PORT  = flag("--mqtt-listen-port", DEF_MQTT_PORT)
    MQTT_LISTEN_SOCK  = flag("--mqtt-listen-sock", DEF_MQTT_SOCK)
    MQTT_MAXMESSAGE   = flag("--mqtt-max-message", 1048576) // 1MB
    MQTT_PERSISTENCE  = flag("--mqtt-persistence", false)

    HTTP_ENABLE_TOKENAUTH = flag("--http-enable-token-auth", false)
    MQTT_ENABLE_TOKENAUTH = flag("--mqtt-enable-token-auth", false)
    MQTT_ENABLE_TLS       = flag("--mqtt-enable-tls", false)

    HTTP_DEBUG_MODE       = flag("--http-debug", false)
    HTTP_DEBUG_LATENCY    = flag("--http-debug-latency", "0")
    HTTP_READBUF_SIZE     = flag("--http-readbuf-size", 0)  // 0 means default, bytes
    HTTP_WRITEBUF_SIZE    = flag("--http-writebuf-size", 0) // 0 means default, bytes
    HTTP_LINGER           = flag("--http-linger", -1)       // -1 means disable so_linger, >= 0 means set so_linger
    HTTP_KEEPALIVE        = flag("--http-keepalive", 0)     // 0 means disable, > 0 means enable and set_keepalive_period in seconds
    HTTP_ALLOW_STATZ      = flag("--http-allow-statz", "")  // allow statz for the given IP address
    HTTP_QUERY_CYPHER     = flag("--http-query-cypher", "") // format: <alg>:<key>, e.g., AES:1234567890abcdef

    MAX_POOL_SIZE         = flag("--max-pool-size", 0)
    MAX_OPEN_CONN         = flag("--max-open-conn", -1)
    MAX_OPEN_CONN_FACTOR  = flag("--max-open-conn-factor", 2.0)
    MAX_OPEN_QUERY        = flag("--max-open-query", 0)
    MAX_OPEN_QUERY_FACTOR = flag("--max-open-query-factor", 2.0)

    EXPERIMENT_MODE       = flag("--experiment", false)
    MACHBASE_INIT_OPTION  = flag("--machbase-init-option", 2)
    CREATEDB_SCRIPT_FILES = flag("--createdb-script-files", "")
    STATZ_OUT             = flag("--statz-out", "")
}

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

module "machbase.com/neo-server" {
    name = "neosvr"
    config {
        PrefDir          = VARS_PREF_DIR
        DataDir          = VARS_DATA_DIR
        FileDirs         = [ "/=${VARS_FILE_DIR}" ]
        BackupDir        = VARS_BACKUP_DIR
        ExperimentMode   = VARS_EXPERIMENT_MODE
        MachbaseInitOption  = VARS_MACHBASE_INIT_OPTION
        StatzOut            = VARS_STATZ_OUT
        CreateDBScriptFiles = [ VARS_CREATEDB_SCRIPT_FILES ]
        MaxPoolSize          = VARS_MAX_POOL_SIZE
        MaxOpenConn          = VARS_MAX_OPEN_CONN
        MaxOpenConnFactor    = VARS_MAX_OPEN_CONN_FACTOR
        MaxOpenQuery         = VARS_MAX_OPEN_QUERY
        MaxOpenQueryFactor   = VARS_MAX_OPEN_QUERY_FACTOR
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
            Insecure         = DEF_GRPC_INSECURE
        }
        Http = {
            Listeners        = [
                "unix://${VARS_HTTP_LISTEN_SOCK}",
                "tcp://${VARS_HTTP_LISTEN_HOST}:${VARS_HTTP_LISTEN_PORT}",
            ]
            WebDir           = VARS_UI_DIR
            EnableTokenAuth  = VARS_HTTP_ENABLE_TOKENAUTH
            DebugMode        = VARS_HTTP_DEBUG_MODE
            DebugLatency     = "${VARS_HTTP_DEBUG_LATENCY}"
            WriteBufSize     = VARS_HTTP_WRITEBUF_SIZE
            ReadBufSize      = VARS_HTTP_READBUF_SIZE
            Linger           = VARS_HTTP_LINGER
            KeepAlive        = VARS_HTTP_KEEPALIVE
            AllowStatz       = ["${VARS_HTTP_ALLOW_STATZ}"]
            QueryCypher      = VARS_HTTP_QUERY_CYPHER
        }
        Mqtt = {
            Listeners           = [
                "unix://${VARS_MQTT_LISTEN_SOCK}",
                "tcp://${VARS_MQTT_LISTEN_HOST}:${VARS_MQTT_LISTEN_PORT}",
            ]
            EnableTokenAuth     = VARS_MQTT_ENABLE_TOKENAUTH
            EnableTls           = VARS_MQTT_ENABLE_TLS
            MaxMessageSizeLimit = VARS_MQTT_MAXMESSAGE
            EnablePersistence   = VARS_MQTT_PERSISTENCE
        }
        Jwt = {
            AtDuration = flag("--jwt-at-expire", "5m")
            RtDuration = flag("--jwt-rt-expire", "60m")
            Secret     = flag("--jwt-secret", "__secret__")
        }
    }
}
