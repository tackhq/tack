## 1. Emitter signature

- [x] 1.1 Update the `Emitter` interface in `internal/output/output.go` so `PromptApproval` takes a `target string` parameter.
- [x] 1.2 Update `Output.PromptApproval(target)` to render `Apply these changes to <target>? (yes/no): ` and keep its existing signal/scanner logic.
- [x] 1.3 Update `JSONEmitter.PromptApproval(target)` in `internal/output/json.go` to accept the arg, ignore it, and continue to auto-approve.

## 2. Executor target formatting

- [x] 2.1 Add `formatApprovalTarget(hosts []string, connection string) string` in `internal/executor/`.
- [x] 2.2 Single-host case: when `len(hosts) == 1`, return `<host> (<connection>)`.
- [x] 2.3 Multi-host case ≤5: return `<N> hosts (h1, h2, ..., hN)`.
- [x] 2.4 Multi-host case >5: return `<N> hosts (h1, h2, h3, h4, h5, ...)`.
- [x] 2.5 Wire the helper into the single-host call site at `runPlayOnHost` (passing `[]string{host}` and `play.GetConnection()`).
- [x] 2.6 Wire the helper into the multi-host call site at `runMultiHostPlay` (passing `play.Hosts` and `play.GetConnection()`).

## 3. Tests

- [x] 3.1 Unit-test `formatApprovalTarget` for: single host with various connection types, exactly five hosts, six hosts (truncation case), empty connection (defaults to `local`).
- [x] 3.2 Unit-test `Output.PromptApproval` to confirm the prompt line contains the formatted target string.
- [x] 3.3 Confirm `JSONEmitter.PromptApproval` returns `true` regardless of arg (auto-approve).
- [x] 3.4 Update any existing executor or output tests that stub `PromptApproval` to match the new signature.
- [x] 3.5 Run `make test` and `make lint` clean.

## 4. Documentation

- [x] 4.1 No user-facing doc change planned. If the prompt format is documented anywhere (e.g. docs/getting-started or README quickstart), update the example output to reflect the new text.
