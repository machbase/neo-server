# Build

```sh
go build -o stress-prepare prepare.go
```

# Run

```sh
./stress-prepare -host 127.0.0.1 -port 33000
```

# Options

- `-c <int>` number client connections
- `-n <int>` number of repeapts per a client connection
- `-host <ip>` machbase host
- `-port <int>` machbase port
- `-user <string>`
- `-pass <string>`

# Usage

```
./stress-prepare -h
Usage of ./stress-prepare:
  -h, -help
    	show help
  -host string
    	Database host (default "192.168.0.90")
  -port int
    	Database port (default 33000)
  -c int
    	number of clients (default 20)
  -n int
    	number of prepares per client (default 1000000)
```
