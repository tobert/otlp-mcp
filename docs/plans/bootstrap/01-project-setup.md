# Task 01: Project Setup

## Why

Initialize the Go project with proper module configuration, directory structure, and core dependencies needed for building an OTLP MCP server.

## What

Create:
- `go.mod` with Go 1.25+ requirement
- Directory structure for clean package organization
- Initial dependency list
- `.gitignore` for Go projects
- Basic `README.md` update

## Approach

### Directory Structure
```
otlp-mcp/
├── cmd/
│   └── otlp-mcp/          # Main binary entry point
│       └── main.go
├── internal/
│   ├── otlpreceiver/      # OTLP gRPC server (from otel-cli)
│   ├── storage/           # Ring buffer implementation
│   ├── mcpserver/         # MCP stdio server
│   └── cli/               # CLI framework integration
├── pkg/                   # (Reserved for future exportable packages)
├── docs/
│   └── plans/
│       └── bootstrap/     # This directory
├── go.mod
├── go.sum
├── README.md
├── LICENSE                # MIT (already exists)
└── CLAUDE.md              # Already exists
```

### Core Dependencies

**Required:**
- `go.opentelemetry.io/proto/otlp` - OTLP protobuf definitions
- `google.golang.org/grpc` - gRPC server
- `google.golang.org/protobuf` - Protobuf support
- `github.com/modelcontextprotocol/go-sdk` - Official MCP SDK (maintained with Google)

**CLI Framework:**
- `github.com/urfave/cli/v3` - Modern, actively maintained, clean API

**Testing/Development:**
- Standard library `testing` package
- `go.opentelemetry.io/otel` - For creating test spans

### Go Module Initialization

```bash
go mod init github.com/tobert/otlp-mcp
go mod edit -go=1.25
```

## Dependencies

None - this is the first task.

## Acceptance Criteria

- [ ] `go.mod` exists with `go 1.25` requirement
- [ ] All directories created
- [ ] `cmd/otlp-mcp/main.go` exists with basic package main and hello world
- [ ] Project builds: `go build ./...`
- [ ] Tests pass: `go test ./...` (even if no tests yet)
- [ ] `go mod tidy` runs without errors
- [ ] README.md updated with basic build instructions

## Implementation Notes

### Minimal main.go

```go
package main

import "fmt"

func main() {
    fmt.Println("otlp-mcp: OTLP MCP server")
    // CLI framework and server initialization will be added in task 02
}
```

### .gitignore

```
# Binaries
otlp-mcp
/bin/
/dist/

# Go build artifacts
*.exe
*.exe~
*.dll
*.so
*.dylib
*.test
*.out

# Coverage
*.coverprofile
coverage.out
coverage.html

# IDE
.idea/
.vscode/
*.swp
*.swo
*~

# OS
.DS_Store
Thumbs.db

# Goreleaser
dist/
```

## Testing

```bash
# Verify structure
go build ./cmd/otlp-mcp
./otlp-mcp

# Should print: "otlp-mcp: OTLP MCP server"
```

## Status

Status: pending
Next: 02-cli-framework.md
