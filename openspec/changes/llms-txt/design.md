## Context

Bolt's docs are human-oriented prose spread across 7 markdown files. LLM agents can't follow cross-file links and waste context window on narrative. The `llms.txt` convention (place a curated LLM-optimized file at repo root) is emerging as the standard way to make projects LLM-accessible.

Bolt has a specific challenge: LLMs will hallucinate Ansible syntax unless explicitly told otherwise. The file must actively override Ansible assumptions.

## Goals / Non-Goals

**Goals:**
- Single self-contained file an LLM can consume in one context load
- Structured for fast lookup (not narrative prose)
- Explicit anti-hallucination rules for Ansible-trained models
- Cover all modules with machine-parseable parameter specs
- Stay under ~800 lines to fit comfortably in context windows

**Non-Goals:**
- Auto-generation from code (future enhancement, not this change)
- `bolt module --format yaml` or `bolt schema` commands (separate changes)
- Replacing human docs (llms.txt supplements, doesn't replace)
- Serving llms.txt via HTTP (repo file only for now)

## Decisions

### 1. Single flat file, not structured data

Use markdown with consistent heading structure rather than YAML/JSON schema. LLMs parse markdown natively and it allows mixing structured param tables with examples and rules.

**Alternative considered:** JSON Schema for modules. Rejected because it's harder to include examples and anti-hallucination prose inline. Could be added later as a complement.

### 2. File structure: identity → rules → schema → modules → CLI → connectors → variables

Lead with project identity and "NOT Ansible" rules so they're highest-priority in the context window. Module reference is the bulk. CLI and connectors are compact reference sections.

### 3. Module params as markdown tables with strict format

Each module gets: one-line description, params table (name/type/required/default/description), valid enum values listed inline, one minimal example. No prose paragraphs.

### 4. Anti-hallucination as explicit "RULES" section

A dedicated section near the top with numbered rules like "Use `sudo: true` not `become: true`", "Templates use Go `{{ .var }}` syntax, NOT Jinja2". LLMs follow explicit numbered rules well.

## Risks / Trade-offs

- **[Risk] Staleness** — llms.txt can drift from code as modules/flags change. → Mitigation: Keep format simple enough to update manually. Document in CLAUDE.md that llms.txt must be updated when modules or CLI change. Future: auto-generate from `bolt module --format yaml`.
- **[Trade-off] Duplication** — Content overlaps with docs/modules.md and README. → Acceptable because llms.txt serves a different audience (machines) and must be self-contained.
- **[Trade-off] File size** — ~600-800 lines is large for a single file but necessary for completeness. Smaller would omit modules and force agents to guess.
