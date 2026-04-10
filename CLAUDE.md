# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

knoway is an Envoy-inspired LLM gateway — think Nginx but for LLMs. It proxies OpenAI-compatible API requests to upstream LLM backends with filtering (auth, rate limiting, transformation), observability, and Kubernetes-native CRD configuration.

## Build & Development Commands

```bash
make format          # Format Go + Proto (golangci-lint --fix, goimports, gofmt)
make lint            # Run golangci-lint
make unit-test       # Run all tests with race detector and coverage
make gen             # Regenerate proto + CRDs + format
make gen-crds        # Regenerate K8s CRDs only
make build-binaries  # Cross-platform binary build
make images          # Build Docker images
make helm            # Package Helm chart
```

Run a single test:
```bash
go test -v -run TestFunctionName ./pkg/path/to/package/...
```

Proto generation requires `buf`. CRD generation requires `controller-gen` (auto-downloaded to `./bin/`).

## Architecture

### Request Flow

```
HTTP Request → listener.Mux (route matching)
  → Listener (chat/image/tts) → Filter Chain (auth → ratelimit → transform)
    → Cluster (upstream backend) → Response filters → Client
```

### Key Abstractions

- **Listeners** (`pkg/listener/`): Protocol handlers for different API types. `listener.Mux` routes requests to the right listener. Implementations in `pkg/listener/manager/` (chat, image, tts).
- **Clusters** (`pkg/clusters/`): Upstream LLM backend abstraction. Each cluster wraps a backend endpoint with its own filter chain (`pkg/clusters/filters/`).
- **Filters** (`pkg/filters/`): Request/response middleware — auth (gRPC-based), rate limiting (token/prompt-based), request transformation. Registered via `pkg/registry/`.
- **Routes** (`pkg/route/`): Match requests to clusters by model name and other criteria.
- **BootKit** (`pkg/bootkit/`): Application lifecycle orchestrator — manages ordered start/stop hooks for all components.

### Configuration

Two modes:
1. **Static** (`--static-cluster-only`): YAML config file with `staticListeners` and `staticClusters` arrays. Config types defined via Protocol Buffers.
2. **Kubernetes CRDs**: `LLMBackend`, `ImageGenerationBackend`, `ModelRoute` — reconciled by controllers in `internal/controller/`.

All filter/cluster/listener configs are protobuf-defined in `api/` and registered in `pkg/registry/`.

### Entry Points

- `cmd/main.go` → flag parsing, config loading, dispatches to gateway/server/admin
- `cmd/gateway/` → HTTP proxy server (the core functionality)
- `cmd/server/` → K8s controller-runtime manager
- `cmd/admin/` → Admin gRPC/HTTP introspection API

## Code Conventions

- Go module: `knoway.dev`
- Use `goimports -local knoway.dev` for import ordering
- Tests use `github.com/stretchr/testify`
- Linter config in `.golangci.yml` — many linters enabled by default with specific exclusions
- Proto files in `api/`, generated code also lands in `api/`
- K8s types in `apis/` (the `llm.knoway.dev` API group)
