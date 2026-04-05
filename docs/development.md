# Development

## Requirements

- Go 1.24+
- Docker (for integration tests and `tack test`)
- golangci-lint (for linting)

## Project Structure

```
tack/
├── cmd/tack/           # CLI entrypoint (Cobra)
├── internal/
│   ├── connector/      # Connection backends (local, docker, ssh, ssm)
│   ├── executor/       # Playbook execution engine + parallel host support
│   ├── generate/       # tack generate command
│   ├── module/         # Task modules (apt, brew, yum, file, copy, command, systemd, template)
│   ├── output/         # Formatted terminal and JSON output
│   ├── playbook/       # YAML parsing, variable interpolation, conditions
│   ├── source/         # Remote playbook sources (git, s3, http)
│   ├── testrun/        # tack test command
│   └── vault/          # Encrypted vault file support
├── pkg/
│   ├── facts/          # System fact gathering (OS, arch, network, EC2)
│   └── ssmparams/      # AWS SSM Parameter Store client
├── tests/integration/  # Integration tests (testcontainers)
├── docs/               # Documentation
└── examples/           # Example playbooks and roles
```

## Build & Run

```bash
make build              # Build for current platform (output: ./bin/tack)
make test               # Run unit tests with race detector
make lint               # Run golangci-lint
go run ./cmd/tack       # Run directly without building
```

## Testing

```bash
# Unit tests (skip integration)
go test -short ./...

# Integration tests (requires Docker)
make test-integration

# Or directly
go test -v ./tests/integration/...
```

Integration tests use [testcontainers-go](https://golang.testcontainers.org/) to spin up Docker containers, run playbooks against them, and validate results.

## Releasing

See [RELEASING.md](../RELEASING.md) for release instructions. Releases are automated via GoReleaser and GitHub Actions.
