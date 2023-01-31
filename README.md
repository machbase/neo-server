
[![](https://img.shields.io/github/v/release/machbase-neo/machbase?sort=semver)](https://github.com/machbase/machbase-neo/releases)
[![](https://github.com/machbase/neo-server/actions/workflows/ci-linux-arm64.yml/badge.svg)](https://github.com/machbase/neo-server/actions/workflows/ci-linux-arm64.yml)
[![](https://github.com/machbase/neo-server/actions/workflows/ci-linux-amd64.yml/badge.svg)](https://github.com/machbase/neo-server/actions/workflows/ci-linux-amd64.yml)
[![](https://github.com/machbase/neo-server/actions/workflows/ci-darwin-arm64.yml/badge.svg)](https://github.com/machbase/neo-server/actions/workflows/ci-darwin-arm64.yml)


### Build dependency

```mermaid
flowchart LR

E[neo-engine]
E -->|go.mod| S[neo-server]

R[neo-grpc] -->|go.mod| SH[neo-shell]
SH -->|go.mod| S

```