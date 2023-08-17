
[![](https://img.shields.io/github/v/release/machbase/neo-server?sort=semver)](https://github.com/machbase/neo-server/releases)
[![](https://github.com/machbase/neo-server/actions/workflows/ci-linux-arm64.yml/badge.svg)](https://github.com/machbase/neo-server/actions/workflows/ci-linux-arm64.yml)
[![](https://github.com/machbase/neo-server/actions/workflows/ci-linux-amd64.yml/badge.svg)](https://github.com/machbase/neo-server/actions/workflows/ci-linux-amd64.yml)
[![](https://github.com/machbase/neo-server/actions/workflows/ci-darwin-arm64.yml/badge.svg)](https://github.com/machbase/neo-server/actions/workflows/ci-darwin-arm64.yml)
[![](https://github.com/machbase/neo-server/actions/workflows/ci-darwin-amd64.yml/badge.svg)](https://github.com/machbase/neo-server/actions/workflows/ci-darwin-amd64.yml)
[![](https://github.com/machbase/neo-server/actions/workflows/ci-windows-amd64.yml/badge.svg)](https://github.com/machbase/neo-server/actions/workflows/ci-windows-amd64.yml)
[![](https://github.com/machbase/neo-server/actions/workflows/ci-linux-arm32.yml/badge.svg)](https://github.com/machbase/neo-server/actions/workflows/ci-linux-arm32.yml)

# neo-server

Machbase is the fastest time-series database for [IoT in the world](https://www.tpc.org/tpcx-iot/results/tpcxiot_perf_results5.asp?version=2) implemented in C++. 
`machbase-neo` is an IoT Database Server that embedding the Machbase engine and provides essential and convenient features to build IoT platforms such as MQTT and HTTP API. It can be installed on machines ranging from Raspberry PI to high-performance servers.

API and Interfaces

- [x] HTTP : Applications and Sensors read/write data via HTTP REST API
- [x] MQTT : Sensors write data via MQTT protocol
- [x] gRPC : The first class API for extensions
- [x] SSH : Command line interface for human and batch process
- [x] WEB UI : GUI

Bridges integrated with external systems

- [x] SQLite
- [x] PostgreSQL
- [x] MySQL
- [x] MS-SQL
- [x] MQTT Broker
- [ ] Kafka
- [ ] NATS

## Documents

[https://neo.machbase.com](https://neo.machbase.com/)

## Install Prebuilt Binary

- Download

```sh
sh -c "$(curl -fsSL https://neo.machbase.com/install.sh)"
```

- Unzip the archive file

## Install Using Docker

```sh
docker pull machbase/machbase-neo
```

https://hub.docker.com/r/machbase/machbase-neo

## Build from sources

- Install Go 1.19 or higher 
- Install `make`
- Checkout machbase/neo-server
- Run `make machbase-neo`
- Find the executable binary `./tmp/machbase-neo`

### Dependency

![deps](./docs/deps.png)

- [neo-server](https://github.com/machbase/neo-server) machbase-neo source code
- [neo-spi](https://github.com/machbase/neo-spi) defines interfaces accessing database
- [neo-grpc](https://github.com/machbase/neo-grpc) implements spi accessing database via gRPC
- [neo-engine](https://github.com/machbase/neo-engine) implements spi accessing database via C API
