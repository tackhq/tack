## 1. Escalation vocabulary (become-escalation, playbook-defaults)

- [ ] 1.1 Parse `become:` as an alias of `sudo:` at task level (add it to `knownTaskFields`) and at play level, with a conflict error when both are set and disagree
- [ ] 1.2 Accept `become` in the playbook-level mapping defaults, with the same conflict check
- [ ] 1.3 Add `-b/--become` as an alias of `-s/--sudo` on `run` (and on any other command that has `--sudo`)
- [ ] 1.4 Parser tests: task/play/playbook `become`, agreeing and conflicting combinations, `-s` with task `sudo: false`

## 2. Credential resolver core (become-credentials)

- [ ] 2.1 Add a `becomeCredentials` type in `internal/executor/`: tri-state entries keyed by (source key, method, user), `[]byte` secrets, a per-key once-gate, and a per-host probe cache
- [ ] 2.2 Zero all secrets in the `Run` defer, next to the vault password; remove `promptedSudoPassword`/`sudoPasswordPrompted` (the v1.1.2 cache) in favor of the resolver
- [ ] 2.3 Implement source precedence: inventory `tack_become_password`/`ansible_become_password` > file > `TACK_BECOME_PASSWORD`/`TACK_SUDO_PASSWORD` > `--sudo-password=VALUE` > interpolated `sudo_password:` > prompt
- [ ] 2.4 Interpolate play `sudo_password:` with play vars and vault vars; warn once on literal values
- [ ] 2.5 Stop using `play.SudoPassword` as the runtime carrier: `GetConnector`, `applyBecome`, `planTasks` and apply-time `SetSudo` get the per-host secret from the resolver
- [ ] 2.6 Add `json:"-"` to `Play.SudoPassword` and exclude it from export builders
- [ ] 2.7 `-vv` logging of the password source per host (never the value)
- [ ] 2.8 Unit tests for precedence, tri-state (empty is provided), one resolution per key across plays, and zeroing

## 3. Probe and send-on-demand

- [ ] 3.1 Add static "escalates" detection over the tag-filtered task list, including play `sudo`, task `sudo`/`become`, blocks, includes and handlers; `when:` is ignored
- [ ] 3.2 Run a `sudo -n true` / `doas -n true` probe once per host during prep, only for local/SSH hosts that escalate; skip su, SSM, Docker, already-root connections and `--no-prompt`
- [ ] 3.3 Treat probe errors other than "a password is required" as needing a password
- [ ] 3.4 Never feed a password to hosts marked not-needed (the connector gets an empty password)
- [ ] 3.5 Mid-run expiry: on a sudo "password is required" failure for a not-needed host, switch to a non-interactive source if one exists and retry once; otherwise fail the task with the actionable error
- [ ] 3.6 Tests with a fake connector: NOPASSWD host gets no stdin, mixed fleet, `--tags` with no escalated tasks means no probe

## 4. Prompt timing and non-interactive behavior

- [ ] 4.1 Move credential resolution to the main thread after `discoverAndPlanParallel` prep (connect, facts, probe) and before plan checks; split prep so plan checks run after resolution. Cover the single-host `runPlayOnHost` path too
- [ ] 4.2 Prompt once per key with a label naming the key; never prompt during apply
- [ ] 4.3 `-K/--ask-become-pass` (alias `--ask-sudo-pass`, plus bare `--sudo-password`) prompts before connecting
- [ ] 4.4 Non-TTY, needing a password, no source: fail that host during prep with an error naming `--become-password-file`, `TACK_BECOME_PASSWORD` and `tack_become_password`
- [ ] 4.5 Replace `configureSudoPrompt` / `SudoPromptRequested` so that `-s` no longer influences prompting; keep TestConfigureSudoPrompt's guarantee that `-a` does not suppress a needed prompt
- [ ] 4.6 Tests: prompt appears before plan output; `-a` prompts once; non-TTY missing source fails only the affected host; NOPASSWD CI passes

## 5. Wrong-password handling

- [ ] 5.1 Detect sudo auth failure (exit 1 plus "incorrect password" / "Sorry, try again" on stderr) in local and SSH execution paths
- [ ] 5.2 Verify prompted passwords with `sudo -S -v` on one needing host before fan-out; allow up to 2 re-prompts, then abort before apply
- [ ] 5.3 Non-interactive sources: mark the entry invalid run-wide on first rejection; the remaining hosts fail without attempting sudo
- [ ] 5.4 Tests: a bad env password across N hosts makes exactly one attempt; a typo then a correct entry sends the first entry to at most one host

## 6. CLI surface and deprecations

- [ ] 6.1 Add `--become-password-file PATH|-` and `TACK_BECOME_PASSWORD_FILE`: read the first line, warn if group/world-readable
- [ ] 6.2 Add `TACK_BECOME_PASSWORD`; add `TACK_BECOME_PASSWORD` to the startup env scrub (`cmd/tack/secretenv.go`)
- [ ] 6.3 Add `--no-prompt` covering sudo, SSH and vault prompts; keep `--no-sudo-prompt` / `TACK_SUDO_NO_PROMPT` as aliases
- [ ] 6.4 Print a one-time deprecation warning for `--sudo-password=VALUE`
- [ ] 6.5 Detect `--sudo-password VALUE pb.yml` (first positional is not an existing file and a second positional follows) and error with guidance
- [ ] 6.6 Update help text for `-s`, `-b`, `-K`, `--become-password-file` and `--no-prompt`; document the precedence order in `tack run --help`

## 7. Redaction

- [ ] 7.1 Add an executor secret scrubber; register resolved become passwords with it
- [ ] 7.2 Apply the scrubber to task stdout/stderr before display and register, to error messages, to plan/diff content and to JSON output events
- [ ] 7.3 Tests: a task echoing the password registers `********`; an error containing the password is scrubbed; export contains no password

## 8. Docs and release

- [ ] 8.1 Rewrite the privilege-escalation docs (`docs/playbooks.md`, `docs/connectors.md`, `docs/ci-cd.md`, `llms.txt`) around "escalation vs credential", with the precedence table and a migration table
- [ ] 8.2 Release notes: behavior changes (no prompt from `-s` alone, earlier non-TTY failure, probe) and deprecations
- [ ] 8.3 Run `make test`, `make lint` and the integration tests (NOPASSWD and password-sudo containers)
