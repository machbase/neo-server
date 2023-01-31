
define DEF {
    LISTEN_HOST       = flag("--host", "127.0.0.1")
    SHELL_PORT        = flag("--shell-port", "5652")
    MQTT_PORT         = flag("--mqtt-port", "5653")
    HTTP_PORT         = flag("--http-port", "5654")
    GRPC_PORT         = flag("--grpc-port", "5655")
}

define VARS {
    DATA_DIR          = flag("--data", "${execDir()}/machbase_home")
    SHELL_LISTEN_HOST = flag("--shell-listen-host", DEF_LISTEN_HOST)
    SHELL_LISTEN_PORT = flag("--shell-listen-port", DEF_SHELL_PORT)
    GRPC_LISTEN_HOST  = flag("--grpc-listen-host", DEF_LISTEN_HOST)
    GRPC_LISTEN_PORT  = flag("--grpc-listen-port", DEF_GRPC_PORT)
    HTTP_LISTEN_HOST  = flag("--http-listen-host", DEF_LISTEN_HOST)
    HTTP_LISTEN_PORT  = flag("--http-listen-port", DEF_HTTP_PORT)
    MQTT_LISTEN_HOST  = flag("--mqtt-listen-host", DEF_LISTEN_HOST)
    MQTT_LISTEN_PORT  = flag("--mqtt-listen-port", DEF_MQTT_PORT)
}

module "machbase.com/neo-logging" {
    name = "neolog"
    config {
        Console                     = false
        Filename                    = "-"
        DefaultPrefixWidth          =16
        DefaultEnableSourceLocation = flag("--log-source-location", false)
        DefaultLevel                = "TRACE"
        Levels = [
            { Pattern="machsvr", Level="TRACE" },
        ]
    }
}

module "machbase.com/neo-server" {
    name = "neosvr"
    config {
        MachbaseHome     = VARS_DATA_DIR
        Machbase         = {
            HANDLE_LIMIT     = 2048
        }
        Shell = {
            Listeners        = [ "tcp://${VARS_SHELL_LISTEN_HOST}:${VARS_SHELL_LISTEN_PORT}" ]
            IdleTimeout      = "5m"
        }
        Grpc = {
            Listeners        = [ 
                "unix://${execDir()}/mach-grpc.sock",
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
                { Prefix: "/logvault", Handler: "logvault"},
            ]
        }
        Mqtt = {
            Listeners        = [ "tcp://${VARS_MQTT_LISTEN_HOST}:${VARS_MQTT_LISTEN_PORT}"]
            Handlers         = [
                { Prefix: "db",       Handler: "machbase" },
                { Prefix: "metrics",  Handler: "influx" },
            ]
        }
    }
}
