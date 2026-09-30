## Why

Tack conflates *whether to escalate* (sudo on/off per playbook, play or task) with *the escalation password* (none needed, env, flag, vault, prompt, different per host). The coupling produced repeated bugs and flip-flops (eac3622, e733222, the per-play prompt fixed in v1.1.2): `-s` both escalates and forces a prompt, an empty password is indistinguishable from "not provided", the password lives on `playbook.Play` as a plain string, one password is sprayed to every host whether or not its sudo asks for it, and non-interactive runs discover a missing password only when a task fails mid-apply.

## What Changes

- Escalation is purely declarative: `sudo:` and `become:` become exact, permanent aliases at playbook, play and task level (conflicting values are a parse error). `-b/--become` is added as an alias of `-s/--sudo`. Neither flag triggers a prompt any more.
- A run-scoped **become credential resolver** on the executor replaces `play.SudoPassword`. It is tri-state (unset / provided, including empty / not needed), cached per (credential key, become method, become user), held as `[]byte` and zeroed at end of run.
- Password sources, most specific wins: inventory host/group var `tack_become_password` (also `ansible_become_password`) > `--become-password-file` / `TACK_BECOME_PASSWORD_FILE` > `TACK_BECOME_PASSWORD` (`TACK_SUDO_PASSWORD` alias) > play `sudo_password:` (now interpolated so it can reference vault vars) > interactive prompt.
- **Send on demand**: each host is probed once per run with `sudo -n true` / `doas -n true`; hosts that escalate without a password never receive one.
- **Prompt timing**: at most one prompt per run, after the plan is shown and before apply, and only if a task that will actually run escalates on a host that needs a password. `-K/--ask-become-pass` prompts upfront. Never prompts mid-apply.
- Non-interactive runs with no password source fail fast, before apply, only for hosts whose probe fails; passwordless-sudo CI needs no opt-out flag.
- Wrong password: non-interactive sources are never retried and are invalidated run-wide; a prompted password is verified on one host first and may be re-entered up to twice, before apply.
- New flags/env: `-b/--become`, `-K/--ask-become-pass` (alias `--ask-sudo-pass`), `--become-password-file`, `--no-prompt`, `TACK_BECOME_PASSWORD`, `TACK_BECOME_PASSWORD_FILE`. Existing `--no-sudo-prompt`, `TACK_SUDO_NO_PROMPT`, `TACK_SUDO_PASSWORD` stay as permanent aliases.
- Deprecations (warn in 1.2, no breakage in 1.x): `--sudo-password=VALUE` (argv exposure; removed in 2.0), literal plaintext `sudo_password:` in YAML. Bare `--sudo-password` becomes an alias of `-K`. `--sudo-password VALUE playbook.yml` (space-separated, currently misparsed as the playbook path) is detected and rejected with a clear error.
- The become password never appears in argv, logs, plan/diff, registered output, `tack export`, JSON output, or error messages; `-vv` reports only which source was used per host.

## Capabilities

### New Capabilities
- `become-escalation`: declarative escalation — `sudo`/`become` aliases, `become_user`/`become_method`, precedence task > play > playbook > CLI, `-s`/`-b` semantics.
- `become-credentials`: escalation password resolution — sources and precedence, tri-state credential, per-host probe and send-on-demand, prompt timing, non-interactive behavior, wrong-password handling, deprecations, redaction guarantees.

### Modified Capabilities
- `playbook-defaults`: playbook-level defaults accept `become` as an alias of `sudo`.

## Impact

- `cmd/tack/main.go`: flag definitions for `run` (and help text), sudo prompt wiring (`configureSudoPrompt`, `PromptSudoPassword`), override resolution for `--sudo-password` / env.
- `internal/executor/executor.go`: new credential resolver replaces `needsSudoPassword` and `play.SudoPassword` usage in `GetConnector` and per-task `SetSudo`; probe and verification hooks in the plan → apply flow; filtered-plan escalation detection.
- `internal/playbook/{playbook.go,parser.go}`: `become` alias, conflict detection, `sudo_password` interpolation and literal warning; `SudoPassword` removed from the serialized `Play`.
- `internal/inventory/`: expose `tack_become_password` / `ansible_become_password` host/group vars to the resolver.
- `internal/connector/{local,ssh,ssm}`: accept a per-host credential and support the `-n` probe; SSM continues to refuse passwords by default.
- Docs: `docs/playbooks.md`, `docs/connectors.md`, `docs/ci-cd.md`, `llms.txt`.
- No new dependencies.
