## Context

Tack's existing end-of-run path prints a summary (counts, failed hosts, duration). That data is already accumulated in the executor/output pipeline — the hooks feature needs to harvest the same state rather than re-deriving it.

The design deliberately avoids in-process plugins and callback registries. Those require stability guarantees (plugin API, lifecycle semantics, versioning) that Tack hasn't earned yet. A subprocess with stdin-JSON is the Unix-idiomatic, language-agnostic, low-commitment contract: users write Python, bash, Go, anything.

## Goals / Non-Goals

**Goals:**
- Zero playbook changes — hooks configured entirely at invocation time (CLI/env).
- Simple contract: JSON on stdin, exit code 0/non-zero, timeout-bounded.
- Doesn't affect tack's exit code — hooks are observability, not policy.
- Works in CI pipelines (no interactive terminals required).
- Repeatable flags for multiple destinations (e.g., Slack + metrics + audit).

**Non-Goals:**
- Webhook support (users wrap `curl` / `wget`).
- Per-task / per-play / mid-run hooks.
- Plugin system or compiled callback modules.
- Configurable payload shape — one stable shape, documented.
- Running hooks on target hosts — hooks are control-host only.
- Retry/backoff — if the hook needs retries, the hook script owns that.
- Parallel hook execution.

## Decisions

### Decision 1: Subprocess via `/bin/sh -c`

Hook commands run via `exec.CommandContext(ctx, "/bin/sh", "-c", cmd)`. This lets users pass pipelines, env expansion, and shell idioms (`curl ... | jq ...`). On Windows, use `cmd.exe /C` — but Windows control-host is not officially supported anyway, so document as Linux/macOS.

**Alternatives considered:**
- **Parse cmd into argv with `shlex`:** safer but blocks natural shell idioms. Users are passing arbitrary commands already — `sh -c` is the expected contract.
- **Require structured command config:** heavier surface; contradicts "zero config" goal.

### Decision 2: JSON payload on stdin, versioned via `schema_version`

Payload includes `"schema_version": 1` so future breaking changes can coexist via version sniffing. Stdin (rather than env vars or argv) keeps payload size unbounded and JSON-friendly.

### Decision 3: Hook runs after output is flushed

Order at end of run:
1. Executor finishes all plays.
2. Output/summary is flushed to stdout/stderr.
3. Hooks run sequentially.
4. Tack exits with the playbook's exit code.

This ensures users see the normal summary before any hook side-effect delays (e.g., a 30s webhook timeout).

### Decision 4: Sequential execution, registration order

`--on-failure A --on-failure B --on-complete C`, on failure, runs `A` then `B` then `C`. Rationale: predictable behavior, no thundering-herd, simple timeout accounting, users can order by importance. If a user wants parallel, they use `&` in the shell command.

### Decision 5: Timeout with SIGTERM → SIGKILL

On timeout: send SIGTERM, wait 2 seconds, then SIGKILL. Record timeout as a warning. The 2s grace is not configurable in v1.

### Decision 6: Hook output capture

Stdout + stderr are captured (combined) into a buffer, truncated at 64KB per hook. On non-verbose runs, only printed if the hook exits non-zero (as a warning). On `-v`/`-vv`, always printed. Truncation emits `... [truncated, exceeded 64KB]` suffix.

### Decision 7: Hook failures never change tack's exit code

If a hook exits non-zero or times out:
- Print warning `tack: hook "<cmd>" failed: <reason>` to stderr.
- Continue to next hook.
- Tack exits with the run's original exit code.

Rationale: hooks are observability; making them affect exit codes creates a footgun where a broken notification breaks the pipeline.

### Decision 8: Flag/env precedence

If both `--on-failure` flag(s) AND `TACK_ON_FAILURE` env var are set → **flags take precedence** (env ignored). If only env is set → split on comma to allow multiple commands. Commas inside commands must be escaped via `\,` in env form. Document the env form as less expressive than repeated flags.

### Decision 9: Payload shape

```json
{
  "schema_version": 1,
  "run_id": "rnd-uuid-v4",
  "status": "success" | "failed",
  "playbook": "site.yml",
  "started_at": "2026-04-04T10:00:00Z",
  "ended_at":   "2026-04-04T10:02:15Z",
  "duration_ms": 135000,
  "failed_task_count": 0,
  "changed_task_count": 12,
  "ok_task_count": 48,
  "hosts": [
    {
      "name": "web01",
      "status": "success" | "failed" | "skipped" | "unreachable",
      "ok_task_count": 24,
      "changed_task_count": 6,
      "failed_tasks": [{"task": "apt install nginx", "msg": "..."}],
      "duration_ms": 67000
    }
  ]
}
```

`run_id` is a freshly generated UUIDv4 per tack invocation.

### Decision 10: `status` semantics

Run-level `status: failed` when ANY host has `status: failed`. Otherwise `success`. `unreachable` hosts count as failed (ops expect alerts for unreachable hosts).

### Decision 11: Hook stdin is closed after payload write

Write payload → close stdin. Hooks that don't read stdin still work (their stdin just closes early). Hooks that read stdin get the JSON.

### Decision 12: Env passed to hook

Hook inherits tack's process env PLUS:
- `TACK_RUN_ID` — run_id
- `TACK_RUN_STATUS` — `success`/`failed`
- `TACK_PLAYBOOK` — playbook path

This lets trivial hooks avoid JSON parsing: `slack-notify "Run $TACK_RUN_ID status: $TACK_RUN_STATUS"`.

## Risks / Trade-offs

- **[Risk]** Users expect hooks to run on targets → **Mitigation:** Document clearly as control-host-only. Add a note in hook warnings when invoked.
- **[Risk]** Long-running hooks delay tack exit → **Mitigation:** Default 30s timeout + configurable. Document that CI timeouts need to account for hook time.
- **[Risk]** Secret leakage: hook payload might include sensitive task output → **Mitigation:** `failed_tasks[].msg` SHALL respect `no_log:` / vault redaction already applied by the output layer. Tests verify.
- **[Trade-off]** `sh -c` means injection via variable expansion is possible — but the user is supplying the command, so trust model is "user's own shell". Documented.
- **[Trade-off]** No retry/backoff means flaky webhook notifications get lost → **Mitigation:** Recommend users wrap with `curl --retry` or similar.
- **[Trade-off]** Sequential execution extends runtime when many hooks are configured → **Mitigation:** Users can background commands with `&`; parallel execution is v2.
- **[Risk]** Payload schema evolution → **Mitigation:** `schema_version: 1` included from day one; documented as stable contract.
