## 1. Documentation Guide

- [x] 1.1 Create `docs/ci-cd.md` with overview of the plan→approve→apply pattern, explaining why human-in-the-loop matters for production applies
- [x] 1.2 Add GitHub Actions section to `docs/ci-cd.md` explaining environment protection rules, required reviewers setup, and the two-job workflow
- [x] 1.3 Add Jenkins section to `docs/ci-cd.md` explaining the `input` step pipeline pattern
- [x] 1.4 Add Slack/Teams notification section to `docs/ci-cd.md` explaining webhook setup and the notification scripts
- [x] 1.5 Add limitations section to `docs/ci-cd.md` documenting the drift problem (apply re-plans) and referencing Track 2 frozen plan files as the future fix

## 2. GitHub Actions Example

- [x] 2.1 Create `examples/ci/github-actions/plan-approve-apply.yaml` with `plan` job that runs `tack run --dry-run --output json`, captures output, and uploads as artifact
- [x] 2.2 Add Slack notification step to the `plan` job using the notification script
- [x] 2.3 Add `apply` job with `environment: production` gate, artifact download, and `tack run`

## 3. Jenkins Example

- [x] 3.1 Create `examples/ci/jenkins/Jenkinsfile` with Plan stage running `tack run --dry-run --output json` and stashing output
- [x] 3.2 Add Notify stage calling the Slack notification script
- [x] 3.3 Add Approve stage with `input` step and configurable timeout
- [x] 3.4 Add Apply stage unstashing plan output and running `tack run`

## 4. Notification Scripts

- [x] 4.1 Create `examples/ci/slack-notify.sh` that reads JSON plan output, formats a Slack message with repo/branch/commit info, change counts, truncated task list, and approval link, then posts via `SLACK_WEBHOOK_URL`
- [x] 4.2 Create `examples/ci/teams-notify.sh` with equivalent functionality for Teams via `TEAMS_WEBHOOK_URL` using Adaptive Card format
- [x] 4.3 Add error handling for missing webhook URL and missing input file in both scripts

## 5. Integration

- [x] 5.1 Add CI/CD guide link to main `docs/README.md` table of contents
- [x] 5.2 Verify all example files reference correct tack CLI flags (`--dry-run`, `--check`, `--output json`)
