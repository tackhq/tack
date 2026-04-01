## Context

These are four independent UX fixes bundled into one change because they're all small, low-risk, and improve the CLI surface. None require architectural changes.

## Goals / Non-Goals

**Goals:**
- Remove misleading `--forks` flag
- Make dry-run flag behavior consistent
- Accept common approval responses
- Provide inline module documentation

**Non-Goals:**
- Implementing parallel execution (separate change)
- Full man-page style documentation
- Interactive module parameter wizard

## Decisions

### 1. Remove `--forks` rather than warn

Removing the flag gives a clear error ("unknown flag: --forks") which is better than a deprecation warning that users might miss. When parallel execution is implemented, the flag returns.

**Alternative considered:** Print a warning when `--forks` is used. Rejected because warnings in stdout can break piped output, and the flag literally does nothing.

### 2. Case-insensitive approval matching

Use `strings.EqualFold` to match against "y" and "yes". This handles Y, y, Yes, YES, yEs, etc. No other inputs are accepted (not "ok", "sure", etc.).

### 3. Module help via `Describer` interface

Add an optional `Describer` interface to the module system:
```go
type Describer interface {
    Description() string
    Parameters() []ParamDoc
}
```

Modules that implement it get rich help output. Those that don't show a basic "no documentation available" message. This is opt-in so existing modules don't need changes immediately.

### 4. Keep --check as alias

`--check` is familiar to Ansible users. Keep it as a documented alias for `--dry-run` but make both PersistentFlags so they work identically at all command levels.

## Risks / Trade-offs

- **[Risk] Breaking `--forks` removal** — Scripts using `-f 1` will break. → Acceptable since the flag never functioned; the "break" changes nothing about actual behavior.
- **[Trade-off] Describer is optional** — Not all modules will have rich help immediately. → Acceptable; can be backfilled incrementally.
