define GLOBAL {
    IP_BIND      = "127.0.0.1"
    IP_ADVERTISE = "10.10.10.1"

    SERVER_CERT  = "./test/test_server_cert.pem"
    SERVER_KEY   = "./test/test_server_key.pem"

    VERSION      = customfunc()
    
    LOGDIR           = "./tmp"
    LOG_LEVEL        = "ERROR"
    LOG_PREFIX_WIDTH = 51
    LOG_APPEND       = true

    BASE_PRIORITY_APP = 200
}

define anyname {
    MAX_BACKUPS = 3
    ROTATE = "@midnight"
}
