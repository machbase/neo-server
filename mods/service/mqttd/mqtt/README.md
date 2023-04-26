
```h
module "github.com/machbase/neo-server/mods/service/mqttsvr/mqtt" {
    config {
        TcpConfig {
            ListenAddress    = "${GLOBAL_IP_BIND}:1884"
            SoLinger         = 0
            KeepAlive        = 10
            NoDelay          = true
            Tls {
                Disabled         = false // false is default
                LoadSystemCAs    = false
                LoadPrivateCAs   = true
                CertFile         = GLOBAL_SERVER_CERT
                KeyFile          = GLOBAL_SERVER_KEY
                HandshakeTimeout = "5s" // equivalent 5000000000
            }
        }
    }
    inject <your_module> Mqttd {}
}
```