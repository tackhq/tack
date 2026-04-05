# Modules Reference

Modules are the units of work in Tack. Each module performs a specific action like installing packages, managing files, or running commands.

## Available Modules

| Module | Description |
|--------|-------------|
| [apt](#apt) | Manage packages on Debian/Ubuntu |
| [brew](#brew) | Manage Homebrew packages on macOS |
| [yum](#yum) | Manage packages on RHEL/CentOS/Fedora |
| [command](#command) | Execute shell commands |
| [copy](#copy) | Copy files to targets |
| [file](#file) | Manage files and directories |
| [systemd](#systemd) | Manage systemd services |
| [template](#template) | Render templates to targets |
| [user](#user) | Manage system users on Linux |
| [group](#group) | Manage system groups on Linux |
| [wait_for](#wait_for) | Wait for a condition before proceeding |
| [assert](#assert) | Fail fast on precondition expressions (built-in keyword) |

---

## apt

Manage packages on Debian/Ubuntu systems using apt-get.

### Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `name` | string/list | no* | - | Package name(s) |
| `state` | string | no | `present` | `present`, `absent`, `latest`, `purged` |
| `update_cache` | bool | no | `false` | Run `apt-get update` first |
| `cache_valid_time` | int | no | `0` | Skip update if cache newer than N seconds |
| `upgrade` | string | no | `none` | `none`, `yes`, `safe`, `full`, `dist` |
| `install_recommends` | bool | no | `true` | Install recommended packages |
| `autoremove` | bool | no | `false` | Remove unused dependencies |
| `deb` | string | no | - | Path or URL to .deb file |

*Required unless using `update_cache`, `upgrade`, or `deb`

### States

| State | Description |
|-------|-------------|
| `present` | Ensure package is installed |
| `absent` | Remove package (keep config files) |
| `latest` | Install and upgrade to latest version |
| `purged` | Remove package and config files |

### Examples

```yaml
# Install a single package
- name: Install nginx
  apt:
    name: nginx
    state: present

# Install multiple packages
- name: Install development tools
  apt:
    name:
      - build-essential
      - git
      - curl
    state: present

# Update cache and install
- name: Install with fresh cache
  apt:
    name: nginx
    state: present
    update_cache: true
    cache_valid_time: 3600

# Upgrade all packages
- name: Full system upgrade
  apt:
    upgrade: dist
    autoremove: true

# Install from .deb file
- name: Install local package
  apt:
    deb: /tmp/package.deb

# Remove package completely
- name: Purge old package
  apt:
    name: apache2
    state: purged
```

---

## brew

Manage Homebrew packages on macOS.

### Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `name` | string/list | no* | - | Package name(s) |
| `state` | string | no | `present` | `present`, `absent`, `latest` |
| `cask` | bool | no | `false` | Install as cask (GUI application) |
| `update_homebrew` | bool | no | `false` | Run `brew update` first |
| `upgrade_all` | bool | no | `false` | Upgrade all packages |
| `options` | list | no | - | Additional install options |

*Required unless using `update_homebrew` or `upgrade_all`

### Examples

```yaml
# Install CLI tool
- name: Install ripgrep
  brew:
    name: ripgrep
    state: present

# Install multiple tools
- name: Install development tools
  brew:
    name:
      - git
      - go
      - node
    state: present

# Install GUI application (cask)
- name: Install VS Code
  brew:
    name: visual-studio-code
    cask: true

# Install multiple casks
- name: Install GUI apps
  brew:
    name:
      - docker
      - slack
      - 1password
    cask: true
    state: present

# Update Homebrew and upgrade all
- name: Update everything
  brew:
    update_homebrew: true
    upgrade_all: true

# Keep package at latest version
- name: Ensure go is latest
  brew:
    name: go
    state: latest
```

---

## yum

Manage packages on RPM-based systems (RHEL, CentOS, Fedora, Amazon Linux, Rocky Linux) using yum or dnf. The module auto-detects whether `dnf` or `yum` is available, preferring `dnf`.

### Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `name` | string/list | no* | - | Package name(s) |
| `state` | string | no | `present` | `present`, `absent`, `latest` |
| `update_cache` | bool | no | `false` | Run `yum makecache` first |
| `upgrade` | string | no | `none` | `none`, `yes` |
| `autoremove` | bool | no | `false` | Remove unused dependencies |

*Required unless using `update_cache` or `upgrade`

### States

| State | Description |
|-------|-------------|
| `present` | Ensure package is installed |
| `absent` | Remove package |
| `latest` | Install and upgrade to latest version |

### Examples

```yaml
# Install a single package
- name: Install nginx
  yum:
    name: nginx
    state: present

# Install multiple packages
- name: Install development tools
  yum:
    name:
      - gcc
      - make
      - git
    state: present

# Update cache and install
- name: Install with fresh cache
  yum:
    name: nginx
    state: present
    update_cache: true

# Upgrade all packages
- name: Full system upgrade
  yum:
    upgrade: yes
    autoremove: true

# Keep package at latest version
- name: Ensure nginx is latest
  yum:
    name: nginx
    state: latest

# Remove a package
- name: Remove old package
  yum:
    name: httpd
    state: absent
    autoremove: true
```

---

## command

Execute shell commands on the target.

### Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `cmd` | string | **yes** | - | Command to execute |
| `chdir` | string | no | - | Change to directory before running |
| `creates` | string | no | - | Skip if this path exists |
| `removes` | string | no | - | Only run if this path exists |

### Idempotency

Use `creates` and `removes` for idempotent commands:

- `creates` - Skip command if the path already exists
- `removes` - Only run if the path exists

### Examples

```yaml
# Simple command
- name: Show current user
  command:
    cmd: whoami

# Change directory first
- name: Build project
  command:
    cmd: make build
    chdir: /opt/myapp

# Idempotent with creates
- name: Initialize database
  command:
    cmd: ./init-db.sh
    creates: /var/lib/myapp/db.sqlite

# Only run if file exists
- name: Remove old logs
  command:
    cmd: rm -rf /var/log/myapp/*.old
    removes: /var/log/myapp

# Capture output
- name: Get version
  command:
    cmd: cat /etc/os-release
  register: os_info
```

### Result Data

When using `register`, the result contains:

```yaml
result:
  changed: true
  data:
    cmd: "whoami"
    stdout: "alice"
    stderr: ""
    exit_code: 0
```

---

## copy

Copy files or content to the target.

### Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `dest` | string | **yes** | - | Destination path |
| `src` | string | no* | - | Source file path |
| `content` | string | no* | - | Inline content to write |
| `mode` | string | no | `0644` | File permissions |
| `owner` | string | no | - | Owner username |
| `group` | string | no | - | Group name |
| `backup` | bool | no | `false` | Create backup before overwriting |
| `force` | bool | no | `true` | Overwrite if exists |
| `create_dirs` | bool | no | `false` | Create parent directories |
| `validate` | string | no | - | Validation command (`%s` = temp path) |

*Either `src` or `content` is required (mutually exclusive)

### Examples

```yaml
# Copy a file
- name: Copy nginx config
  copy:
    src: ./files/nginx.conf
    dest: /etc/nginx/nginx.conf
    mode: "0644"
    owner: root
    group: root
    backup: true

# Write inline content
- name: Create config file
  copy:
    dest: /etc/myapp/config.yaml
    content: |
      database:
        host: localhost
        port: 5432
      logging:
        level: info
    mode: "0600"

# With validation
- name: Copy SSH config
  copy:
    src: ./sshd_config
    dest: /etc/ssh/sshd_config
    validate: "/usr/sbin/sshd -t -f %s"
    backup: true

# Create with parent directories
- name: Create nested config
  copy:
    dest: /opt/myapp/config/settings.yaml
    content: "# Settings"
    create_dirs: true
```

### Using with Roles

When using the copy module within a role, relative `src` paths automatically resolve to the role's `files/` directory:

```yaml
# In roles/webserver/tasks/main.yaml
- name: Copy nginx config
  copy:
    src: nginx.conf              # Looks in roles/webserver/files/nginx.conf
    dest: /etc/nginx/nginx.conf
```

This allows you to organize static files within your role:

```
roles/webserver/
├── tasks/
│   └── main.yaml
└── files/
    ├── nginx.conf
    └── index.html
```

### Idempotency

The copy module uses SHA256 checksums to detect changes. It will:
- Skip if content already matches
- Only update attributes if content is same but mode/owner differs

---

## file

Manage files, directories, and symlinks.

### Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `path` | string | **yes** | - | Path to manage |
| `state` | string | no | `file` | `file`, `directory`, `link`, `absent`, `touch` |
| `mode` | string | no | - | Permissions (e.g., `0755`) |
| `owner` | string | no | - | Owner username |
| `group` | string | no | - | Group name |
| `src` | string | no | - | Source for symlinks |
| `recurse` | bool | no | `false` | Apply attributes recursively |
| `force` | bool | no | `false` | Force symlink creation |

### States

| State | Description |
|-------|-------------|
| `file` | Ensure file exists (error if missing) |
| `directory` | Create directory if missing |
| `link` | Create symlink (requires `src`) |
| `absent` | Remove file or directory |
| `touch` | Create empty file or update timestamp |

### Examples

```yaml
# Create directory
- name: Create app directory
  file:
    path: /opt/myapp
    state: directory
    mode: "0755"
    owner: appuser
    group: appgroup

# Create nested directories
- name: Create config structure
  file:
    path: /opt/myapp/config/ssl
    state: directory
    mode: "0700"

# Create symlink
- name: Link current version
  file:
    path: /opt/myapp/current
    src: /opt/myapp/releases/v1.2.3
    state: link

# Force symlink (replace existing)
- name: Update symlink
  file:
    path: /opt/myapp/current
    src: /opt/myapp/releases/v1.2.4
    state: link
    force: true

# Set permissions recursively
- name: Fix permissions
  file:
    path: /var/www/html
    mode: "0755"
    owner: www-data
    group: www-data
    recurse: true
    state: directory

# Remove file or directory
- name: Clean up temp files
  file:
    path: /tmp/myapp-cache
    state: absent

# Touch file (create or update timestamp)
- name: Update marker file
  file:
    path: /var/run/myapp.updated
    state: touch
```

---

## systemd

Manage systemd services on Linux systems.

### Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `name` | string | **yes** | - | Service unit name (e.g., `nginx`, `docker.service`) |
| `state` | string | no | - | `started`, `stopped`, `restarted`, `reloaded` |
| `enabled` | bool | no | - | Enable/disable service at boot |
| `daemon_reload` | bool | no | `false` | Run `systemctl daemon-reload` first |
| `masked` | bool | no | - | Mask/unmask the service |

### Examples

```yaml
# Start and enable a service
- name: Enable nginx
  systemd:
    name: nginx
    state: started
    enabled: true

# Restart after config change
- name: Restart app
  systemd:
    name: myapp
    state: restarted

# Reload systemd after unit file changes
- name: Reload and start
  systemd:
    name: myapp
    daemon_reload: true
    state: started

# Mask a service
- name: Mask unused service
  systemd:
    name: cups
    masked: true
```

### Idempotency

The module checks current service state before acting. If the service is already in the desired state, no changes are made.

---

## template

Render templates to the target with variable substitution using Go's text/template syntax.

### Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `src` | string | **yes** | - | Template file path (relative to role's templates/) |
| `dest` | string | **yes** | - | Destination path on target |
| `mode` | string | no | `0644` | File permissions |
| `owner` | string | no | - | Owner username |
| `group` | string | no | - | Group name |
| `backup` | bool | no | `false` | Create backup before overwriting |

### Template Syntax

Templates use Go's `text/template` syntax with `{{ .variable }}` notation:

```
# Config file for {{ .app_name }}
server:
  host: {{ .server_host }}
  port: {{ .server_port }}
```

All playbook variables (play vars, role vars, facts, registered variables) are available in templates.

### Built-in Functions

| Function | Description | Example |
|----------|-------------|---------|
| `default` | Fallback value if empty | `{{ default "localhost" .host }}` |
| `lower` | Lowercase string | `{{ lower .env }}` |
| `upper` | Uppercase string | `{{ upper .env }}` |
| `trim` | Trim whitespace | `{{ trim .value }}` |

### Examples

```yaml
# Render a config template
- name: Deploy nginx config
  template:
    src: nginx.conf.j2
    dest: /etc/nginx/nginx.conf
    mode: "0644"
    owner: root
    group: root
    backup: true

# Simple application config
- name: Create app config
  template:
    src: app.yaml.j2
    dest: /opt/myapp/config.yaml
    mode: "0600"
```

### Using with Roles

When using the template module within a role, relative `src` paths automatically resolve to the role's `templates/` directory:

```yaml
# In roles/webserver/tasks/main.yaml
- name: Deploy nginx config
  template:
    src: nginx.conf.j2     # Looks in roles/webserver/templates/nginx.conf.j2
    dest: /etc/nginx/nginx.conf
```

Role directory structure:

```
roles/webserver/
├── tasks/
│   └── main.yaml
├── templates/
│   ├── nginx.conf.j2
│   └── app.conf.j2
└── files/
    └── static-file.txt
```

### Idempotency

The template module uses SHA256 checksums to detect changes. It will:
- Render the template and compare checksum with destination
- Skip if rendered content matches existing file
- Only update attributes if content is same but mode/owner differs

---

## user

Manage system users on Linux using useradd/usermod/userdel.

### Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `name` | string | **yes** | - | Username |
| `state` | string | no | `present` | `present`, `absent` |
| `uid` | int | no | - | User ID |
| `shell` | string | no | - | Login shell (e.g., `/bin/bash`) |
| `home` | string | no | - | Home directory path |
| `groups` | list | no | - | Supplementary groups (appended to existing) |
| `system` | bool | no | `false` | Create a system user |
| `password` | string | no | - | Pre-hashed password (e.g., SHA-512 crypt format) |
| `remove` | bool | no | `false` | Remove home directory when `state: absent` |

### Examples

```yaml
# Create a user with defaults
- name: Create deploy user
  user:
    name: deploy

# Create user with full options
- name: Create app user
  user:
    name: app
    shell: /bin/bash
    home: /opt/app
    uid: 1500
    groups:
      - docker
      - wheel
    system: true

# Set user password (pre-hashed)
- name: Set deploy password
  user:
    name: deploy
    password: "$6$rounds=100000$salt$hash..."

# Remove user and home directory
- name: Remove old user
  user:
    name: olduser
    state: absent
    remove: true
```

### Idempotency

The module queries current state via `getent passwd` and `id -Gn` before acting. It will:
- Skip if user already exists with matching attributes
- Only modify attributes that differ from current state
- Supplementary groups are always appended (`usermod -aG`), never replaced

### Notes

- Linux only — macOS (`dscl`) is not supported
- Passwords must be provided pre-hashed; the module does not hash plaintext
- The password hash is briefly visible in `ps` output (same as Ansible)
- `groups` are appended to existing supplementary groups, not replaced

---

## group

Manage system groups on Linux using groupadd/groupmod/groupdel.

### Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `name` | string | **yes** | - | Group name |
| `state` | string | no | `present` | `present`, `absent` |
| `gid` | int | no | - | Group ID |
| `system` | bool | no | `false` | Create a system group |

### Examples

```yaml
# Create a group
- name: Create deploy group
  group:
    name: deploy

# Create group with specific GID
- name: Create app group
  group:
    name: app
    gid: 1500

# Create system group
- name: Create service group
  group:
    name: myservice
    system: true

# Remove a group
- name: Remove old group
  group:
    name: oldgroup
    state: absent
```

### Idempotency

The module queries current state via `getent group` before acting. It will:
- Skip if group already exists with matching attributes
- Only modify GID if it differs from current state

---

## wait_for

Wait for a condition to be met before continuing playbook execution. Supports waiting for TCP ports, filesystem paths, shell commands, and HTTP URLs.

### Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `type` | string | **yes** | - | Condition type: `port`, `path`, `command`, `url` |
| `host` | string | no | `localhost` | Host to check (port type only) |
| `port` | int | no | - | TCP port number (required for port type) |
| `path` | string | no | - | Filesystem path (required for path type) |
| `cmd` | string | no | - | Shell command (required for command type) |
| `url` | string | no | - | HTTP(S) URL (required for url type) |
| `timeout` | int | no | `300` | Maximum wait time in seconds |
| `interval` | int | no | `5` | Poll interval in seconds |
| `state` | string | no | `started` | Desired state: `started` or `stopped` (port and path types) |

### Condition Types

| Type | Checks From | Success Condition |
|------|-------------|-------------------|
| `port` | Controller | TCP connection succeeds (started) or is refused (stopped) |
| `path` | Target | File/directory exists (started) or is absent (stopped) |
| `command` | Target | Command returns exit code 0 |
| `url` | Controller | HTTP response with status 200-399 |

### Examples

```yaml
# Wait for a service port to open
- name: Wait for PostgreSQL
  wait_for:
    type: port
    port: 5432
    timeout: 60

# Wait for a remote port
- name: Wait for web server
  wait_for:
    type: port
    host: 10.0.1.5
    port: 443
    timeout: 30
    interval: 2

# Wait for a port to close
- name: Wait for old service to stop
  wait_for:
    type: port
    port: 8080
    state: stopped
    timeout: 30

# Wait for a file to appear
- name: Wait for PID file
  wait_for:
    type: path
    path: /var/run/myapp.pid
    timeout: 60

# Wait for a lock file to be removed
- name: Wait for deploy lock
  wait_for:
    type: path
    path: /var/lock/deploy.lock
    state: stopped
    timeout: 120

# Wait for a command to succeed
- name: Wait for database ready
  wait_for:
    type: command
    cmd: pg_isready -h localhost
    timeout: 60
    interval: 5

# Wait for HTTP endpoint
- name: Wait for health check
  wait_for:
    type: url
    url: http://localhost:8080/health
    timeout: 120
    interval: 5
```

### Result Data

When using `register`, the result contains:

```yaml
result:
  changed: true
  data:
    elapsed: 12.5      # seconds waited
    attempts: 3         # number of polls
    # For command type:
    stdout: "ready"
    stderr: ""
    # For url type:
    status_code: 200
```

### Notes

- Port and URL checks run from the **controller** (the machine running Tack), not from the target. To check from the target's perspective, use `type: command` with tools like `nc` or `curl`.
- Path and command checks execute on the **target** via the connector.
- The `state` parameter only applies to `port` and `path` types. Use `started` (default) to wait for the condition to be true, or `stopped` to wait for it to become false.
- On timeout, the module returns an error with a descriptive message.

---

## assert

Validate preconditions at the top of a play and fail fast with a clear message. `assert` is a **built-in task keyword** (like `block:` and `include_tasks:`), not a registered module — it runs locally on the control host and never invokes the connector, so it works identically for every connection type.

### Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `that` | string or list of strings | **yes** | - | One or more boolean expressions using the same syntax as `when:` |
| `fail_msg` | string | no | - | Custom message emitted when any condition is false |
| `success_msg` | string | no | - | Message emitted when all conditions pass |
| `quiet` | bool | no | `false` | Suppress per-condition output on success |

The `that:` expressions use the exact same engine as `when:`, so every supported operator is available: `==`, `!=`, `<`, `>`, `<=`, `>=`, `in`, `not in`, `is defined`, `is not defined`, `and`, `or`, `not`, and parenthesized grouping.

### Examples

```yaml
# Single condition as string
- name: OS must be Linux
  assert:
    that: "facts.os_type == 'Linux'"
    fail_msg: "this playbook only supports Linux"

# Multiple conditions
- name: Preflight checks
  assert:
    that:
      - "facts.os_family in ['Debian', 'RedHat']"
      - "deploy_env is defined"
      - "deploy_env in ['staging', 'prod']"
    success_msg: "all preconditions satisfied"

# Register the result and branch on it
- register: chk
  assert:
    that:
      - "version is defined"

- name: Continue only if chk passed
  when: "chk.failed == false"
  command:
    cmd: echo "proceeding"

# Assert inside a block triggers rescue on failure
- block:
    - assert:
        that:
          - "facts.arch == 'x86_64'"
  rescue:
    - command:
        cmd: echo "unsupported architecture — falling back"
```

### Behavior

- A false condition fails the task. Block/rescue/always semantics apply normally.
- `when:`, `tags:`, `register:`, and `--dry-run` all work on assert tasks.
- Under `--dry-run`, asserts are still evaluated and failing asserts still fail the play (preconditions should fail fast regardless of mode).
- `--diff` is a no-op for assert tasks.
- Assert never reports `changed: true` — it only validates, it never mutates state.
- The registered result contains `changed`, `failed`, `msg`, and `evaluated_conditions` (an array of `{expr, result}` entries).

---

## Writing Custom Modules

Modules implement the `Module` interface:

```go
type Module interface {
    Name() string
    Run(ctx context.Context, conn connector.Connector, params map[string]any) (*Result, error)
}

type Result struct {
    Changed bool
    Message string
    Data    map[string]any
}
```

Register modules in `init()`:

```go
func init() {
    module.Register(&MyModule{})
}
```

See existing modules in `internal/module/` for examples.
