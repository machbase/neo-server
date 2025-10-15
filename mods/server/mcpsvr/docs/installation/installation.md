# Machbase Neo Installation & Getting Started Guide

## Platform & Architecture Support

- **Raspberry Pi**: Ubuntu 22.04 with Raspberry Pi 4
- **Linux arm64**: Ubuntu 22.04, 24.04
- **Linux amd64**: Ubuntu 20.04, 22.04, 24.04
- **macOS**: Intel CPU (macOS 13), Apple Silicon (macOS 14, 15)
- **Windows**: Windows 10 Fall 2018 or newer, Windows 11

## Direct Installation

### Setup Process

1. **Download (recommended)**
   
   One-line instant script:
   ```bash
   sh -c "$(curl -fsSL https://docs.machbase.com/install.sh)"
   ```
   
   Or download the latest version for your platform from https://docs.machbase.com/neo/releases/

2. **Extract Archive**
   ```bash
   unzip machbase-neo-v8.0.58-linux-amd64.zip
   ```
   
   **By platform:**
   ```bash
   # Linux ARM64
   unzip machbase-neo-v8.0.58-linux-arm64.zip
   
   # macOS Apple Silicon
   unzip machbase-neo-v8.0.58-darwin-arm64.zip
   
   # macOS Intel
   unzip machbase-neo-v8.0.58-darwin-amd64.zip
   
   # Windows
   unzip machbase-neo-v8.0.58-windows-amd64.zip
   ```

3. **Confirm Executable**
   ```bash
   machbase-neo version
   ```

## Docker Installation

### Prerequisites
- Docker

### Docker Pull

To install the latest version of machbase-neo with Docker, enter the following command in terminal:

```bash
$ docker pull machbase/machbase-neo
```

If you want a specific version, add a tag:

```bash
$ docker pull machbase/machbase-neo:v8.0.58
```

> **Note**: To find different Docker versions, check https://hub.docker.com/r/machbase/machbase-neo/

### Docker Run

#### Foreground Execution
```bash
$ docker run -it machbase/machbase-neo
```

**Options:**
- `-i`, `--interactive`: Keep STDIN open
- `-t`, `--tty`: Allocate a pseudo-TTY

If running in foreground, you can exit directly with `Ctrl + c`.

#### Background Execution
```bash
$ docker run -d machbase/machbase-neo
```

**Options:**
- `-d`, `--detach`: Run container in background and print container ID

If running in background, you can exit with the following command:

```bash
$ docker stop $(docker ps | grep machbase-neo | awk '{print $1}')
```

If using multiple machbase-neo images, it's recommended to stop by entering the Container ID directly:

```bash
$ docker ps
CONTAINER ID   IMAGE                   COMMAND                   CREATED         STATUS        PORTS           NAMES
92382cf7b738   machbase/machbase-neo   "/bin/sh -c '/opt/ma…"   2 seconds ago   Up 1 second   5652-5656/tcp   exciting_volhard

$ docker stop 92382cf7b738
```

### Docker Configuration

#### Volume Binding
You can bind host directories to machbase-neo home path in docker:

```bash
docker run -d \
           -v /path/to/host/data:/data \
           -v /path/to/host/file:/file \
           machbase/machbase-neo
```

**Paths:**
- `/data`: machbase-neo home path in docker
- `/file`: machbase-neo tql path in docker
- `-v`, `--volume`: Bind mount a volume

#### Port Configuration
Machbase-neo exposes several ports in Docker:

| Port | Description |
|:-----|:------------|
| 5652 | sshd |
| 5653 | mqtt |
| 5654 | http |
| 5655 | grpc |
| 5656 | database engine |

#### Port Mapping (Forwarding)
```bash
$ docker run -d -p <host port>:<container port>/<protocol> machbase/machbase-neo
```

**Example:**
```bash
$ docker run -d \
             -p 5652-5652:5652-5656/tcp \
             --name machbase-neo \
             machbase/machbase-neo
```

#### Remote Access Using SSH Key

1. **Generate SSH key:**
   ```bash
   $ ssh-keygen -t rsa
   ```

2. **Run machbase-neo:**
   ```bash
   $ docker pull machbase/machbase-neo
   $ docker run -d \
                -p 5652-5656:5652-5656/tcp \
                --name machbase-neo \
                machbase/machbase-neo
   ```

3. **Register SSH key:**
   ```bash
   $ ssh -l sys -p 5652 192.168.0.116 ssh-key add `cat ~/.ssh/id_rsa.pub`
   sys@192.168.0.116's password? manager
   Add sshkey success
   ```

#### Using Docker Compose

Create `docker-compose.yml` file:

```yml
# docker-compose.yml
version: '3'
services:
  machbase-neo:
    image: machbase/machbase-neo
    container_name: machbase-neo
    hostname: machbase
    volumes:
      - /data:/data
      - /file:/file
    ports:
      - "5652:5652" # sshd
      - "5653:5653" # mqtt
      - "5654:5654" # http
      - "5655:5655" # grpc
      - "5656:5656" # database engine
```

**Commands:**
```bash
# Start
$ docker compose up -d

# Or specify file
$ docker compose -f docker-compose.yml up -d

# Stop
$ docker compose down
```

## Start and Stop

### Linux & macOS

#### Start
```bash
machbase-neo serve
```

#### Expose Ports
By default, machbase-neo runs only on localhost for security reasons. To allow remote client access:

**Allow access from all addresses:**
```bash
machbase-neo serve --host 0.0.0.0
```

**Allow specific address only:**
```bash
machbase-neo serve --host 192.168.1.10
```

#### Stop
If running in foreground mode, press `Ctrl+C`.

Or use shutdown command:
```bash
machbase-neo shell shutdown
```

### Windows

On Windows, double-click "neow.exe" and click the "machbase-neo serve" button in the top left of the window.

#### Windows Service Registration

> **Important**: Must be executed in Administrator mode.

**Install:**
```
.\machbase-neo service install --host 127.0.0.1 --data C:\neo-server\database --file C:\neo-server\files --log-filename C:\neo-server\machbase-neo.log --log-level INFO
```

**Start/Stop:**
```
.\machbase-neo service start
.\machbase-neo service stop
```

**Remove:**
```
.\machbase-neo service remove
```

## Deploy Modes

### Head Only Mode

Use URL pointing to another Machbase DBMS's mach port as `--data` flag value:

```bash
machbase-neo serve --data machbase://sys:manager@192.168.1.100:5656
```

Or using environment variables:
```bash
SECRET="sys:manager" \
machbase-neo serve --data machbase://${SECRET}@192.168.1.100:5656
```

### Headless Mode

Start only DBMS process (using mach port 5656 only):

```bash
machbase-neo serve-headless
```

## Web UI Access

### Login

Navigate to [http://127.0.0.1:5654/](http://127.0.0.1:5654/) in your web browser.

**Default credentials:** ID `sys`, Password `manager`

### Change Password

It's recommended to change the default password for security reasons.

#### Via Web UI:
1. Select "Change password" from the bottom left menu
2. Enter new password and confirm

#### Via SQL:
```sql
ALTER USER sys IDENTIFIED BY new_password;
```

#### Via Command Line:
```bash
machbase-neo shell "ALTER USER SYS IDENTIFIED BY new_password"
```

## Quick Reference

| Method | Command/Action | Description |
|--------|----------------|-------------|
| **Direct Install** | `curl install.sh` script | Recommended one-line installation |
| **Docker Install** | `docker pull machbase/machbase-neo` | Container-based installation |
| **Start Service** | `machbase-neo serve` | Start on localhost only |
| **Remote Access** | `--host 0.0.0.0` | Allow remote connections |
| **Web UI** | http://127.0.0.1:5654 | Default web interface |
| **Default Login** | sys/manager | Change password after first login |