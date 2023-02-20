
define DEF {
    LISTEN_HOST       = flag("--host", "127.0.0.1")
    SHELL_PORT        = flag("--shell-port", "5652")
    MQTT_PORT         = flag("--mqtt-port", "5653")
    HTTP_PORT         = flag("--http-port", "5654")
    GRPC_PORT         = flag("--grpc-port", "5655")
    GRPC_SOCK         = flag("--grpc-sock", "${execDir()}/mach-grpc.sock")
}

define VARS {
    PREF_DIR          = flag("--pref", prefDir("machbase"))
    DATA_DIR          = flag("--data", "${execDir()}/machbase_home")
    SHELL_LISTEN_HOST = flag("--shell-listen-host", DEF_LISTEN_HOST)
    SHELL_LISTEN_PORT = flag("--shell-listen-port", DEF_SHELL_PORT)
    GRPC_LISTEN_HOST  = flag("--grpc-listen-host", DEF_LISTEN_HOST)
    GRPC_LISTEN_PORT  = flag("--grpc-listen-port", DEF_GRPC_PORT)
    GRPC_LISTEN_SOCK  = flag("--grpc-listen-sock", DEF_GRPC_SOCK)
    HTTP_LISTEN_HOST  = flag("--http-listen-host", DEF_LISTEN_HOST)
    HTTP_LISTEN_PORT  = flag("--http-listen-port", DEF_HTTP_PORT)
    MQTT_LISTEN_HOST  = flag("--mqtt-listen-host", DEF_LISTEN_HOST)
    MQTT_LISTEN_PORT  = flag("--mqtt-listen-port", DEF_MQTT_PORT)
    MQTT_MAXMESSAGE   = flag("--mqtt-max-message", 1048576) // 1MB

    HTTP_ENABLE_TOKENAUTH = flag("--http-enable-token-auth", false)
}

module "machbase.com/neo-logging" {
    name = "neolog"
    config {
        Console                     = false
        Filename                    = flag("--log-filename", "-")
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
        Machbase         = {
            HANDLE_LIMIT     = 2048
            BIND_IP_ADDRESS  = DEF_LISTEN_HOST
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
            Handlers         = [
                { Prefix: "/db",      Handler: "machbase" },
                { Prefix: "/metrics", Handler: "influx" },
            ]
            EnableTokenAuth  = VARS_HTTP_ENABLE_TOKENAUTH
        }
        Mqtt = {
            Listeners        = [ "tcp://${VARS_MQTT_LISTEN_HOST}:${VARS_MQTT_LISTEN_PORT}"]
            Handlers         = [
                { Prefix: "db",       Handler: "machbase" },
                { Prefix: "metrics",  Handler: "influx" },
            ]
            MaxMessageSizeLimit = VARS_MQTT_MAXMESSAGE
        }
    }
}
