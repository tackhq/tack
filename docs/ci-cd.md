# CI/CD Integration

This guide explains how to run tack in a CI/CD pipeline with a human-in-the-loop approval step between planning and applying changes.

## The Plan → Approve → Apply Pattern

Running tack directly on push to production is risky — changes apply immediately with no review. A safer pattern splits the pipeline into two phases:

1. **Plan** — Run `tack run --dry-run` to preview what would change. Capture the output.
2. **Approve** — Notify the team (Slack, Teams), then pause until a human reviews and approves.
3. **Apply** — Run `tack run` to apply the approved changes.

This uses your CI system's native approval mechanisms — no custom infrastructure required.

## GitHub Actions

GitHub Actions supports this pattern through [environment protection rules](https://docs.github.com/en/actions/deployment/targeting-different-environments/using-environments-for-deployment).

### Setup

1. **Create an environment** in your repo settings (e.g., `production`)
2. **Add required reviewers** — one or more users/teams who must approve before the apply job runs
3. **Use the example workflow** below

When the `plan` job completes, the `apply` job will show as "Waiting" in the GitHub UI. Required reviewers receive a GitHub notification and can approve or reject directly from the Actions run page.

### Example Workflow

See [`examples/ci/github-actions/plan-approve-apply.yaml`](../examples/ci/github-actions/plan-approve-apply.yaml) for a complete, working workflow.

The workflow:
- Runs on push to `main`
- Plan job: runs `tack run --dry-run --output json`, uploads plan output as an artifact, sends a Slack notification with a summary
- Apply job: requires the `production` environment (with required reviewers), downloads the plan artifact for reference, then runs `tack run`

### Environment Variables

Set these as GitHub Actions secrets:

| Secret | Required | Description |
|---|---|---|
| `SLACK_WEBHOOK_URL` | No | Slack incoming webhook URL for notifications |
| `TACK_SSH_KEY` | Depends | SSH private key if using SSH connector |
| `TACK_SSH_PASSWORD` | Depends | SSH password if using password auth |
| `TACK_SUDO_PASSWORD` | Depends | Sudo password for privilege escalation |

## Jenkins

Jenkins supports this pattern through the [`input` step](https://www.jenkins.io/doc/pipeline/steps/pipeline-input-step/) in Pipeline scripts.

### Setup

1. **Create a Pipeline job** pointing to your `Jenkinsfile`
2. **Configure credentials** for any SSH keys, passwords, or cloud provider access tack needs
3. The `input` step handles approval — authorized users click Approve or Abort in the Jenkins UI

### Example Pipeline

See [`examples/ci/jenkins/Jenkinsfile`](../examples/ci/jenkins/Jenkinsfile) for a complete, working pipeline.

The pipeline:
- Plan stage: runs `tack run --dry-run --output json`, stashes the plan output
- Notify stage: posts a Slack notification with plan summary and a link to the build
- Approve stage: pauses with `input` step, configurable timeout (default 24h)
- Apply stage: unstashes plan output for reference, runs `tack run`

## Notifications

### Slack

Use [`examples/ci/slack-notify.sh`](../examples/ci/slack-notify.sh) to post a formatted plan summary to Slack.

The script reads tack's NDJSON output and sends a message containing:
- Repository, branch, and commit info
- Changed/OK/failed/skipped task counts
- List of planned changes (first 15 tasks)
- Link to the CI run for review and approval

**Setup:**
1. [Create an incoming webhook](https://api.slack.com/messaging/webhooks) in your Slack workspace
2. Set `SLACK_WEBHOOK_URL` as a CI secret
3. Call the script after the plan step:
   ```bash
   ./examples/ci/slack-notify.sh plan-output.json "$CI_RUN_URL"
   ```

### Microsoft Teams

Use [`examples/ci/teams-notify.sh`](../examples/ci/teams-notify.sh) for the same functionality with Microsoft Teams.

**Setup:**
1. [Create an incoming webhook](https://learn.microsoft.com/en-us/microsoftteams/platform/webhooks-and-connectors/how-to/add-incoming-webhook) in your Teams channel
2. Set `TEAMS_WEBHOOK_URL` as a CI secret
3. Call the script after the plan step:
   ```bash
   ./examples/ci/teams-notify.sh plan-output.json "$CI_RUN_URL"
   ```

## JSON Output Format

When using `--output json`, tack emits newline-delimited JSON (NDJSON). Key event types:

**`plan_task`** — emitted for each task during `--dry-run`:
```json
{"type":"plan_task","task":"Install nginx","module":"apt","action":"will_change","params":{"name":"nginx","state":"present"}}
```

**`playbook_recap`** — emitted at the end of a run:
```json
{"type":"playbook_recap","ok":5,"changed":3,"failed":0,"skipped":1,"duration":12.5,"success":true}
```

The notification scripts parse these events to build the summary message.

## Limitations

### Plan Drift

The current plan→approve→apply workflow has an important limitation: **the apply step re-evaluates system state and may produce different changes than the approved plan.**

This can happen when:
- Another process modifies the target system between plan and apply
- A concurrent tack run changes state
- The approval takes a long time and system state drifts

**Mitigations for now:**
- Keep approval windows short
- Avoid concurrent changes to the same targets
- Review the apply output after it runs

**Future improvement (Track 2):** Frozen plan files will allow `tack plan --out plan.tack` to serialize the exact plan, and `tack apply --plan plan.tack` to replay it exactly. This ensures the apply matches what was approved. Track 2 will also add SHA verification and TTL expiry for plan files.
