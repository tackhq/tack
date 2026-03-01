# Bolt

A Go-based configuration management and system bootstrapping tool inspired by Ansible.

Bolt uses simple YAML playbooks to automate system setup and configuration on macOS and Linux systems. It's distributed as a single binary with no runtime dependencies.

## Installation

### Homebrew (macOS/Linux)

```bash
brew install eugenetaranov/tap/bolt
```

### Download Binary

Download the latest release from the [releases page](https://github.com/eugenetaranov/bolt/releases).

### Go Install

```bash
go install github.com/eugenetaranov/bolt/cmd/bolt@latest
```

### Build from Source

```bash
git clone https://github.com/eugenetaranov/bolt.git
cd bolt
make build
sudo make install
```

## Features

- **Simple YAML playbooks** - Declarative configuration with familiar syntax
- **Ansible-compatible roles** - Reusable role structure with tasks, handlers, vars
- **Idempotent operations** - Safe to run multiple times
- **Cross-platform** - Supports macOS and Linux
- **Remote playbook sources** - Run playbooks directly from git repos, S3, or HTTP URLs
- **Multiple connectors** - Local, Docker, SSH, AWS SSM (planned)
- **Built-in modules** - Package management, file operations, commands
- **Variable interpolation** - Dynamic configuration with `{{ variables }}`
- **System facts** - Auto-detected OS, architecture, and environment info
- **No dependencies** - Single static binary

## Quick Start

**Try it now** (after cloning):

```bash
# Build and run example playbook
make build && ./bin/bolt run examples/playbooks/setup-dev.yaml --dry-run
```

**Try with Docker:**

```bash
# Test a role in a Docker container (container is reused across runs)
make build && ./bin/bolt test examples/roles-demo/roles/webserver
```

**Bootstrap macOS:**

```bash
# Install dev tools, apps, and configure your Mac
bolt run examples/playbooks/macos-setup.yaml --dry-run  # preview
bolt run examples/playbooks/macos-setup.yaml            # apply
```

**Run from a git repo:**

```bash
# Run a playbook directly from a git repo
bolt run git@github.com:user/repo.git//path/to/playbook.yaml

# Pin to a branch or tag
bolt run git@github.com:user/repo.git@main//path/to/playbook.yaml
bolt run https://github.com/user/repo.git@v1.0//playbook.yaml

# Run from an HTTP URL
bolt run https://example.com/playbook.yaml

# Run from S3
bolt run s3://bucket/path/to/playbook.yaml
```

**CLI usage:**

```bash
# Run a playbook
bolt run playbook.yaml

# Dry run (see what would happen)
bolt run playbook.yaml --dry-run

# Validate syntax without running
bolt validate playbook.yaml

# List available modules
bolt modules

# Scaffold a new role with sample files
bolt scaffold myrole
bolt scaffold myrole --path ./my-roles

# Test a role in a Docker container
bolt test myrole
bolt test myrole --new       # force fresh container
bolt test myrole --rm        # remove container after run
```

## Generating Playbooks from Live Systems

`bolt generate` connects to a target system, reads the current state of specified resources, and outputs a ready-to-use playbook.

```bash
# Capture installed packages (auto-detects brew/apt/dnf)
bolt generate --packages neovim,tmux,ripgrep

# Capture files, services, and users from a remote host
bolt generate -c ssh://root@web1 \
  --packages nginx \
  --files /etc/nginx/nginx.conf,/etc/systemd/system/myapp.service \
  --services nginx,myapp \
  --users deploy,app \
  -o playbook.yaml

# Use sudo for privileged queries
bolt generate -c ssh://deploy@web1 --services nginx --sudo
```

Resource flags (use any combination):
- `--packages` — installed packages (apt, brew, or dnf/yum based on target OS)
- `--files` — file content/permissions, directory permissions, symlink targets
- `--services` — systemd unit enabled/running state
- `--users` — user existence, uid, groups, shell, home directory

## Scaffolding Roles

`bolt scaffold` creates a new role directory with sample files demonstrating all resource types.

```bash
# Create roles/myrole/ with sample tasks, handlers, vars, files, and templates
bolt scaffold myrole

# Create in a custom directory
bolt scaffold myrole --path ./my-roles
```

Generated structure:

```
roles/myrole/
├── tasks/main.yaml       # Sample tasks: packages, copy, file, systemd, template
├── handlers/main.yaml    # Sample handler (restart service)
├── defaults/main.yaml    # Default variables referenced by tasks
├── vars/main.yaml        # Role variables (placeholder)
├── files/config.txt      # Sample static file for copy module
└── templates/app.conf.j2 # Sample Go template
```

## Testing Roles

`bolt test` runs a role or playbook inside a Docker container. Containers are **reused by default** — the container name is derived from the target, so repeated runs hit the same container. This lets you verify idempotency: the first run applies changes, the second run should show no drift.

```bash
# Run a role — creates container "bolt-test-webserver", keeps it after
bolt test myrole

# Second run reuses the container — verify idempotency
bolt test myrole

# Force a fresh container (removes existing first)
bolt test myrole --new

# Remove container after the run
bolt test myrole --rm

# Fresh + disposable (one-shot, like the old default)
bolt test myrole --new --rm

# Use a different base image
bolt test myrole --image debian:12

# Test a playbook file directly
bolt test setup.yaml
```

## Examples

### Local Machine Setup

```yaml
name: Setup Development Machine
hosts: localhost
connection: local

vars:
  projects_dir: "{{ env.HOME }}/projects"

tasks:
  - name: Install packages
    brew:
      name: [git, go, ripgrep]
      state: present
    when: facts.os_family == 'Darwin'

  - name: Create projects directory
    file:
      path: "{{ projects_dir }}"
      state: directory
      mode: "0755"
```

### Docker Container

```yaml
name: Configure Container
hosts: my-container
connection: docker

tasks:
  - name: Install packages
    command:
      cmd: apt-get update && apt-get install -y curl vim

  - name: Create app directory
    file:
      path: /app
      state: directory
      mode: "0755"

  - name: Copy config file
    copy:
      dest: /app/config.yaml
      content: |
        server:
          port: 8080
        logging:
          level: info
```

```bash
# Start container, run playbook
docker run -d --name my-container ubuntu:22.04 sleep 600
bolt run container-setup.yaml
```

### Remote Host via SSH

```yaml
name: Configure Web Server
hosts: web1
connection: ssh

vars:
  bolt_ssh_user: deploy
  bolt_ssh_key: ~/.ssh/deploy_key

tasks:
  - name: Install nginx
    apt:
      name: nginx
      state: present

  - name: Copy site config
    copy:
      dest: /etc/nginx/sites-available/app.conf
      content: |
        server {
          listen 80;
          root /var/www/app;
        }
```

```bash
# Run against a host defined in ~/.ssh/config
bolt run server-setup.yaml

# Override connection settings via CLI flags
bolt run server-setup.yaml --ssh-user deploy --ssh-key ~/.ssh/deploy_key

# Skip host key verification for new hosts
bolt run server-setup.yaml --ssh-insecure

# Use password authentication
bolt run server-setup.yaml --ssh-user admin --ssh-password

# Target multiple hosts
bolt run server-setup.yaml --hosts web1,web2,web3
```

SSH connection settings can be provided via playbook vars, CLI flags, or environment variables (`BOLT_SSH_USER`, `BOLT_SSH_PORT`, `BOLT_SSH_KEY`, `BOLT_SSH_PASSWORD`, `BOLT_SSH_INSECURE`). CLI flags take highest precedence, then environment variables, then playbook values. Host settings from `~/.ssh/config` (HostName, User, Port, IdentityFile) are resolved automatically.

## Documentation

| Document | Description |
|----------|-------------|
| [Getting Started](docs/getting-started.md) | Installation and first steps |
| [Playbooks](docs/playbooks.md) | Playbook structure, tasks, handlers, loops |
| [Roles](docs/roles.md) | Reusable role structure |
| [Modules](docs/modules.md) | Available modules reference |
| [Variables & Facts](docs/variables.md) | Variable interpolation and system facts |
| [Connectors](docs/connectors.md) | Connection methods (local, Docker, SSH, SSM) |

## Available Modules

| Module | Description |
|--------|-------------|
| `apt` | Manage packages on Debian/Ubuntu |
| `brew` | Manage Homebrew packages on macOS |
| `command` | Execute shell commands |
| `copy` | Copy files or write content |
| `file` | Manage files, directories, and symlinks |
| `template` | Render templates with variable substitution |

## Project Structure

```
bolt/
├── cmd/bolt/           # CLI entrypoint
├── internal/
│   ├── connector/      # Connection backends (local, docker, ssh, ssm)
│   ├── executor/       # Playbook execution engine
│   ├── module/         # Task modules (apt, brew, file, etc.)
│   ├── output/         # Formatted terminal output
│   ├── playbook/       # YAML parsing
│   └── source/         # Remote playbook sources (git, s3, http)
├── pkg/facts/          # System fact gathering
├── tests/integration/  # Integration tests (testcontainers)
├── docs/               # Documentation
└── examples/           # Example playbooks
```

## Development

```bash
# Build for current platform
make build

# Build for all platforms (cross-compile)
make build-all

# Run unit tests
make test

# Run integration tests (requires Docker)
make test-integration

# Run linter
make lint
```

### Integration Tests

Integration tests use [testcontainers-go](https://golang.testcontainers.org/) to spin up a Docker container, run a playbook against it, and validate the results with Go assertions.

```bash
# Run integration tests
go test -v ./tests/integration/...

# Skip integration tests (short mode)
go test -short ./...
```

## Requirements

- **Running**: macOS or Linux
- **Building**: Go 1.21+

## License

MIT License
