# Machbase Neo Security Guide

## Generates Key & Token

### Web UI

1. Select the menu icon from the left most side.

2. And Click `+` icon from the top left pane.

3. Set "Client Id" for unique name and set the valid period (default is 3 years from today).
Then click "Generate" to generates key files for the client.

4. Click "Download *.zip" button or copy & paste each file's content. This is not re-generatable and only chance to make a copy.

### Shell Command

The subcommand `machbase-neo shell key` manages client keys and tokens.

**List registered client authentication keys and tokens**

```
machbase-neo shell key list
```

List all pre-registered client-id and validation periods.

```
$ machbase-neo shell key list
┌────────┬──────────────────────┬───────────────────────────────┬───────────────────────────────┐
│ ROWNUM │ ID                   │ VALID FROM                    │ EXPIRE                        │
├────────┼──────────────────────┼───────────────────────────────┼───────────────────────────────┤
│      1 │ myid2                │ 2023-02-05 01:55:18 +0000 UTC │ 2033-02-02 01:55:18 +0000 UTC │
│      2 │ myid3                │ 2023-02-05 01:56:36 +0000 UTC │ 2033-02-02 01:56:36 +0000 UTC │
......
```

**Delete an existing client authentication key and token**

```
machbase-neo shell key del <client-id>
```

```
$ machbase-neo shell key del myid2
deleted
```

**Register new client authentication keys and tokens**

`machbase-neo shell key gen` subcommand generates new key pair and token for the given client-id.
It writes keys and token into the file that you specify by `--output` option.

```
machbase-neo shell key gen <client-id> --output <output_file>
```

Generate and register new key for the client-id `myapp01`. It stores the generated key and token to the `*_cert.pem`, `*_key.pem` and `*_token` files.

```
$ machbase-neo shell key gen myapp01 --output ./myapp01 
Save certificate ./myapp01_cert.pem
Save private key ./myapp01_key.pem
Save token ./myapp01_token
```

Check the generated files.

```
$ ls -al ./myapp01*
-rw-r--r--  1 eirny  staff  782 Feb 20 19:33 ./myapp01_cert.pem
-rw-------  1 eirny  staff  390 Feb 20 19:33 ./myapp01_key.pem
-rw-------  1 eirny  staff   81 Feb 20 19:33 ./myapp01_token
```

- `*_cert.pem` file is the X.509 certificate for the client which is signed by the server.
- `*_key.pem` file is the private key for the client.
- `*_token` file contains token string for the client.

For the token based authentication, see the content of the `*_token` file.

```
$ cat ./myapp01_token 
myapp01:b:d59310703c1ebf627f8b781fb50437326ec65b067257ebc72f07b12846761d17   
```

**Server Certificate**

To retrieve server's certificate, execute command `machbase-neo key server-key --output <path>`, it export server's certificate into the file that specified the path.

```
machbase-neo shell key server-cert --output ./machbase-neo.crt
```

## HTTP Token authentication

HTTP API of machbase-neo supports the token based authentication.

Enable it by specifying `--http-enable-token-auth true` command line option or set `EnableTokenAuth = true` in the config file.
When you launching server with the option, all HTTP API invocations requires `Authorization` header with pre-registered token.

```
machbase-neo serve --http-enable-token-auth true
```

The starting log shows HTTP token authentication is enabled.

```
......
2023/02/20 20:14:29.878 INFO  neo neosvr           HTTP token authentication enabled
2023/02/20 20:14:29.878 INFO  neo neosvr           HTTP Listen tcp://127.0.0.1:5654
......
```

### HTTP Client using token

Let's use the token for API authentication. Set `Authorization` bearer header with the content of token file.

```
curl --output - http://127.0.0.1:5654/db/query \
    --data-urlencode "q=select * from EXAMPLE limit 2" \
    -H "Authorization: Bearer `cat ./http-api-app01_token`"
```

```json
{
  "data": {
    "columns": [ "NAME", "TIME", "VALUE" ],
    "types": [ "string", "datetime", "double" ],
    "rows": [
      [ "wave.sin", 1675851592000000000, 0 ],
      [ "wave.cos", 1675851592000000000, 1 ]
    ]
  },
  "success": true,
  "reason": "success",
  "elapse": "1.866708ms"
}
```

Let's try without the `Authorization` header, or wrong token.

```
curl --output - http://127.0.0.1:5654/db/query \
    --data-urlencode "q=select * from EXAMPLE limit 2" \
    -H "Authorization: Bearer http-api-app01:b:intended-wrong-value"
```

If client provides an invalid token, the server responses `HTTP/1.1 401 Unauthorized` with an error json message below.

```json
{"success":false,"reason":"invalid token"}
```

## MQTT Token authentication

MQTT API of machbase-neo supports the token based authentication.

Enable it by specifying `--mqtt-enable-token-auth true` command line option or set `EnableTokenAuth = true` in the config file.
When you launching server with this option, MQTT CONNECT message requires `client-id`, `username` with pre-registered id and token.

```
machbase-neo serve --mqtt-enable-token-auth true
```

The starting log shows MQTT token authentication is enabled.

```
......
2023/02/21 13:43:11.178 INFO  neosvr           MQTT token authentication enabled
2023/02/21 13:43:11.180 INFO  mqtt-tcp         MQTT Listen tcp://127.0.0.1:5653
......
```

### MQTT client using token

Use the registered token as the `username` in the CONNECT message, and leave the `password` field empty.

```
mosquitto_pub -h 127.0.0.1 -p 5653 \
    --username `cat ./mqtt-api-app01_token` \
    -t db/write/EXAMPLE            \
    -m '[ "wave.pi", `date +%s000000000`, 3.1415]'
```

If a client does not provide the correct token in the `username` field, the server will reject the CONNECT message.

```
mosquitto_pub -h 127.0.0.1 -p 5653 -t db/write/EXAMPLE \
    -m '[ "wave.pi", `date +%s000000000`, 3.1415]'

Connection error: Connection Refused: not authorized.
Error: The connection was refused.
```

## MQTT X.509 authentication

When machbase-neo starts with `--mqtt-enable-tls true` command line option or set `Tls.Enabled = true` in the configurationfile,
machbase-neo accepts TLS (a.k.a SSL) connections from clients. 
If TLS is enabled, it ignores token based authentication and accepts only connection that finished ssl-handshaking successfully 
with pre-registered X.509 certificates.

> When TLS option is applied, machbase-neo mqtt server ignores `username` and `password` fields of CONNECT message.
> Do not specify those values. But still need to set `client-id` for the clarity.

### MQTT client using X.509

A client should use the pre-registered client-id and key and certificate those were generated as the above section.
Apply client-id for the `client-id` of CONNECT message and do not set the `username` and `password`.

```sh
mosquitto_pub -h 127.0.0.1 -p 5653 \
    --id myapp01            \
    --cert ./myapp01_cert.pem \
    --key ./myapp01_key.pem   \
    --cafile ./machbase-neo.crt --insecure \
    -t db/append/EXAMPLE            \
    -m '[ "wave.pi", `date +%s000000000`, 3.1415]'
```

- `--id` apply `client-id` that was used for generating key
- `--cert` client's certificate file which was generated as `*_cert.pem`
- `--key` client's key file that was generated as `*_key.pem`
- `--cafile` set server's certificate since the client's certificate is signed by server. see below to know how to get this file.
- `--insecure` additionally required because server's certificate is self-signed one.

---

## Security Configuration Summary

| Authentication Method | Activation Option | Usage Example |
|----------------------|-------------------|---------------|
| HTTP Token | `--http-enable-token-auth true` | curl -H "Authorization: Bearer token" |
| MQTT Token | `--mqtt-enable-token-auth true` | mosquitto_pub --username token |
| MQTT X.509 | `--mqtt-enable-tls true` | mosquitto_pub --cert cert.pem --key key.pem |