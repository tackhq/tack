## Context

Tack already supports `--dry-run`/`--check` for plan mode and `--output json` for machine-readable output. CI systems (GitHub Actions, Jenkins) have native approval gates. Slack and Teams support incoming webhooks for notifications. All the pieces exist — this change connects them with documentation and copy-paste-ready examples.

This is a docs-only change. No Go code is modified.

## Goals / Non-Goals

**Goals:**
- Provide a complete, working GH Actions workflow for plan→approve→apply
- Provide an equivalent Jenkins pipeline
- Provide notification scripts for Slack and Teams that surface plan summaries
- Document the pattern clearly enough that users can adapt it to other CI systems
- Honestly document the drift limitation (apply re-plans, doesn't replay the approved plan)

**Non-Goals:**
- Frozen plan files (`--out plan.tack` / `--plan <file>`) — that's Track 2
- Interactive Slack/Teams approval buttons (bidirectional integration)
- Slack-to-GitHub identity mapping
- Custom approval UI outside CI system
- Supporting CI systems beyond GH Actions and Jenkins in v1

## Decisions

### 1. Approval happens in CI system UI, not in Slack/Teams

Slack/Teams messages link out to the CI run for approval. The actual approve click is in GitHub UI or Jenkins UI.

**Why over in-Slack buttons:** Avoids building a Slack app, webhook receiver, and identity mapping. CI systems already provide audited approval with proper identity, MFA, and role-based access. Track 1 should have zero infrastructure requirements beyond what users already have.

### 2. Notification scripts are standalone shell scripts, not built into tack

Notification is handled by `examples/ci/slack-notify.sh` and `examples/ci/teams-notify.sh` — not a `tack notify` subcommand.

**Why over tack subcommand:** Keeps tack's surface area small. Notification config (webhook URLs, channels) belongs in CI secrets, not in tack's config model. Shell scripts are transparent, forkable, and don't add dependencies to the Go binary.

### 3. GH Actions workflow uses `environment` protection rules

The workflow splits into two jobs: `plan` and `apply`. The `apply` job references an environment (e.g., `production`) that has required reviewers configured. GitHub natively blocks the job until a reviewer approves.

**Why over manual dispatch or issue-based approval:** Environment protection is the standard GH Actions pattern. It integrates with branch protection, deployment status, and the deployments API. No custom Actions or external services needed.

### 4. Jenkins pipeline uses `input` step

The Jenkinsfile uses a declarative pipeline with a `input` stage between plan and apply.

**Why over other Jenkins mechanisms:** `input` is the canonical Jenkins approval primitive. It's supported in both declarative and scripted pipelines, has built-in timeout, and integrates with Jenkins' authorization model.

### 5. Plan output passed as CI artifact between jobs

GH Actions: `upload-artifact`/`download-artifact`. Jenkins: `stash`/`unstash`. The plan output (both human-readable and JSON) is captured in the plan job and made available to the apply job and notification step.

**Why:** Jobs may run on different runners/agents. Artifacts are the standard mechanism for inter-job data transfer in both CI systems.

### 6. File organization under `examples/ci/`

```
examples/ci/
├── github-actions/
│   └── plan-approve-apply.yaml
├── jenkins/
│   └── Jenkinsfile
├── slack-notify.sh
└── teams-notify.sh
```

Plus `docs/ci-cd.md` as the main guide.

## Risks / Trade-offs

**[Drift between plan and apply]** → Document clearly. Apply re-evaluates state — if infrastructure changed between approval and apply, the actual changes may differ from what was approved. Mitigated in Track 2 with frozen plan files. For now, recommend short approval windows and avoiding concurrent changes.

**[Examples go stale]** → Keep examples minimal and well-commented. Reference tack CLI flags by name so users can check `tack run --help` for current options.

**[Webhook URL security]** → Document that webhook URLs should be stored as CI secrets, never committed. Scripts read from environment variables only.
