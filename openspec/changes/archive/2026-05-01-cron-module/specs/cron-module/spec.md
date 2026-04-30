## ADDED Requirements

### Requirement: Module registration
The system SHALL register a `cron` module in the module registry, invokable via `cron:` in playbook tasks.

#### Scenario: cron appears in module list
- **WHEN** the module registry is listed
- **THEN** `cron` SHALL be present

#### Scenario: Task dispatch
- **WHEN** a task specifies `cron: { name: "backup", job: "/usr/local/bin/backup.sh" }`
- **THEN** the executor SHALL dispatch it to the cron module

### Requirement: Required and mutually-exclusive parameters
The cron module SHALL enforce parameter rules: `name` is always required; `job` is required when `state=present` and `env=false`; `special_time` and time fields (`minute`/`hour`/`day`/`month`/`weekday`) are mutually exclusive. The `user` parameter has two compatible meanings based on context: when `cron_file` is unset it names whose crontab to edit via `crontab -u`; when `cron_file` is set it names the execution user written into the drop-in line.

#### Scenario: Missing name
- **WHEN** task omits `name`
- **THEN** the task SHALL fail with a validation error

#### Scenario: Missing job with present state
- **WHEN** `state: present`, `env: false`, and `job` is not set
- **THEN** the task SHALL fail with a validation error

#### Scenario: Both special_time and time fields
- **WHEN** `special_time: daily` and `hour: 3` are both set
- **THEN** the task SHALL fail with a validation error

#### Scenario: user with cron_file names the execution user
- **WHEN** `user: alice` and `cron_file: /etc/cron.d/backup` are both set
- **THEN** validation SHALL pass and the written drop-in line SHALL use `alice` as the user field

### Requirement: Name validation
The cron module SHALL validate that `name` is non-empty, contains no newlines, no `#` characters, no non-printable characters, and is at most 200 characters.

#### Scenario: Name with newline
- **WHEN** `name: "foo\nbar"`
- **THEN** the task SHALL fail with a validation error

#### Scenario: Name with `#`
- **WHEN** `name: "backup #1"`
- **THEN** the task SHALL fail with a validation error

#### Scenario: Name over length limit
- **WHEN** `name` is 201 characters
- **THEN** the task SHALL fail with a validation error

### Requirement: Linux-only support
The cron module SHALL return an error when executed against a target whose `facts.os_type` is not `Linux`. The error message SHALL name the detected OS and suggest alternatives.

#### Scenario: macOS target
- **WHEN** the target reports `os_type: Darwin`
- **THEN** the task SHALL fail with an error mentioning `Linux` and suggesting launchd

#### Scenario: Linux target
- **WHEN** the target reports `os_type: Linux`
- **THEN** the module SHALL proceed normally

### Requirement: Managed-comment marker
The cron module SHALL place a marker comment `# TACK: <name>` on the line immediately preceding each managed entry. Managed entries SHALL be located on subsequent runs by scanning for this marker.

#### Scenario: First-time creation writes marker
- **WHEN** a new cron entry is added with `name: backup`
- **THEN** the crontab SHALL contain `# TACK: backup` followed by the schedule line

#### Scenario: Marker is used for identification
- **WHEN** the module runs and finds `# TACK: backup` in the crontab
- **THEN** the module SHALL treat the next non-empty line as its managed entry

### Requirement: User crontab management
When `cron_file` is not set, the cron module SHALL read the target user's crontab via `crontab -l [-u <user>]`, edit in memory, and write back via `crontab - [-u <user>]`. An "empty crontab" stderr (e.g., `no crontab for <user>`) SHALL be treated as an empty crontab, not an error. When the resulting crontab is non-empty, the module SHALL create or replace the user crontab.

#### Scenario: Empty crontab is handled
- **WHEN** `crontab -l` returns non-zero with "no crontab for" stderr
- **THEN** the module SHALL treat it as empty and proceed

#### Scenario: Add entry to empty crontab
- **WHEN** the user has no crontab and a `state: present` task runs
- **THEN** after execution the user crontab SHALL contain the marker + schedule line

#### Scenario: Target a different user
- **WHEN** `user: alice` is set
- **THEN** the module SHALL invoke `crontab -u alice -l` and `crontab -u alice -`

### Requirement: /etc/cron.d drop-in management
When `cron_file` is set, the cron module SHALL read the drop-in file (treating a missing file as empty), edit in memory, and write the file back with mode 0644. When the last managed entry is removed and the file becomes empty (ignoring whitespace), the module SHALL delete the file.

#### Scenario: Create drop-in file
- **WHEN** `cron_file: /etc/cron.d/backup` is set, file does not exist, and task is `state: present`
- **THEN** the module SHALL create the file with marker + line

#### Scenario: User field included in drop-in line
- **WHEN** writing to `cron_file` with `user: alice`
- **THEN** the written line SHALL be `<schedule> alice <command>`

#### Scenario: Default user is root for drop-ins
- **WHEN** `cron_file` is set and `user` is omitted
- **THEN** the written line SHALL use `root` as the user field

#### Scenario: Delete drop-in when empty
- **WHEN** removing the only entry in `cron_file` makes the file empty
- **THEN** the module SHALL delete the file

### Requirement: Cron file name validation
When `cron_file` is set, the basename SHALL match the regex `^[A-Za-z0-9_-]+$` (no dots, no extensions, no other characters).

#### Scenario: Valid drop-in name
- **WHEN** `cron_file: /etc/cron.d/backup-job`
- **THEN** validation SHALL pass

#### Scenario: Drop-in name with dot
- **WHEN** `cron_file: /etc/cron.d/backup.sh`
- **THEN** the task SHALL fail with a validation error

### Requirement: Idempotent create/update
The cron module SHALL add a managed entry when absent, update it when schedule/job/user differ from desired state, and report `changed: false` when the entry already matches.

#### Scenario: First run creates entry
- **WHEN** no marker exists and `state: present`
- **THEN** the module SHALL add the marker + line and report `changed: true`

#### Scenario: Second run with same params
- **WHEN** marker + matching line already exist
- **THEN** the module SHALL report `changed: false`

#### Scenario: Schedule change triggers update
- **WHEN** marker exists but `hour` differs from desired
- **THEN** the module SHALL rewrite the line and report `changed: true`

#### Scenario: Command change triggers update
- **WHEN** marker exists but `job` differs from desired
- **THEN** the module SHALL rewrite the line and report `changed: true`

### Requirement: State absent removes entry
When `state: absent`, the cron module SHALL remove both the marker line and the managed entry line. When no marker is found, the module SHALL report `changed: false`.

#### Scenario: Remove existing entry
- **WHEN** marker exists and `state: absent`
- **THEN** both the marker and the entry line SHALL be removed and `changed: true` reported

#### Scenario: Remove when entry does not exist
- **WHEN** no marker found and `state: absent`
- **THEN** the module SHALL report `changed: false` and not modify the crontab

### Requirement: Disabled entries
When `disabled: true` and `state: present`, the cron module SHALL ensure the managed entry line is prefixed with `#` (commented out) while the marker line is preserved. When `disabled: false` (or unset) and the managed line is currently commented, the module SHALL uncomment it.

#### Scenario: Disable an existing entry
- **WHEN** marker + active line exist and `disabled: true` is set
- **THEN** the line SHALL be prepended with `# ` and reported as changed

#### Scenario: Re-enable a disabled entry
- **WHEN** marker + commented line exist and `disabled: false`
- **THEN** the leading `# ` SHALL be removed and reported as changed

#### Scenario: Idempotent disable
- **WHEN** marker + commented line exist and `disabled: true`
- **THEN** the module SHALL report `changed: false`

### Requirement: Special time shortcuts
The cron module SHALL accept `special_time` values `reboot`, `yearly`, `annually`, `monthly`, `weekly`, `daily`, `hourly` and SHALL write them as `@reboot`, `@yearly`, etc. as the schedule portion of the line.

#### Scenario: Daily shortcut
- **WHEN** `special_time: daily`
- **THEN** the written line SHALL begin with `@daily `

#### Scenario: Reboot shortcut
- **WHEN** `special_time: reboot`
- **THEN** the written line SHALL begin with `@reboot `

#### Scenario: Invalid special_time
- **WHEN** `special_time: weekly-ish`
- **THEN** the task SHALL fail with a validation error

### Requirement: Environment line mode
When `env: true`, the cron module SHALL treat `job` as a `KEY=VALUE` environment line and SHALL reject configured time fields. The written managed block SHALL be `# TACK: <name>\n<KEY=VALUE>`.

#### Scenario: Set PATH env line
- **WHEN** `env: true`, `name: path`, `job: "PATH=/usr/local/bin:/usr/bin"`
- **THEN** the crontab SHALL contain the marker + the env line

#### Scenario: env with schedule fields
- **WHEN** `env: true` and `hour: 3` is also set
- **THEN** the task SHALL fail with a validation error

#### Scenario: env with malformed job
- **WHEN** `env: true`, `job: "no-equals-sign"`
- **THEN** the task SHALL fail with a validation error

### Requirement: Check mode (--dry-run)
Under `--dry-run`, the cron module SHALL evaluate the desired state against the current crontab and SHALL NOT modify anything on the target. It SHALL report whether a change would occur.

#### Scenario: Dry-run would create
- **WHEN** marker absent and `--dry-run` with `state: present`
- **THEN** no `crontab -` invocation SHALL run and the task SHALL report `would_change: true`

#### Scenario: Dry-run no change
- **WHEN** marker + matching line exist under `--dry-run`
- **THEN** the task SHALL report `would_change: false`

### Requirement: Diff mode
Under `--diff`, the cron module SHALL emit a unified diff between the current and prospective crontab (user crontab) or drop-in file contents.

#### Scenario: Diff on create
- **WHEN** marker absent, `--diff`, and `state: present`
- **THEN** the output SHALL include a unified diff adding the marker + line

#### Scenario: Diff on unchanged
- **WHEN** entry already matches desired under `--diff`
- **THEN** no diff output SHALL be produced for the task

### Requirement: Works through all connectors
The cron module SHALL function identically through local, SSH, SSM, and Docker connectors, using only `Execute`, `Upload`, and `Download` primitives.

#### Scenario: SSH connector
- **WHEN** module runs via SSH connector against a Linux host
- **THEN** it SHALL invoke `crontab` commands via the connector's Execute method

#### Scenario: Docker connector
- **WHEN** module runs via Docker connector against a Linux container with cron installed
- **THEN** it SHALL manage the crontab inside the container

### Requirement: Sudo integration
The cron module SHALL use the connector's sudo configuration when set. Commands invoking `crontab -u <other-user>` or writing to `/etc/cron.d/` SHALL run through sudo when enabled.

#### Scenario: crontab for another user with sudo
- **WHEN** sudo is enabled and `user: alice` is set while connected as a non-root user
- **THEN** `crontab -u alice ...` SHALL be invoked via sudo

#### Scenario: drop-in write requires sudo
- **WHEN** sudo is enabled and `cron_file: /etc/cron.d/foo` is set
- **THEN** the Upload (or tmp+mv) SHALL run via sudo
