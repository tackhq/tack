## Why

Teams running tack in CI/CD pipelines need a human-in-the-loop approval step between planning and applying infrastructure changes. Without this, pushes to main trigger immediate applies — unacceptable for production environments. CI systems (GitHub Actions, Jenkins) already provide native pause/approve mechanisms, but users need guidance, example workflows, and notification scripts to wire it all together with tack's existing `--dry-run` and `--output json` capabilities.

This is Track 1 of a two-track effort. Track 1 delivers docs and examples using what tack and CI systems already support today. Track 2 (deferred) adds frozen plan files to eliminate drift between approval and apply.

## What Changes

- **New documentation**: `docs/ci-cd.md` guide explaining the plan→approve→apply pattern for CI/CD pipelines
- **New GH Actions example**: Complete workflow using `environments` with required reviewers for the approval gate
- **New Jenkins example**: Equivalent pipeline using `input` step for approval
- **New Slack notification script**: Shell script that reads tack's JSON plan output and posts a formatted message with plan summary and approval link
- **New Teams notification script**: Same for Microsoft Teams incoming webhooks
- **Drift limitation documented**: Honest disclosure that without frozen plan files (Track 2), apply re-evaluates state and may diverge from the approved plan

## Capabilities

### New Capabilities
- `cicd-approval-workflow`: Documentation and example CI workflows demonstrating tack's plan→approve→apply pattern using native CI system features (GH Actions environments, Jenkins input step) with Slack/Teams notifications

### Modified Capabilities

_None — no existing specs or tack code changes._

## Impact

- No code changes — documentation and examples only
- New files under `docs/` and `examples/ci/`
- Users gain copy-paste-ready CI integration patterns
- Sets expectations for Track 2 (frozen plan files)
