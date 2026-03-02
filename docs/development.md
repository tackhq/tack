# Development

## Requirements

- Go 1.21+
- Docker (for integration tests and `bolt test`)

## Project Structure

```
bolt/
├── cmd/bolt/           # CLI entrypoint
├── internal/
│   ├── connector/      # Connection backends (local, docker, ssh, ssm)
│   ├── executor/       # Playbook execution engine
│   ├── module/         # Task modules (apt, brew, file, systemd, etc.)
│   ├── output/         # Formatted terminal output
│   ├── playbook/       # YAML parsing
│   └── source/         # Remote playbook sources (git, s3, http)
├── pkg/
│   ├── facts/          # System fact gathering (OS, arch, EC2)
│   └── ssmparams/      # AWS SSM Parameter Store client
├── tests/integration/  # Integration tests (testcontainers)
├── docs/               # Documentation
└── examples/           # Example playbooks
```

## Build & Run

```bash
# Build for current platform
make build

# Build for all platforms (cross-compile)
make build-all

# Run directly without building
go run ./cmd/bolt

# Install to /usr/local/bin
sudo make install
```

## Testing

```bash
# Run unit tests
make test

# Run linter
make lint
```

### Integration Tests

Integration tests use [testcontainers-go](https://golang.testcontainers.org/) to spin up a Docker container, run a playbook against it, and validate the results with Go assertions.

```bash
# Run integration tests (requires Docker)
make test-integration

# Or directly with go test
go test -v ./tests/integration/...

# Skip integration tests (short mode)
go test -short ./...
```
