# OPCUA Simulator Server

This server provides basic system metrics via the OPCUA protocol. It is intended for testing and simulation purposes.

## Exposed OPCUA Nodes

The following nodes are available and provide simulated metric values:

- `ns=1;s=sys_cpu`   : System CPU usage
- `ns=1;s=sys_mem`   : System memory usage
- `ns=1;s=load1`     : 1-minute load average
- `ns=1;s=load5`     : 5-minute load average
- `ns=1;s=load15`    : 15-minute load average

## How to Run the OPCUA Simulator Server

Follow these steps to set up and run the OPCUA simulator server:

1. **Create a new directory for the server:**
    ```sh
    mkdir opcua-server
    cd opcua-server
    ```

2. **Copy the provided Go source code into a file named `main.go` in this directory.**

3. **Initialize a new Go module:**
    ```sh
    go mod init opcua-server
    ```

4. **Download the required dependencies:**
    ```sh
    go mod tidy
    ```

5. **Run the OPCUA simulator server:**
    ```sh
    go run .
    ```

The server will start and listen for OPCUA client connections, exposing the nodes listed above.

## Connecting to the Server

You can use any OPCUA client to connect to this server. The endpoint URL will typically be:
```
opc.tcp://localhost:4840
```
Adjust the address and port as needed if you modify the server code.

---

For further customization or integration, edit `main.go` as needed.

