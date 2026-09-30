## Context

Escalation today is controlled by `play.Sudo` / `task.Sudo` plus `become_user` / `become_method`, and the password by a single `play.SudoPassword string`, filled from `--sudo-password`, `TACK_SUDO_PASSWORD`, YAML `sudo_password:`, or an interactive prompt (`executor.needsSudoPassword`). `-s` both enables escalation and forces the prompt.

v1.1.2 already fixed the most visible symptoms: the prompt is cached for the run instead of repeated per play, the wrapped command detaches stdin (`sh -c 'exec </dev/null; …'`) so an unread password can't reach it, password env vars are removed from the process environment after startup, and the SSM heredoc uses a random delimiter.

This design comes from a four-perspective panel (fleet SRE, laptop bootstrapper, security, CLI/compat). Their non-negotiables:
- **SRE:** never hang, never prompt per play.
- **Laptop:** at most one prompt, and none if sudo already works.
- **Security:** the password is never in argv, logs or stdin, and is sent only to hosts that ask.
- **CLI/compat:** every v1.1.x invocation keeps working in 1.x.

Constraint discovered while designing: the plan phase runs module `Check()` calls with escalation (`planTasks` toggles `SetSudo` per task), and checks silently fall back to "will_run" on error. The credential is therefore needed **before** plan checks, not merely before apply.

## Goals / Non-Goals

**Goals:**
- Treat "whether to escalate" and "which credential" as separate concepts end to end, in YAML, CLI, executor and connectors.
- Prompt at most once per run, at a predictable point, never mid-apply.
- Send a password only to hosts whose escalation actually requires one.
- Support per-host/group passwords (vault-backed) for mixed fleets.
- Keep every existing flag, env var and YAML key working through 1.x.

**Non-Goals:**
- Password support for `su` or `doas`. Both currently read from a TTY and remain passwordless-only; the error message says so.
- Sending passwords over SSM by default. The existing `TACK_SSM_ALLOW_SUDO_PASSWORD` opt-in is unchanged.
- Credential helpers or external secret stores (1Password, AWS Secrets Manager). The file and env sources cover these via `--become-password-file <(op read …)`.
- Removing deprecated surfaces. That is a 2.0 change.

## Decisions

### D1. `sudo` and `become` are exact, permanent aliases
Both keys are accepted at playbook, play and task level. If both are set with different values, parsing fails. `-b/--become` aliases `-s/--sudo`.
- *Alternative:* make `become` canonical and deprecate `sudo`. Rejected because every existing playbook and doc uses `sudo`, and aliasing is free.
- Task-level `become:` currently fails as "unknown module 'become'", so adding it cannot break a working playbook.

### D2. `-s`/`-b` only escalate
The prompt is decoupled from the flag. Whether tack prompts depends only on need (D5); `-K` forces the prompt upfront. This ends the eac3622 → e733222 flip-flop, where the flag's meaning kept changing.

### D3. Run-scoped `becomeCredentials` resolver on the executor
```go
type credState int // credUnset, credProvided, credNotNeeded
type credKey struct{ source, method, user string } // source = inventory host/group key or "global"
type becomeCredentials struct {
    mu      sync.Mutex
    sf      singleflight.Group     // or a hand-rolled once-per-key; no new deps
    entries map[credKey]*credEntry // {state, secret []byte, verified bool, invalid bool}
    probed  map[string]bool        // host -> needs password
}
```
- `play.SudoPassword` stops being the runtime carrier. YAML `sudo_password:` is only one of the sources, and the `Play` field loses its `yaml` tag for output so it is never serialized.
- Connectors receive the secret per host through the existing `SetSudo(enabled, password)` path.
- Secrets are zeroed in the `Run` defer, alongside the vault password.
- A hand-rolled per-key `sync.Once`-style gate avoids adding `x/sync`.

### D4. Source precedence: most specific wins
Highest first:
1. Inventory host/group var `tack_become_password` (alias `ansible_become_password`)
2. `--become-password-file` / `TACK_BECOME_PASSWORD_FILE`
3. `TACK_BECOME_PASSWORD` / `TACK_SUDO_PASSWORD`
4. Explicit `--sudo-password=VALUE` (deprecated)
5. Play `sudo_password:` (interpolated)
6. Prompt

- *Alternative (security reviewer):* CLI sources override inventory. Rejected because one CLI password cannot be correct for a mixed fleet, and it matches Ansible, where `ansible_become_password` beats `-K`.
- `-vv` logs `become password for web1: source=inventory(group:db)`, never the value.

### D5. Probe, then resolve, before plan checks
The resolution point moves from "before PLAY header" to **after host prep (connect + facts), before plan checks**:
1. During host prep, if the tag-filtered task list for that host escalates (static: play/task `sudo`, `-s`, tags applied; `when:` is not evaluated because it may depend on registered results), run the probe once per host:
   - `sudo -n true` for the sudo method;
   - `doas -n true` for doas;
   - none for su, SSM, docker or root connections, which are treated as not needing a password.
2. Hosts whose probe succeeds get `credNotNeeded`. Nothing is ever fed to their stdin.
3. On the main thread, after all preps and before plan rendering: if any host needs a password and its key has no non-interactive source, prompt **once** per key.
   - The prompt label names the key, e.g. `Sudo password (global):` or `Sudo password for group db:`.
   - `-K` moves this prompt before connecting.
4. Verify the prompted password against one needing host (`sudo -S -v`, password on stdin) before any other host uses it (D7).

- *Why not after plan (panel's first choice):* plan checks escalate, so resolving later would give wrong plans. The panel's real requirements still hold: one prompt, on the main thread, never interleaved with parallel output, never after changes have been applied, and never when nothing escalates.
- *Why the probe despite its cost (SRE's objection):* one extra round trip per escalating host per run, run in parallel during prep. It gives:
  - send-on-demand;
  - zero prompts for NOPASSWD or a cached sudo timestamp;
  - no opt-out flag needed for NOPASSWD CI.
- *Timestamp expiry mid-run:* a host probed as `credNotNeeded` whose later escalated command fails with sudo's "a password is required" is marked as needing one. If a non-interactive source exists, it is used. Otherwise the task fails with the actionable error. Tack never prompts mid-apply.
- `--no-prompt` / `--no-sudo-prompt` skip the probe as well and assume no password is needed, which is today's behavior.

### D6. Non-interactive runs fail fast, per host
With no TTY, no source for a needed key, and a failed probe, those hosts fail during prep with:

> `host web1: sudo requires a password; provide --become-password-file, TACK_BECOME_PASSWORD, or tack_become_password in inventory (or configure NOPASSWD)`

Other hosts continue. This replaces today's silent skip, which failed later with an opaque sudo error. NOPASSWD CI is unaffected because its probe succeeds.

### D7. Wrong-password handling
Detected by sudo exit 1 plus a stderr match on "incorrect password" / "Sorry, try again", or by the verify step.
- **Non-interactive source:** the entry is marked invalid run-wide, with no retry. All hosts using that key fail their escalated tasks without attempting sudo, which protects PAM faillock across fleets.
- **Prompt:** verification happens on a single host before fan-out. Up to 2 re-prompts are allowed (3 attempts, matching sudo's own default), then the run aborts before apply.

### D8. CLI surface and deprecations
| Surface | 1.2 behavior |
|---|---|
| `-s/--sudo`, `-b/--become` | escalate all tasks; no prompt |
| `-K/--ask-become-pass`, `--ask-sudo-pass` | prompt upfront |
| `--become-password-file PATH\|-` | read first line; warn if file mode is group/world-readable |
| `--no-prompt` | never prompt for any credential (sudo, SSH, vault) |
| `--no-sudo-prompt`, `TACK_SUDO_NO_PROMPT` | permanent aliases (sudo scope only) |
| `TACK_BECOME_PASSWORD[_FILE]` | new; `TACK_SUDO_PASSWORD` permanent alias |
| bare `--sudo-password` | alias of `-K` |
| `--sudo-password=VALUE` | works, warns once: exposed in process listings/history; removed in 2.0 |
| `--sudo-password VALUE pb.yml` | error: "--sudo-password takes its value with '=' …" when the positional arg is not an existing file and a second positional follows |
| YAML literal `sudo_password: foo` | works, warns: use a vault var reference |

### D9. Redaction
- Resolved secrets are registered with a new executor-level secret scrubber. There is none today; `no_log` only suppresses whole tasks. Any occurrence in task output, errors, plan/diff, JSON events or `tack export` is replaced with `********`. The scrubber is designed so vault values can register with it later.
- `Play.SudoPassword` gets `json:"-"` and is excluded from export builders.

## Risks / Trade-offs

- **[Probe misreads a host]:** e.g. `sudo -n` is disallowed by policy, or sudo is not installed. → A failure to run the probe (as opposed to "a password is required") is treated as "needs password". Worst case is one unnecessary prompt, never a leak. Mitigated further by the v1.1.2 stdin detach.
- **[Static escalation detection over-approximates]:** tasks guarded by `when:` that won't run still count. → Worst case is one prompt that wasn't strictly needed. That is acceptable, and far better than a mid-apply prompt.
- **[Behavior change in non-TTY runs]:** CI that relied on a silent skip with a host that really needs a password now fails in prep instead of at the task. → Strictly earlier failure of a run that would fail anyway. Called out in the release notes.
- **[Inventory var holds plaintext]:** `tack_become_password` in a plain inventory file. → Warn when the value did not come from vault or interpolation, the same as for `sudo_password:`.
- **[Extra round trip per host]:** → Only for hosts with escalated tasks, done in the parallel prep phase, and skipped with `--no-prompt`.

## Migration Plan

1. 1.2 ships all new surfaces. Old surfaces keep working, with warnings only for `--sudo-password=VALUE` and literal `sudo_password:`.
2. Docs are rewritten around "escalation vs credential", with a migration table.
3. 2.0 removes `--sudo-password=VALUE`.

Rollback: the change is additive, and reverting the release restores the 1.1.x behavior with no data or format migration.

## Open Questions

- Should `--no-prompt` also imply `--no-ssh-prompt` and skip the vault prompt? This is proposed as yes, and needs confirming with the vault UX.
- Inventory var naming: `tack_become_password` with `ansible_become_password` accepted. Should a plain `become_password` also be accepted? The panel was split 2–2; the default here is no, to avoid collisions with user vars.
