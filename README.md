# Bolt

A Go-based configuration management and system bootstrapping tool inspired by Ansible. Single binary, no dependencies.

## Installation

```bash
brew install eugenetaranov/tap/bolt          # Homebrew
go install github.com/eugenetaranov/bolt/cmd/bolt@latest  # Go
```

Or download from the [releases page](https://github.com/eugenetaranov/bolt/releases).

## Quick Start

```bash
bolt run examples/playbooks/setup-dev.yaml --check   # preview changes
bolt run examples/playbooks/setup-dev.yaml            # apply
```

```yaml
# setup.yaml
name: Setup Development Machine
hosts: localhost
connection: local

tasks:
  - name: Install packages
    brew:
      name: [git, go, ripgrep]
      state: present
    when: facts.os_family == 'Darwin'

  - name: Create projects directory
    file:
      path: "{{ env.HOME }}/projects"
      state: directory
```

## Features

- **Simple YAML playbooks** with Ansible-compatible role structure
- **Idempotent modules** -- apt, brew, yum, file, copy, command, systemd, template
- **Cross-platform** -- macOS (brew) and Linux (apt, yum, systemd)
- **Multiple connectors** -- Local, Docker, SSH, AWS SSM with tag-based discovery
- **Plan/apply workflow** -- preview changes before applying, `--auto-approve` for CI
- **Parallel execution** -- `--forks N` for concurrent multi-host runs
- **Variable system** -- interpolation, filters, registered outputs, vault encryption
- **System facts** -- OS, arch, network, EC2 metadata (via IMDSv2)
- **Remote sources** -- run playbooks from git repos, S3, or HTTP URLs
- **No dependencies** -- single static binary

## Connectors

| Connector | Example | Description |
|-----------|---------|-------------|
| **Local** | `connection: local` | Run on the current machine (default) |
| **Docker** | `-c docker://container` | Run inside a Docker container |
| **SSH** | `-c ssh://user@host:port` | Connect via SSH; reads `~/.ssh/config` |
| **SSM** | `--ssm-tags env=prod` | AWS SSM; tag-based instance discovery, S3 file transfer |

Connection type is auto-detected from flags when not specified. See [Connectors docs](docs/connectors.md) for full configuration and environment variables.

## Multi-Host & Parallel Execution

```bash
# Serial (default)
bolt run deploy.yaml --hosts web1,web2,web3

# Parallel -- up to 5 hosts at once
bolt run deploy.yaml --hosts web1,web2,web3,web4,web5 --forks 5

# With inventory groups
bolt run deploy.yaml -i inventory.yaml --hosts webservers --forks 10
```

Output is buffered per-host and flushed in order. Errors on one host don't stop others. Use `BOLT_FORKS` env var for CI.

## Playbook Examples

### Remote Host via SSH

```yaml
name: Configure Web Server
hosts: web1
connection: ssh
ssh:
  user: deploy
  key: ~/.ssh/deploy_key

tasks:
  - name: Install nginx
    apt:
      name: nginx
      state: present

  - name: Enable nginx
    systemd:
      name: nginx
      state: started
      enabled: true
```

```bash
bolt run server-setup.yaml
bolt run server-setup.yaml --hosts web1,web2,web3 --forks 3
```

### AWS SSM with Tag-Based Discovery

```yaml
name: Patch App Servers
connection: ssm
ssm:
  region: us-east-1
  bucket: my-ssm-transfer-bucket
  tags:
    env: production
    role: app-server

tasks:
  - name: Install security updates
    apt:
      name: "*"
      state: latest
```

### Remote Playbook Sources

```bash
bolt run git@github.com:user/repo.git//path/to/playbook.yaml
bolt run https://github.com/user/repo.git@v1.0//playbook.yaml
bolt run s3://bucket/path/to/playbook.yaml
```

## Inventory Files

Define hosts, groups, SSH config, and variables in a reusable file:

```yaml
# inventory.yaml
hosts:
  web1:
    ssh: { user: deploy, key: ~/.ssh/id_deploy }
    vars: { region: us-east-1 }
  web2:
    ssh: { user: deploy }

groups:
  webservers:
    hosts: [web1, web2]
    vars: { app_port: 8080 }
```

```bash
bolt run deploy.yaml -i inventory.yaml --hosts webservers
```

See [`examples/inventory.yaml`](examples/inventory.yaml) for a complete sample.

## Available Modules

| Module | Description |
|--------|-------------|
| `apt` | Manage packages on Debian/Ubuntu |
| `brew` | Manage Homebrew packages on macOS |
| `yum` | Manage packages on RHEL/CentOS/Fedora (auto-detects dnf) |
| `command` | Execute shell commands |
| `copy` | Copy files or write inline content |
| `file` | Manage files, directories, and symlinks |
| `systemd` | Manage systemd services (start, stop, enable, mask, daemon-reload) |
| `template` | Render Go templates with variable substitution |

Run `bolt module <name>` for detailed parameter docs.

## Tooling

```bash
bolt generate --packages nginx --files /etc/nginx/nginx.conf -c ssh://root@web1  # capture live state
bolt scaffold myrole          # create role boilerplate
bolt test myrole              # test in Docker container (reused for idempotency checks)
bolt validate playbook.yaml   # syntax check
bolt vault encrypt secrets.yaml   # encrypt variables file
```

## Documentation

| Document | Description |
|----------|-------------|
| [Getting Started](docs/getting-started.md) | Installation, first playbook, CLI reference |
| [Playbooks](docs/playbooks.md) | Tasks, handlers, loops, conditionals |
| [Roles](docs/roles.md) | Reusable role structure |
| [Modules](docs/modules.md) | All modules with parameters and examples |
| [Variables & Facts](docs/variables.md) | Interpolation, filters, system/network/EC2 facts |
| [Connectors](docs/connectors.md) | Local, Docker, SSH, SSM configuration |
| [Development](docs/development.md) | Building, testing, project structure |

## License

MIT License
