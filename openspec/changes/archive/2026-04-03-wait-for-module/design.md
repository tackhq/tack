## Context

Bolt modules implement the `Module` interface (`Name()` + `Run()`) and register via `init()`. Each module lives in its own subpackage under `internal/module/`. The `wait_for` module follows this pattern but is unique in that it introduces time-based polling — something no existing module does.

The module must work across all four connector types (local, SSH, Docker, SSM). Port and URL checks run from the controller side (no connector needed), while path and command checks execute on the remote target via `connector.Execute()`.

**Stakeholders:**
- **DevOps engineers** — primary users who need to gate deployments on service readiness
- **Python developers** — familiar with Ansible's `wait_for`, expect similar UX
- **PM** — wants reliable playbook execution without manual retry workarounds
- **IT support** — needs clear error messages when waits time out

## Goals / Non-Goals

**Goals:**
- Provide a single `wait_for` module that handles port, path, command, and URL conditions
- Follow Bolt's module conventions exactly (params via `map[string]any`, `Result` return, `init()` registration)
- Configurable timeout and poll interval with sensible defaults (300s timeout, 5s interval)
- Return useful data in `Result.Data` (elapsed time, attempts, condition details)
- Support `state: started` (wait for condition to be true) and `state: stopped` (wait for condition to be false) for port and path types
- Implement `Checker` interface returning `UncertainChange` (can't predict future state)

**Non-Goals:**
- No regex/content matching on URL responses or command output (keep it simple for v1)
- No WebSocket or UDP support — TCP only for port checks
- No parallel condition checking (wait for multiple conditions at once)
- No persistent connections or connection pooling for URL checks
- No custom HTTP headers, auth, or request body for URL checks (use `command` + `curl` for complex HTTP)

## Decisions

### 1. Single module with `type` parameter vs. four separate modules

**Decision:** Single `wait_for` module with a required `type` parameter (`port`, `path`, `command`, `url`).

**Why:** Ansible uses a single `wait_for` module and the mental model is "wait for X". Splitting into `wait_for_port`, `wait_for_path`, etc. would clutter the module namespace for closely related functionality. The implementation shares polling logic across all types.

**Alternatives considered:**
- Four separate modules — rejected because they'd share 80% of their code and the single module is more discoverable

### 2. Where do port/URL checks execute?

**Decision:** Port and URL checks run on the controller (the machine running Bolt), not on the target.

**Why:** This matches the primary use case: "wait until I can reach the service from where I'm deploying." Running `net.Dial` or `http.Get` from the controller is simpler, avoids requiring `curl`/`nc` on the target, and works identically across all connector types.

For cases where users need to check from the target's perspective, they can use `type: command` with `cmd: "nc -z localhost 8080"`.

**Alternatives considered:**
- Execute on target via connector — rejected because it requires tool availability on the target and the common case is checking reachability from the controller

### 3. Polling implementation

**Decision:** Simple `time.Ticker` loop with `context.WithTimeout`. The poll loop checks the condition, sleeps for the interval, and repeats until success or timeout.

**Why:** Go's `time.Ticker` + context cancellation is idiomatic, handles cleanup on SIGINT, and avoids complex state machines. The granularity of seconds is sufficient for infrastructure readiness checks.

**Alternatives considered:**
- Exponential backoff — rejected for v1; fixed interval is simpler and more predictable for users. Can add `backoff: exponential` parameter later.

### 4. Port check mechanism

**Decision:** Use `net.DialTimeout` with a per-attempt connection timeout of `min(interval, 5s)`.

**Why:** Simple, stdlib-only, works for TCP. The per-attempt timeout prevents a single slow connection from consuming the entire poll interval.

### 5. URL check mechanism

**Decision:** Use `http.Client` with per-request timeout. Success = status code in 200-399 range. Follow redirects (default Go behavior).

**Why:** Simple success criteria that covers the common "is this service up?" case. No need to parse response bodies or match specific codes for v1.

### 6. Module result semantics

**Decision:** The module always returns `Changed: true` on success (the condition was met), `error` on timeout. This follows the `command` module pattern — the module performed an action (waiting) and it completed.

**Why:** `wait_for` is not idempotent in the traditional sense — it's a synchronization primitive. Returning `Changed: true` signals that execution proceeded past the wait point.

## Risks / Trade-offs

- **[Risk] Long-running tasks block playbook execution** → Mitigated by mandatory timeout (default 300s). Users can set shorter timeouts. Context cancellation (Ctrl+C) stops the wait immediately.

- **[Risk] Port checks from controller may not reflect target-local connectivity** → Documented clearly. Users needing target-perspective checks should use `type: command` with appropriate tooling.

- **[Risk] URL checks may be affected by proxies, DNS, or TLS issues** → The error message will include the underlying Go error. TLS verification uses system defaults (can be revisited if `insecure` parameter is requested).

- **[Trade-off] No content matching on URL responses** → Keeps v1 simple. Complex HTTP checks can use `type: command` with `curl`. Content matching can be added as a `response_regex` parameter in a future version.

- **[Trade-off] Fixed polling interval, no backoff** → Simpler for users to reason about. A 5-second default is a reasonable balance between responsiveness and resource usage for infrastructure checks.
