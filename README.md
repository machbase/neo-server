
[![](https://img.shields.io/github/v/release/machbase/neo-server?sort=semver)](https://github.com/machbase/neo-server/releases)
[![](https://github.com/machbase/neo-server/actions/workflows/ci-main.yml/badge.svg)](https://github.com/machbase/neo-server/actions/workflows/ci-main.yml)
[![codecov](https://codecov.io/gh/machbase/neo-server/graph/badge.svg?token=4IJ83M8R0B)](https://codecov.io/gh/machbase/neo-server)

# machbase-neo

Machbase is a blazing fast time-series database designed specifically for IoT applications and implemented in C.
`machbase-neo` is an IoT Database Server that embeds the Machbase engine and offers essential and convenient features for building IoT platforms,
including MQTT and HTTP APIs. It is highly versatile and can be installed on a wide range of machines,
from Raspberry Pi devices to high-performance servers.

API and Interfaces

- [x] HTTP: Applications and sensors read/write data via HTTP REST API
- [x] MQTT: Sensors write data via MQTT protocol
- [x] SSH: Command line interface for human and batch processes
- [x] WEB UI (Batteries included)
- [x] UI API to build custom UI (Batteries replaceable)
- [x] A zero-dependency Go client for building applications, available at https://github.com/machbase/neo-client

## Documents

[https://docs.machbase.com/neo](https://docs.machbase.com/neo)

## API Token Authentication

HTTP and MQTT token authentication are enabled by default. Clients using these APIs
must provide valid API tokens.

To explicitly disable token authentication for trusted environments, start the server with:

```sh
machbase-neo serve --http-enable-token-auth=false --mqtt-enable-token-auth=false
```

Each flag controls its protocol independently. Existing custom configuration files
that explicitly disable token authentication retain their configured behavior.
MQTT TLS client-certificate authentication is unchanged.

## Install Prebuilt Binary

- Download

```sh
sh -c "$(curl -fsSL https://docs.machbase.com/install.sh)"
```

- Unzip the archive file

## Install Using Docker

```sh
docker pull machbase/machbase-neo
```

https://hub.docker.com/r/machbase/machbase-neo

### Build using docker

It is recommended to build machbase-neo using a container to ensure a consistent and reproducible build environment.

- Checkout `machbase/neo-server`

- Prepare build container

```sh
docker build -t centos-build-env -f ./scripts/CentOS7.Dockerfile .
```

- Run build container

```sh
docker run --rm -v ./tmp:/app/tmp -v ./packages:/app/packages centos-build-env
```

- Find the executable binary in `./tmp/machbase-neo` and pakcage zip file in `./packages`.

## Build from Sources

- Install Go latest version
- Require a C compiler and linker (e.g., gcc)
- Checkout `machbase/neo-server`
- Execute `go run mage.go machbase-neo` to build `machbase-neo`
- Find the executable binary in `./tmp/machbase-neo`

The build automatically installs the versions specified in
`magefiles/neo-web-version.txt` and `magefiles/neo-engine-version.txt`. Release
archives are cached by version and platform under `tmp/release-cache`, so
switching branches can reuse previously downloaded versions. The web UI and
engine library are restored from the cache if they differ from the selected
release. `go run mage.go test`, `testN`, and `testAll` also ensure that the
selected engine is installed before running tests. The checked-in engine
header is not replaced.

Run `go run mage.go install-neo-web` or `go run mage.go install-neo-engine` to
download the selected release again and refresh its cache. A build replaces
locally modified embedded UI files; use the server's `--ui` option to test a
local web build instead.

## Web User Interface

- SQL
![screen](./docs/screenshot02.jpg)

- TQL: Script for transforming data
![screen](./docs/screenshot01.jpg)

- Dashboard: Data Monitoring
![screen](./docs/dashboard.png)

- Geodetic Data Visualization
![screen](./docs/screenshot03.jpg)

- Data Visualization<br/>
Find more samples in the [documents](https://docs.machbase.com/neo/tql/chart/).
![charts](./docs/charts_demo.jpg)
