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

# Paste a GitHub/GitLab browse URL directly
bolt run https://github.com/user/repo/tree/main/path/to/role

# Run from an HTTP URL or S3
bolt run https://example.com/playbook.yaml
bolt run s3://bucket/path/to/playbook.yaml
```

## Features

- **Simple YAML playbooks** — Declarative configuration with familiar syntax
- **Ansible-compatible roles** — Reusable role structure with tasks, handlers, vars, files, templates
- **Idempotent operations** — Safe to run multiple times
- **Cross-platform** — Supports macOS (brew) and Linux (apt, systemd)
- **Multiple connectors** — Local, Docker, SSH, and AWS SSM with tag-based instance discovery
- **Inventory files** — Define hosts, groups, per-host SSH config, and variables in a reusable YAML file
- **Remote playbook sources** — Run playbooks directly from git repos, S3, or HTTP URLs
- **SSM Parameter Store** — Fetch secrets at runtime with `ssm_param()` in vars and templates
- **EC2 instance facts** — Auto-gathered instance ID, region, instance type, AMI, and tags
- **Variable filters** — Transform values with `default`, `upper`, `lower`, `trim`, `join`, `first`, `last`, `length`, `int`, `bool`, `string`
- **Conditional execution** — `when`, `changed_when`, `failed_when` on any task
- **Task retries** — Automatic retries with configurable delay; `ignore_errors` to continue on failure
- **Generate playbooks** — `bolt generate` captures live system state into a playbook
- **Scaffold roles** — `bolt scaffold` creates a new role with sample files
- **Test in Docker** — `bolt test` runs roles in containers with idempotency verification
- **No dependencies** — Single static binary

## Connectors

Bolt supports four connection backends:

| Connector | Syntax | Description |
|-----------|--------|-------------|
| **Local** | `connection: local` | Run on the current machine |
| **Docker** | `connection: docker` | Run inside a Docker container (set `hosts` to container name) |
| **SSH** | `connection: ssh` or `-c ssh://user@host:port` | Connect via SSH; resolves `~/.ssh/config` automatically |
| **SSM** | `connection: ssm` | Connect via AWS SSM; supports tag-based discovery with `ResolveInstancesByTags` and S3 file transfer |

SSH connection settings can be provided via the playbook `ssh:` block, an inventory file, CLI flags, or environment variables (`BOLT_SSH_USER`, `BOLT_SSH_PORT`, `BOLT_SSH_KEY`, `BOLT_SSH_PASSWORD`, `BOLT_SSH_INSECURE`). Priority (highest first): CLI flags → playbook `ssh:` → per-host inventory → group inventory → `~/.ssh/config` → defaults.

The connection type is auto-detected when not explicitly set: SSH flags (`--ssh-user`, `--ssh-key`, etc.) or remote `--hosts` values infer `ssh`; SSM flags (`--ssm-instances`, `--ssm-tags`) infer `ssm`. SSH config aliases work directly — `bolt run --hosts myserver role/` resolves `myserver` via `~/.ssh/config`.

## Playbook Examples

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
bolt run server-setup.yaml --ssh-user deploy --ssh-key ~/.ssh/deploy_key
bolt run server-setup.yaml --hosts web1,web2,web3  # connection: ssh is auto-detected
bolt run --hosts web1 --ssh-user deploy role/       # run a role directory over SSH
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

  - name: Copy config file
    copy:
      dest: /app/config.yaml
      content: |
        server:
          port: 8080
```

### AWS SSM with Tag-Based Discovery

```yaml
name: Patch App Servers
connection: ssm

ssm:
  region: us-east-1
  bucket: my-ssm-transfer-bucket   # required for file upload/download
  tags:
    env: production
    role: app-server

tasks:
  - name: Install security updates
    apt:
      name: "*"
      state: latest

  - name: Restart app service
    systemd:
      name: myapp
      state: restarted
```

```bash
# Tags in the playbook — just run it
bolt run patch-app-servers.yaml

# Or pass tags on the CLI (no hosts needed, SSM is auto-detected)
bolt run patch-app-servers.yaml --ssm-tags env=production,role=app-server --ssm-region us-east-1

# Target specific instances directly
bolt run patch-app-servers.yaml --ssm-instances i-0abc123,i-0def456
```

## Inventory Files

An inventory file decouples *where to connect* from *what to run*. Define hosts, groups, per-host SSH settings, and variables once — then reference them from any playbook.

```yaml
# inventory.yaml

hosts:
  web1:
    ssh:
      user: deploy
      port: 22
      key: ~/.ssh/id_deploy
    vars:
      region: us-east-1

  web2:
    ssh:
      user: deploy
    vars:
      region: us-west-2

  db1:
    ssh:
      user: postgres
      host_key_checking: false
    vars:
      role: database

groups:
  webservers:
    hosts: [web1, web2]
    ssh:
      user: deploy          # group default; per-host ssh config takes priority
    vars:
      app_port: 8080

  # explicit instance IDs (ssm.instances or hosts: both work)
  prod-app:
    connection: ssm
    ssm:
      region: us-east-1
      bucket: my-ssm-transfer-bucket
      instances: [i-0abc1234, i-0def5678]
    vars:
      env: production

  # tag-based discovery — instances resolved at runtime
  prod-workers:
    connection: ssm
    ssm:
      region: us-east-1
      tags:
        env: production
        role: worker
```

Reference a group name in `hosts:`:

```yaml
name: Deploy Web Servers
connection: ssh
hosts: webservers         # expanded from inventory at runtime
```

Or target a group from the CLI:

```bash
bolt run deploy.yaml -i inventory.yaml --hosts webservers
bolt run patch.yaml  -i inventory.yaml --hosts prod-app
```

**Variable precedence** (highest → lowest): play `vars:` → per-host inventory vars → group inventory vars → role vars → role defaults.

**SSH config precedence** (highest → lowest): CLI flags → playbook `ssh:` → per-host inventory `ssh:` → group inventory `ssh:` → `~/.ssh/config` → defaults.

See [`examples/inventory.yaml`](examples/inventory.yaml) for a complete sample.

## SSM Parameter Store

Use `ssm_param()` in playbook vars or templates to fetch secrets from AWS SSM Parameter Store at runtime. SecureString parameters are automatically decrypted.

```yaml
vars:
  db_password: "{{ ssm_param('/myapp/prod/db_password') }}"
  api_key: "{{ ssm_param(api_key_path) }}"
```

## Generating Playbooks

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

Resource flags: `--packages`, `--files`, `--services`, `--users` (use any combination).

## Scaffolding Roles

`bolt scaffold` creates a new role directory with sample files demonstrating all resource types.

```bash
bolt scaffold myrole
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

`bolt test` runs a role or playbook inside a Docker container. Containers are **reused by default** so repeated runs verify idempotency: the first run applies changes, the second run should show no drift.

```bash
bolt test myrole                # creates container "bolt-test-myrole", keeps it
bolt test myrole                # reuses container — verify idempotency
bolt test myrole --new          # force fresh container
bolt test myrole --rm           # remove container after run
bolt test myrole --new --rm     # one-shot: fresh + disposable
bolt test myrole --image debian:12  # custom base image
bolt test setup.yaml            # test a playbook file directly
```

## Available Modules

| Module | Description |
|--------|-------------|
| `apt` | Manage packages on Debian/Ubuntu |
| `brew` | Manage Homebrew packages on macOS |
| `command` | Execute shell commands |
| `copy` | Copy files or write content |
| `file` | Manage files, directories, and symlinks |
| `systemd` | Manage systemd services (start, stop, enable, mask, daemon-reload) |
| `template` | Render templates with variable substitution |

## Documentation

| Document | Description |
|----------|-------------|
| [Getting Started](docs/getting-started.md) | Installation and first steps |
| [Playbooks](docs/playbooks.md) | Playbook structure, tasks, handlers, loops |
| [Roles](docs/roles.md) | Reusable role structure |
| [Modules](docs/modules.md) | Available modules reference |
| [Variables & Facts](docs/variables.md) | Variable interpolation and system facts |
| [Connectors](docs/connectors.md) | Connection methods (local, Docker, SSH, SSM) |
| [Development](docs/development.md) | Building, testing, and project structure |

## License

MIT License
