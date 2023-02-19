
[![](https://img.shields.io/github/v/release/machbase/machbase-neo?sort=semver)](https://github.com/machbase/machbase-neo/releases)
[![](https://github.com/machbase/neo-server/actions/workflows/ci-linux-arm64.yml/badge.svg)](https://github.com/machbase/neo-server/actions/workflows/ci-linux-arm64.yml)
[![](https://github.com/machbase/neo-server/actions/workflows/ci-linux-amd64.yml/badge.svg)](https://github.com/machbase/neo-server/actions/workflows/ci-linux-amd64.yml)
[![](https://github.com/machbase/neo-server/actions/workflows/ci-darwin-arm64.yml/badge.svg)](https://github.com/machbase/neo-server/actions/workflows/ci-darwin-arm64.yml)

## Nightly Build Packages

- [machbase-neo docs](https://neo.machbase.com/)

### Build dependency

```mermaid
flowchart LR

SPI[neo-spi] -->|impl| E
SPI-->|impl|R

E[neo-engine]
E --> S[neo-server]

R -->|server impl| S

R[neo-grpc] -->|client impl| SH[neo-shell]
SH --> S

```