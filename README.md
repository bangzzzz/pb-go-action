# pb-go-action

A GitHub Action written in Go that detects changes to Protobuf IDL files,
generates the corresponding Go code with `protoc`, and pushes it to the
configured target GitHub repositories so that other services can pull it as a
regular Go module dependency.

---

## Features

| Feature | Detail |
|---|---|
| **Change detection** | Computes a reverse import graph so that changing a shared/common `.proto` file automatically triggers regeneration for every service that (transitively) imports it. |
| **Branch mirroring** | Generated code is pushed to the same branch name in the target repo.  Merging to `master` pushes to `master`. |
| **Extensible plugins** | Any `protoc-gen-*` plugin can be configured.  `protoc-gen-go` and `protoc-gen-go-grpc` are pre-installed in the Docker image. |
| **Idempotent pushes** | A commit is only created when the generated files actually differ from what is already in the target repo. |

---

## Quick Start

### 1. Add the action to your workflow

```yaml
# .github/workflows/pb-go.yml
name: Generate Protobuf Go Code

on:
  push:
    paths:
      - '**/*.proto'

jobs:
  generate:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0   # needed for git diff

      - uses: bangzzzz/pb-go-action@v1
        with:
          github_token: ${{ secrets.PB_GO_TOKEN }}
          # config: .pb-go-action.yml   # default
```

> **Note** – `secrets.PB_GO_TOKEN` must have **Contents: write** permission on
> every target repository.

### 2. Create the config file

```yaml
# .pb-go-action.yml  (in the root of your IDL repository)

services:
  - name: user-service
    proto_dir: proto/user           # directory containing *.proto for this service
    include_dirs:                   # extra -I paths for protoc (shared/common protos)
      - proto/common
    target_repo: my-org/user-pb-go  # owner/repo of the generated-code repository
    go_module: github.com/my-org/user-pb-go  # module name written into go.mod

  - name: order-service
    proto_dir: proto/order
    target_repo: my-org/order-pb-go
    go_module: github.com/my-org/order-pb-go

plugins:
  - name: go
    out: .
    opt:
      - paths=source_relative

  - name: go-grpc
    out: .
    opt:
      - paths=source_relative
      - require_unimplemented_servers=false
```

---

## Inputs

| Name | Required | Default | Description |
|---|---|---|---|
| `github_token` | **yes** | — | Token used to push to target repos. |
| `config` | no | `.pb-go-action.yml` | Path to the config file (relative to the workspace root). |
| `changed_files` | no | _(auto)_ | Newline-separated list of changed `.proto` files.  If omitted the action auto-detects changes via `git diff`. |
| `git_user_name` | no | `PB Go Action` | Commit author name used in target repos. |
| `git_user_email` | no | `pb-go-action[bot]@users.noreply.github.com` | Commit author e-mail used in target repos. |

---

## Config Reference

### `services[]`

| Field | Required | Description |
|---|---|---|
| `name` | **yes** | Human-readable label (used in logs). |
| `proto_dir` | **yes** | Directory with `.proto` files for this service (relative to repo root). |
| `include_dirs` | no | Additional `-I` include paths for `protoc` (relative to repo root). |
| `target_repo` | **yes** | GitHub repository slug (`owner/repo`) receiving generated code. |
| `go_module` | no | Go module name.  If set, a `go.mod` is created/kept in the target repo. |

### `plugins[]`

Each entry maps to a `protoc-gen-<name>` binary that must exist in `PATH`.

| Field | Required | Default | Description |
|---|---|---|---|
| `name` | **yes** | — | Plugin name, e.g. `go`, `go-grpc`. |
| `out` | no | `.` | Output directory relative to the per-service generated root. |
| `opt` | no | `[]` | List of plugin options passed via `--<name>_opt`. |

Pre-installed plugins (in the Docker image):

- `protoc-gen-go` (google.golang.org/protobuf)
- `protoc-gen-go-grpc` (google.golang.org/grpc)

To use additional plugins (e.g. `protoc-gen-validate`) add an installation step
**before** this action in your workflow:

```yaml
- name: Install extra protoc plugins
  run: |
    go install github.com/envoyproxy/protoc-gen-validate/cmd/protoc-gen-validate@latest
    echo "$HOME/go/bin" >> $GITHUB_PATH
```

---

## Repository Layout Example

```
my-idl-repo/
├── .github/
│   └── workflows/
│       └── pb-go.yml
├── .pb-go-action.yml
├── proto/
│   ├── common/
│   │   └── base.proto
│   ├── user/
│   │   └── user.proto        ← imports common/base.proto
│   └── order/
│       └── order.proto
```

Changing `proto/common/base.proto` will trigger regeneration for **both**
`user-service` and `order-service` because both import it.

---

## How It Works

```
Push event
    │
    ▼
 git diff (before → after SHAs from GITHUB_EVENT_PATH)
    │
    ▼
 Parse all .proto imports → build reverse import graph
    │
    ▼
 Expand changed files through graph → affected services
    │
    ▼
 For each affected service:
   └─ protoc [plugins] proto/*.proto  →  /tmp/pb-go-action-XXXX/
   └─ git clone target_repo @ branch
   └─ copy generated files
   └─ git commit + push
```

---

## Development

```bash
# Build
go build ./...

# Test
go test ./...

# Build Docker image
docker build -t pb-go-action .
```
