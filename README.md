
[![](https://img.shields.io/github/v/release/machbase/machbase-neo?sort=semver)](https://github.com/machbase/machbase-neo/releases)
[![](https://github.com/machbase/neo-server/actions/workflows/ci-linux-arm64.yml/badge.svg)](https://github.com/machbase/neo-server/actions/workflows/ci-linux-arm64.yml)
[![](https://github.com/machbase/neo-server/actions/workflows/ci-linux-amd64.yml/badge.svg)](https://github.com/machbase/neo-server/actions/workflows/ci-linux-amd64.yml)
[![](https://github.com/machbase/neo-server/actions/workflows/ci-darwin-arm64.yml/badge.svg)](https://github.com/machbase/neo-server/actions/workflows/ci-darwin-arm64.yml)

## Nightly Build Packages

- [nightly builds](https://github.com/machbase/neo-server/releases)

### Build dependency

```mermaid
flowchart LR

C[machbase] --> E

E[neo-engine]
E -->|go.mod| S[neo-server]

R[neo-grpc] -->|go.mod| SH[neo-shell]
SH -->|go.mod| S

```