## Why

LLMs don't know Bolt -- it's not in their training data. When asked to generate Bolt playbooks, they hallucinate Ansible syntax (Jinja2 templates, `become:` instead of `sudo:`, wrong module params). The existing human-oriented docs are spread across multiple files and contain narrative prose that's inefficient for LLM context windows. A single, self-contained `llms.txt` file following the emerging convention gives LLM agents everything they need to correctly generate playbooks, CLI commands, and configurations without hallucination.

## What Changes

- Add `llms.txt` at the repo root -- a single, self-contained file optimized for LLM consumption
- Covers: project identity, Ansible-vs-Bolt differences, playbook YAML schema, all module parameters with types/defaults/valid values, CLI reference, connector configuration, variable system rules
- Includes explicit anti-hallucination sections ("Do NOT use Jinja2 syntax", "Use `sudo:` not `become:`")
- Targets ~500-800 lines, dense and structured, no narrative prose
- No code changes required -- this is a documentation-only addition

## Capabilities

### New Capabilities
- `llms-txt`: Self-contained LLM-optimized documentation file covering Bolt's playbook schema, module reference, CLI usage, and connector configuration

### Modified Capabilities

_None -- this adds a new file without changing any existing behavior or specs._

## Impact

- **New file**: `llms.txt` at repo root
- **No code changes**: Pure documentation addition
- **No dependency changes**
- **Maintenance**: File should be updated when modules, CLI flags, or playbook schema change
