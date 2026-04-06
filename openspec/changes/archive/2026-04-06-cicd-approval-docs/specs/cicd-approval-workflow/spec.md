## ADDED Requirements

### Requirement: CI/CD integration guide documents plan-approve-apply pattern
The `docs/ci-cd.md` guide SHALL explain the two-job CI pattern: a plan job that runs `tack run --dry-run` and captures output, followed by an apply job gated behind human approval. The guide SHALL cover both GitHub Actions and Jenkins. The guide SHALL document the drift limitation and reference Track 2.

#### Scenario: User reads guide and understands the pattern
- **WHEN** a user reads `docs/ci-cd.md`
- **THEN** they understand how to set up a plan→approve→apply pipeline using their CI system's native approval mechanism

#### Scenario: Guide documents drift limitation
- **WHEN** a user reads the limitations section
- **THEN** they understand that apply re-evaluates state and may differ from the approved plan, and that Track 2 (frozen plan files) addresses this

### Requirement: GitHub Actions workflow provides working plan-approve-apply pipeline
The `examples/ci/github-actions/plan-approve-apply.yaml` workflow SHALL define two jobs: `plan` and `apply`. The `plan` job SHALL run `tack run --dry-run --output json`, capture both human-readable and JSON output as artifacts, and post a notification. The `apply` job SHALL require the `production` environment (with required reviewers configured), download the plan artifact, and run `tack run`.

#### Scenario: Plan job runs and uploads artifact
- **WHEN** the workflow triggers on push to main
- **THEN** the `plan` job runs `tack run --dry-run --output json`, uploads plan output as a GitHub Actions artifact, and completes successfully

#### Scenario: Apply job waits for environment approval
- **WHEN** the `plan` job completes
- **THEN** the `apply` job is blocked until a required reviewer approves the deployment in the GitHub UI

#### Scenario: Apply job runs after approval
- **WHEN** a reviewer approves the environment deployment
- **THEN** the `apply` job downloads the plan artifact (for reference) and runs `tack run`

### Requirement: Jenkins pipeline provides working plan-approve-apply pipeline
The `examples/ci/jenkins/Jenkinsfile` SHALL define stages for plan, notification, approval (`input` step), and apply. The plan stage SHALL run `tack run --dry-run --output json`. The approval stage SHALL use Jenkins `input` step to pause for human approval.

#### Scenario: Pipeline pauses at input step
- **WHEN** the plan stage completes
- **THEN** the pipeline pauses at the `input` step until a user approves or rejects in the Jenkins UI

#### Scenario: Pipeline continues after approval
- **WHEN** a user approves the input step
- **THEN** the apply stage runs `tack run`

### Requirement: Slack notification script posts plan summary
The `examples/ci/slack-notify.sh` script SHALL read tack's JSON plan output and post a formatted message to a Slack channel via incoming webhook. The message SHALL include: repository/branch/commit info, changed/unchanged/failed task counts, a truncated list of planned changes, and a link to the CI run for approval. The script SHALL read the webhook URL from the `SLACK_WEBHOOK_URL` environment variable.

#### Scenario: Script posts plan summary to Slack
- **WHEN** the script is invoked with a path to tack's JSON plan output and the CI run URL
- **THEN** it posts a formatted Slack message with plan summary and approval link

#### Scenario: Script fails gracefully without webhook URL
- **WHEN** `SLACK_WEBHOOK_URL` is not set
- **THEN** the script exits with a clear error message

### Requirement: Teams notification script posts plan summary
The `examples/ci/teams-notify.sh` script SHALL read tack's JSON plan output and post a formatted message to a Microsoft Teams channel via incoming webhook. The message SHALL include the same information as the Slack script. The script SHALL read the webhook URL from the `TEAMS_WEBHOOK_URL` environment variable.

#### Scenario: Script posts plan summary to Teams
- **WHEN** the script is invoked with a path to tack's JSON plan output and the CI run URL
- **THEN** it posts a formatted Teams message with plan summary and approval link

#### Scenario: Script fails gracefully without webhook URL
- **WHEN** `TEAMS_WEBHOOK_URL` is not set
- **THEN** the script exits with a clear error message
