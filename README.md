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
- **Idempotent modules** -- apt, brew, yum, file, copy, command, systemd, template, lineinfile, blockinfile, wait_for
- **Cross-platform** -- macOS (brew) and Linux (apt, yum, systemd)
- **Multiple connectors** -- Local, Docker, SSH, AWS SSM with tag-based discovery
- **Plan/apply workflow** -- preview changes before applying, `--auto-approve` for CI
- **Parallel execution** -- `--forks N` for concurrent multi-host runs
- **Block/rescue/always** -- structured error handling with rollback and guaranteed cleanup
- **Task inclusion** -- `include_tasks` with scoped `vars:`, `loop:`, conditional `when:`, circular detection
- **Tag-based filtering** -- `--tags` and `--skip-tags` for selective task execution, with `always`/`never` special tags
- **Variable system** -- interpolation, filters, registered outputs, vars_files, vault encryption
- **JSON output** -- `--output json` for machine-readable output in CI pipelines
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

## Task Inclusion

Use `include_tasks` to include shared task files from your playbooks. This eliminates YAML duplication and enables reusable task libraries.

```yaml
tasks:
  # Basic include
  - name: Setup common packages
    include_tasks: tasks/common.yml

  # Include with scoped variables
  - name: Install nginx
    include_tasks: tasks/install-package.yml
    vars:
      package_name: nginx
      version: "1.24"

  # Conditional include
  - name: Debian-specific setup
    include_tasks: "{{ facts.os_family }}/packages.yml"
    when: facts.os_family == "Debian"

  # Loop-driven include
  - name: Configure services
    include_tasks: tasks/configure-service.yml
    loop:
      - nginx
      - redis
      - postgres
    loop_var: service_name
```

`include:` and `include_tasks:` are equivalent -- `include_tasks:` is the preferred form for consistency with Ansible conventions. Tasks are loaded and executed at runtime, supporting `when:` conditions, variable-interpolated paths, and `loop:` iteration.

Variables passed via `vars:` are scoped to the included tasks and do not persist after the include completes. Registered variables from included tasks do persist.

Circular includes are detected and reported with a clear error chain. Maximum nesting depth is 64.

See [`examples/include-tasks/`](examples/include-tasks/) for a complete example.

## Block / Rescue / Always

Group tasks with structured error handling -- attempt a block, run rescue on failure, and always run cleanup:

```yaml
tasks:
  - name: Deploy with rollback
    block:
      - name: Pull latest code
        command:
          cmd: git -C /opt/app pull origin main
      - name: Run migrations
        command:
          cmd: /opt/app/migrate.sh
      - name: Restart service
        command:
          cmd: systemctl restart app
    rescue:
      - name: Rollback code
        command:
          cmd: git -C /opt/app reset --hard HEAD~1
      - name: Restart previous version
        command:
          cmd: systemctl restart app
    always:
      - name: Send deploy notification
        command:
          cmd: /opt/app/notify.sh
```

**Execution flow:**
1. `block:` tasks run sequentially. If all succeed, `rescue:` is skipped.
2. If any `block:` task fails, remaining block tasks stop and `rescue:` runs.
3. `always:` runs regardless of block/rescue outcome.
4. If `rescue:` succeeds, the block is considered recovered (no error propagated).

**Block-level directives:**
- `when:` gates the entire block (including rescue and always)
- `sudo:` is inherited by all tasks within block/rescue/always unless overridden
- `name:` provides descriptive output in plan and execution
- Blocks can be nested (block within rescue, etc.)

See [`examples/block-rescue/`](examples/block-rescue/) for a complete example.

## Tags

Selectively run or skip tasks using tags:

```bash
bolt run deploy.yaml --tags deploy          # only deploy-tagged tasks
bolt run deploy.yaml --skip-tags debug      # skip debug tasks
bolt run deploy.yaml --tags deploy,config   # OR logic: deploy or config
bolt run deploy.yaml --check --tags deploy  # plan mode respects tags
```

Tags can be applied to tasks, blocks, plays, and role references:

```yaml
name: Deploy
hosts: webservers
tags: [infra]  # play-level: inherited by all tasks

roles:
  - role: webserver
    tags: [web]  # role-level: inherited by all role tasks

tasks:
  - name: Install nginx
    apt:
      name: nginx
    tags: [packages, web]

  - name: Deploy block
    tags: deploy  # block-level: inherited by child tasks
    block:
      - name: Pull code
        command:
          cmd: git pull
      - name: Restart
        command:
          cmd: systemctl restart app
```

**Special tags:**
- `always` -- task runs even when `--tags` filter is active (unless explicitly in `--skip-tags`)
- `never` -- task is skipped by default, runs only when explicitly included via `--tags`

**Tag inheritance:** A task's effective tags are the union of its own tags plus inherited tags from its play, role, and enclosing block(s). Tags are additive.

**Handlers:** Notified handlers always run regardless of `--tags`, but respect `--skip-tags`.

See [`examples/playbooks/tags-demo.yaml`](examples/playbooks/tags-demo.yaml) for a complete example.

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
| `lineinfile` | Ensure a specific line is present or absent in a file |
| `blockinfile` | Manage a block of text between marker lines in a file |
| `systemd` | Manage systemd services (start, stop, enable, mask, daemon-reload) |
| `template` | Render Go templates with variable substitution |
| `wait_for` | Wait for port, path, command, or URL before proceeding |

Run `bolt module <name>` for detailed parameter docs.

## Tooling

```bash
bolt generate --packages nginx --files /etc/nginx/nginx.conf -c ssh://root@web1  # capture live state
bolt scaffold myrole          # create role boilerplate
bolt test myrole              # test in Docker container (reused for idempotency checks)
bolt validate playbook.yaml   # syntax check
bolt vault init secrets.yaml      # create encrypted vault file
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
| [llms.txt](llms.txt) | LLM-optimized reference (for AI code generation) |

## License

MIT License
