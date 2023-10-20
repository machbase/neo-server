
[![](https://img.shields.io/github/v/release/machbase/neo-server?sort=semver)](https://github.com/machbase/neo-server/releases)
[![](https://github.com/machbase/neo-server/actions/workflows/ci-main.yml/badge.svg)](https://github.com/machbase/neo-server/actions/workflows/ci-main.yml)
[![codecov](https://codecov.io/gh/machbase/neo-server/graph/badge.svg?token=4IJ83M8R0B)](https://codecov.io/gh/machbase/neo-server)

# neo-server

Machbase is the fastest time-series database for [IoT in the world](https://www.tpc.org/tpcx-iot/results/tpcxiot_perf_results5.asp?version=2) implemented in C++. 
`machbase-neo` is an IoT Database Server that embedding the Machbase engine and provides essential and convenient features to build IoT platforms such as MQTT and HTTP API. It can be installed on machines ranging from Raspberry PI to high-performance servers.

API and Interfaces

- [x] HTTP : Applications and Sensors read/write data via HTTP REST API
- [x] MQTT : Sensors write data via MQTT protocol
- [x] gRPC : The first class API for extensions
- [x] SSH : Command line interface for human and batch process
- [x] WEB UI (Batteries included)
- [x] [UI API](https://machbase.com/neo/api-http/ui-api) to build custom UI (Batteries replaceable)

Bridges integrated with external systems

- [x] SQLite
- [x] PostgreSQL
- [x] MySQL
- [x] MS-SQL
- [x] MQTT Broker
- [ ] Kafka
- [ ] NATS

## Documents

[https://machbase.com/neo](https://machbase.com/neo)

## Web User Interface

- SQL
![screen](./docs/screenshot02.jpg)

- TQL Script
![screen](./docs/screenshot01.jpg)

## Install Prebuilt Binary

- Download

```sh
sh -c "$(curl -fsSL https://machbase.com/install.sh)"
```

- Unzip the archive file

## Install Using Docker

```sh
docker pull machbase/machbase-neo
```

https://hub.docker.com/r/machbase/machbase-neo

## Build from sources

- Install Go 1.20 or higher
- Require C compiler and linker (e.g: gcc) 
- Checkout machbase/neo-server
- Execute `go run mage.go machbase-neo`
- Find the executable binary `./tmp/machbase-neo`

### Dependency

![deps](./docs/deps.png)

- [neo-server](https://github.com/machbase/neo-server) machbase-neo source code
- [neo-spi](https://github.com/machbase/neo-spi) defines interfaces accessing database
- [neo-grpc](https://github.com/machbase/neo-grpc) implements spi accessing database via gRPC
- [neo-engine](https://github.com/machbase/neo-engine) implements spi accessing database via C API
