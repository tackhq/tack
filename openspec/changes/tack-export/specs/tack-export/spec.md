## ADDED Requirements

### Requirement: `tack export` subcommand
The tack CLI SHALL provide a `tack export <playbook>` subcommand that compiles a playbook into a standalone bash script per host.

#### Scenario: Subcommand is registered
- **WHEN** `tack --help` is invoked
- **THEN** `export` SHALL appear in the command list with a description

#### Scenario: Missing playbook argument
- **WHEN** `tack export` is invoked without a playbook path
- **THEN** the command SHALL fail with a usage error

### Requirement: Host selection flags
The export command SHALL accept `--host <name>` to target a single host or `--all-hosts` to emit one script per host in the inventory. Exactly one SHALL be specified (the two flags are mutually exclusive). When inventory has a single matching host and neither flag is set, the command SHALL default to that host.

#### Scenario: Single host via flag
- **WHEN** `tack export play.yml --host web01` is run
- **THEN** a single script SHALL be produced targeting web01

#### Scenario: All hosts via flag
- **WHEN** `tack export play.yml --all-hosts` is run against an inventory with 3 hosts
- **THEN** 3 scripts SHALL be produced

#### Scenario: Mutually exclusive
- **WHEN** both `--host` and `--all-hosts` are specified
- **THEN** the command SHALL fail with a validation error

### Requirement: Output flag
The export command SHALL accept `--output <path>`. With `--host`, `--output` is a file path; if unset, output goes to stdout. With `--all-hosts`, `--output` MUST be a directory path and one file per host is written as `<dir>/<hostname>.sh`.

#### Scenario: Stdout in single-host mode
- **WHEN** `tack export play.yml --host web01` is run without `--output`
- **THEN** the script SHALL be written to stdout

#### Scenario: File output in single-host mode
- **WHEN** `tack export play.yml --host web01 --output /tmp/web01.sh` is run
- **THEN** the script SHALL be written to /tmp/web01.sh

#### Scenario: Directory output in all-hosts mode
- **WHEN** `tack export play.yml --all-hosts --output /tmp/scripts/` is run against 2 hosts
- **THEN** `/tmp/scripts/<host>.sh` SHALL be written for each host plus `/tmp/scripts/INDEX.txt`

#### Scenario: All-hosts without --output
- **WHEN** `--all-hosts` is set and `--output` is missing
- **THEN** the command SHALL fail with a validation error

#### Scenario: Hostname sanitization in filenames
- **WHEN** a host is named `web.example.com`
- **THEN** the output filename SHALL be `web.example.com.sh` (dots preserved; other unsafe chars replaced with `_`)

### Requirement: Variable and template resolution at export time
The export command SHALL resolve `{{ var }}` interpolations, template files, and loop expansions at export time using the combined variable context (extra vars, play vars, host vars, frozen facts) with the same precedence as runtime.

#### Scenario: Interpolation resolved
- **WHEN** a task contains `{{ app_version }}` and `app_version: 1.2.3` is set
- **THEN** the emitted shell SHALL contain `1.2.3` literally

#### Scenario: Template file rendered
- **WHEN** a `template:` task references `nginx.conf.j2`
- **THEN** the emitted shell SHALL write the fully rendered content via heredoc

#### Scenario: Extra vars precedence
- **WHEN** play vars set `env: dev` and `-e env=prod` is passed to export
- **THEN** emitted shell SHALL reflect `env=prod`

### Requirement: `when:` evaluated at export time
The export command SHALL evaluate `when:` conditions against the export-time variable context and SHALL exclude tasks whose condition resolves to false. Each excluded task SHALL be represented by a single-line comment `# SKIPPED (when false): <expression>` in the emitted script.

#### Scenario: Task excluded on false condition
- **WHEN** `when: facts.os_type == 'Darwin'` and frozen facts say Linux
- **THEN** the emitted script SHALL contain `# SKIPPED (when false): ...` and NOT contain the task's shell

#### Scenario: Task included on true condition
- **WHEN** `when: app_version == '1.0'` and `app_version: 1.0` is set
- **THEN** the task's shell SHALL be emitted normally

#### Scenario: Runtime registered variable in when
- **WHEN** `when: previous_task.rc == 0` where `previous_task` is a registered variable
- **THEN** the task SHALL be emitted unconditionally with a `# WARN: when references runtime variable` comment

### Requirement: Loop expansion
The export command SHALL unroll `loop:` iterations at export time when the loop source is a static list or a resolvable export-time variable, producing N sequential blocks in the emitted script. `loop:` over dynamic/runtime-only sources SHALL emit as UNSUPPORTED.

#### Scenario: Static list loop unrolled
- **WHEN** `loop: [nginx, postgres, redis]` is present
- **THEN** the emitted script SHALL contain three task blocks with `item` bound to each value

#### Scenario: Variable list loop unrolled
- **WHEN** `loop: "{{ packages }}"` and `packages: [git, curl]` is resolvable
- **THEN** the emitted script SHALL contain two task blocks

#### Scenario: Dynamic loop is unsupported
- **WHEN** `loop:` references a registered-variable result
- **THEN** the task SHALL emit as UNSUPPORTED

### Requirement: Tag filtering
The export command SHALL accept `--tags` and `--skip-tags` flags with runtime-equivalent semantics. Only surviving tasks SHALL appear in the emitted script. Each emitted task block SHALL list its effective tags in the header comment.

#### Scenario: Tag selection
- **WHEN** `--tags web` is set
- **THEN** only tasks with the `web` tag SHALL appear in the script

#### Scenario: Tag skipping
- **WHEN** `--skip-tags deploy` is set
- **THEN** tasks with the `deploy` tag SHALL be excluded

#### Scenario: Tags in block header
- **WHEN** a task has `tags: [web, setup]`
- **THEN** the emitted block header SHALL be `# === TASK: <name> === (tags: setup,web)` (sorted)

### Requirement: Script banner and runtime scaffolding
The emitted script SHALL begin with `#!/usr/bin/env bash`, `set -euo pipefail`, and a banner comment containing the tack version, playbook path, host name, export timestamp (UTC, second-precision), and summary of frozen facts. It SHALL declare counters `TACK_CHANGED=0`, `TACK_FAILED=0`, `TACK_CURRENT_TASK=""`, and install a trap printing a summary on exit.

#### Scenario: Header structure
- **WHEN** any script is emitted
- **THEN** the first two lines SHALL be `#!/usr/bin/env bash` and `set -euo pipefail`

#### Scenario: Banner content
- **WHEN** a script is emitted for host web01 with tack version 1.2.3
- **THEN** the banner SHALL name the tack version, playbook path, host, and export timestamp

#### Scenario: Counters declared
- **WHEN** any script is emitted
- **THEN** the prelude SHALL contain `TACK_CHANGED=0` and `TACK_FAILED=0`

#### Scenario: Trap prints summary
- **WHEN** the emitted script runs to completion
- **THEN** the trap SHALL print `tack-export: summary: changed=<N> failed=<M>` to stderr

#### Scenario: Trap names failing task
- **WHEN** the emitted script exits non-zero mid-run
- **THEN** the trap SHALL print `tack-export: FAILED on task: <name>` to stderr before exiting

### Requirement: Deterministic output
The export command SHALL produce byte-identical output for identical inputs, subject only to the banner timestamp. All map iteration SHALL use sorted keys. Fact ordering SHALL be alphabetical. Loop iterations SHALL preserve input list order. A `--no-banner-timestamp` flag SHALL omit the timestamp line for fully reproducible output.

#### Scenario: Repeated export with same inputs
- **WHEN** `tack export play.yml --host web01 --no-banner-timestamp` is run twice
- **THEN** the two outputs SHALL be byte-identical

#### Scenario: Sorted fact list in banner
- **WHEN** facts include os_type, arch, os_family
- **THEN** they SHALL appear alphabetized in the banner comment

#### Scenario: Sorted keys in emitted maps
- **WHEN** a module emits env-vars from a map `{B: 2, A: 1}`
- **THEN** the emitted shell SHALL order them alphabetically

### Requirement: Per-task block format
Each supported task SHALL emit as a block with this shape: a header comment `# === TASK: <name> === (tags: <sorted-csv>)`, a `TACK_CURRENT_TASK="<name>"` assignment, the module-emitted shell, and a conditional `TACK_CHANGED` bump based on the module's change-detection logic.

#### Scenario: Block header
- **WHEN** a task is emitted
- **THEN** the header SHALL be `# === TASK: <name> === (tags: ...)` on a single line

#### Scenario: Current-task assignment
- **WHEN** a task is emitted
- **THEN** the block SHALL set `TACK_CURRENT_TASK="<name>"` before the shell payload

#### Scenario: Changed counter bump
- **WHEN** a module's emitted shell performs a potentially-changing operation
- **THEN** the block SHALL bump `TACK_CHANGED` when the change is detected at runtime

### Requirement: Module Emitter interface
The system SHALL define an optional `Emitter` interface: `Emit(params map[string]any, pctx *PlayContext) (*EmitResult, error)`. Modules implementing this interface SHALL be supported by export. Modules not implementing it SHALL be treated as unsupported and emitted as comment blocks.

#### Scenario: Module implements Emitter
- **WHEN** a module defines `Emit` and returns a valid `EmitResult`
- **THEN** export SHALL render the resulting shell fragment in the script

#### Scenario: Module lacks Emitter
- **WHEN** a module does not implement `Emit`
- **THEN** export SHALL emit `# UNSUPPORTED: module does not support export`

### Requirement: Unsupported construct handling
The export command SHALL render unsupported constructs as a comment block `# === TASK: <name> ===\n# UNSUPPORTED: <reason>\n# Original task YAML:\n#   <yaml>` instead of silently dropping them. The original task YAML SHALL be embedded as a comment for audit review. Parameter values marked `no_log: true` SHALL be redacted in this embedded YAML.

#### Scenario: Async task is unsupported
- **WHEN** a task uses `async:` 
- **THEN** export SHALL emit an UNSUPPORTED block naming async as the reason

#### Scenario: Handlers unsupported
- **WHEN** a play defines `handlers:`
- **THEN** the handlers SHALL be emitted as a single UNSUPPORTED block with embedded YAML

#### Scenario: block/rescue/always unsupported
- **WHEN** a play contains `block:`, `rescue:`, or `always:` constructs
- **THEN** the entire block SHALL be emitted as a single `# UNSUPPORTED: block/rescue/always not supported in v1` comment with the embedded YAML, and `--check-only` SHALL exit non-zero

#### Scenario: Embedded YAML redaction
- **WHEN** an unsupported task has `password: "secret"` and `no_log: true`
- **THEN** the embedded YAML comment SHALL show `password: "<REDACTED>"`

### Requirement: Fact freezing
The export command SHALL gather facts from the target via the chosen connector once at export start, store them in-memory, and use them to resolve `{{ facts.* }}` references during compilation. Frozen facts SHALL appear in the script banner as a sorted key=value list (excluding unusually large or sensitive values).

#### Scenario: Facts gathered once
- **WHEN** a playbook references `facts.os_type` in 10 tasks
- **THEN** facts SHALL be gathered exactly once at the start of export

#### Scenario: Facts in banner
- **WHEN** export runs
- **THEN** the banner SHALL contain a line `# Facts: arch=x86_64 os_family=Debian os_type=Linux ...`

### Requirement: `--no-facts` flag
The export command SHALL accept `--no-facts` to skip fact gathering. When set, `{{ facts.* }}` references SHALL resolve to the sentinel string `__TACK_FACT_NOT_GATHERED__` and a warning comment SHALL appear in the banner.

#### Scenario: Skip facts
- **WHEN** `--no-facts` is set
- **THEN** no fact-gathering connector call SHALL be made

#### Scenario: Warning in banner
- **WHEN** `--no-facts` is set
- **THEN** the banner SHALL contain `# WARNING: facts not gathered; fact references may be unresolved`

### Requirement: `--check-only` mode
The export command SHALL accept `--check-only`. In this mode, the command SHALL compile the playbook as normal but SHALL NOT write any output files; instead it SHALL print a summary listing supported tasks, unsupported constructs with reasons, and exit non-zero if any unsupported construct was encountered.

#### Scenario: All tasks supported
- **WHEN** `--check-only` and all tasks are supported
- **THEN** the command SHALL print a summary and exit 0

#### Scenario: Unsupported constructs present
- **WHEN** `--check-only` and one task uses async
- **THEN** the command SHALL print the unsupported construct and exit non-zero

#### Scenario: No file output in check-only
- **WHEN** `--check-only --output /tmp/out.sh` is set
- **THEN** `/tmp/out.sh` SHALL NOT be written

### Requirement: Vault value warning
When the emitted script contains values decrypted from a vault, the script banner SHALL include a prominent warning: `# WARNING: This script contains values decrypted from vault.\n# Treat this file as SECRET and do not commit to version control.`

#### Scenario: Vault value resolved
- **WHEN** a task references a vault-encrypted variable and it is decrypted during export
- **THEN** the script banner SHALL include the secret warning

#### Scenario: No vault values
- **WHEN** no vault values are used
- **THEN** the secret warning SHALL NOT appear

### Requirement: `no_log` task output suppression
Tasks marked `no_log: true` SHALL be emitted with their shell wrapped to suppress stdout and stderr at runtime (redirect to `/dev/null`), matching runtime behavior.

#### Scenario: no_log wraps output
- **WHEN** a task has `no_log: true` and emits `my-cmd`
- **THEN** the emitted shell SHALL be `my-cmd >/dev/null 2>&1`

### Requirement: Static include_tasks inlined
Static `include_tasks: <path>` (no loop, no interpolated path) SHALL be recursively compiled and inlined into the emitted script. Dynamic `include_tasks` (with loop or variable-interpolated path) SHALL emit as UNSUPPORTED.

#### Scenario: Static include inlined
- **WHEN** a play has `include_tasks: setup.yml` with a constant path
- **THEN** the tasks from setup.yml SHALL appear in the emitted script in place of the include

#### Scenario: Dynamic include unsupported
- **WHEN** `include_tasks: "{{ role_path }}/main.yml"` uses a variable that references runtime registered state
- **THEN** the include SHALL emit as UNSUPPORTED

#### Scenario: Circular include detected
- **WHEN** include_tasks forms a cycle
- **THEN** export SHALL fail with a clear cycle-detection error

### Requirement: Template content handling
The template module's Emit SHALL render the template at export time and embed the content via heredoc, wrapped in an idempotency guard that writes-if-different. Binary content SHALL be base64-encoded. File mode and owner SHALL be applied via chmod/chown after the write.

#### Scenario: Text template heredoc
- **WHEN** a `template:` task renders to text content
- **THEN** the emitted shell SHALL contain a heredoc block writing the rendered content to a temp file and mv-ing it if content differs

#### Scenario: Binary template base64
- **WHEN** a `copy:` task sources a binary file
- **THEN** the emitted shell SHALL decode base64 content to write the target file

#### Scenario: Mode and owner applied
- **WHEN** `mode: 0600, owner: root` is set
- **THEN** the emitted shell SHALL include `chmod 0600` and `chown root` for the target

### Requirement: Extra-vars support
The export command SHALL accept `-e key=value` and `--extra-vars` flags with the same semantics as `tack run`. These values SHALL participate in variable resolution at export time.

#### Scenario: Extra var applied
- **WHEN** `tack export play.yml --host web01 -e app_version=2.0` is run
- **THEN** `{{ app_version }}` SHALL resolve to `2.0` in the emitted script

### Requirement: Connection flag for fact gathering
The export command SHALL accept `--connection <type>` (local/ssh/ssm/docker) to control fact-gathering. Default SHALL be the playbook's connection setting or local.

#### Scenario: SSH connection for fact gathering
- **WHEN** `--connection ssh` is set
- **THEN** facts SHALL be gathered via SSH against the target

### Requirement: Exit codes
The export command SHALL exit 0 on success. It SHALL exit non-zero when: validation fails, the playbook or inventory cannot be parsed, fact gathering fails, `--check-only` finds unsupported constructs, or a circular include is detected.

#### Scenario: Success
- **WHEN** export completes successfully
- **THEN** exit code SHALL be 0

#### Scenario: Parse error
- **WHEN** the playbook YAML is invalid
- **THEN** exit code SHALL be non-zero

#### Scenario: Check-only with unsupported
- **WHEN** `--check-only` and unsupported constructs are found
- **THEN** exit code SHALL be non-zero
