#!/usr/bin/env bash
# Post a tack plan summary to Microsoft Teams via incoming webhook.
#
# Usage:
#   TEAMS_WEBHOOK_URL=https://... ./teams-notify.sh plan-output.json [run-url]
#
# Arguments:
#   $1 - Path to tack's NDJSON plan output (from --output json)
#   $2 - URL to the CI run for review/approval (optional)
#
# Environment:
#   TEAMS_WEBHOOK_URL - Teams incoming webhook URL (required)
#   REPO_NAME         - Repository name override (default: git remote origin)
#   BRANCH            - Branch name override (default: current git branch)
#   COMMIT_SHA        - Commit SHA override (default: current HEAD)

set -euo pipefail

PLAN_FILE="${1:?Usage: teams-notify.sh <plan-output.json> [run-url]}"
RUN_URL="${2:-}"

if [[ -z "${TEAMS_WEBHOOK_URL:-}" ]]; then
    echo "Error: TEAMS_WEBHOOK_URL is not set" >&2
    exit 1
fi

if [[ ! -f "$PLAN_FILE" ]]; then
    echo "Error: Plan file not found: $PLAN_FILE" >&2
    exit 1
fi

# Extract repo info (with overrides for CI environments)
REPO="${REPO_NAME:-$(git remote get-url origin 2>/dev/null | sed 's|.*[:/]\(.*\)\.git$|\1|' || echo 'unknown')}"
BRANCH="${BRANCH:-$(git rev-parse --abbrev-ref HEAD 2>/dev/null || echo 'unknown')}"
COMMIT="${COMMIT_SHA:-$(git rev-parse --short HEAD 2>/dev/null || echo 'unknown')}"

# Parse recap from NDJSON (last playbook_recap event)
RECAP=$(grep '"playbook_recap"' "$PLAN_FILE" | tail -1)
if [[ -n "$RECAP" ]]; then
    OK=$(echo "$RECAP" | python3 -c "import sys,json; print(json.loads(sys.stdin.read()).get('ok',0))" 2>/dev/null || echo "0")
    CHANGED=$(echo "$RECAP" | python3 -c "import sys,json; print(json.loads(sys.stdin.read()).get('changed',0))" 2>/dev/null || echo "0")
    FAILED=$(echo "$RECAP" | python3 -c "import sys,json; print(json.loads(sys.stdin.read()).get('failed',0))" 2>/dev/null || echo "0")
    SKIPPED=$(echo "$RECAP" | python3 -c "import sys,json; print(json.loads(sys.stdin.read()).get('skipped',0))" 2>/dev/null || echo "0")
else
    OK=0; CHANGED=0; FAILED=0; SKIPPED=0
fi

# Build task list from plan_task events (first 15)
TASKS=$(grep '"plan_task"' "$PLAN_FILE" | head -15 | python3 -c "
import sys, json
for line in sys.stdin:
    event = json.loads(line.strip())
    action = event.get('action', '')
    icon = {'will_change': '+', 'no_change': '=', 'will_skip': '-', 'always_runs': '~', 'conditional': '?'}.get(action, ' ')
    print(f'{icon} {event.get(\"task\", \"unnamed\")} ({event.get(\"module\", \"\")})')
" 2>/dev/null || echo "(could not parse tasks)")

TASK_COUNT=$(grep -c '"plan_task"' "$PLAN_FILE" 2>/dev/null || echo "0")
if [[ "$TASK_COUNT" -gt 15 ]]; then
    TASKS="${TASKS}
... and $((TASK_COUNT - 15)) more"
fi

# Determine status
if [[ "$FAILED" -gt 0 ]]; then
    STATUS="Plan completed with failures"
    COLOR="attention"
elif [[ "$CHANGED" -gt 0 ]]; then
    STATUS="Plan ready for review"
    COLOR="warning"
else
    STATUS="No changes detected"
    COLOR="good"
fi

# Escape for JSON
TASKS_ESCAPED=$(echo "$TASKS" | python3 -c "import sys,json; print(json.dumps(sys.stdin.read()))" 2>/dev/null | sed 's/^"//;s/"$//')

# Build action block if URL provided
ACTION_BLOCK=""
if [[ -n "$RUN_URL" ]]; then
    ACTION_BLOCK=',{
        "type": "ActionSet",
        "actions": [{
            "type": "Action.OpenUrl",
            "title": "View Run & Approve",
            "url": "'"$RUN_URL"'"
        }]
    }'
fi

# Build Adaptive Card payload
PAYLOAD=$(cat <<EOF
{
  "type": "message",
  "attachments": [{
    "contentType": "application/vnd.microsoft.card.adaptive",
    "content": {
      "\$schema": "http://adaptivecards.io/schemas/adaptive-card.json",
      "type": "AdaptiveCard",
      "version": "1.4",
      "body": [
        {
          "type": "TextBlock",
          "text": "$STATUS",
          "weight": "Bolder",
          "size": "Medium",
          "style": "heading"
        },
        {
          "type": "FactSet",
          "facts": [
            {"title": "Repo", "value": "$REPO"},
            {"title": "Branch", "value": "$BRANCH"},
            {"title": "Commit", "value": "$COMMIT"},
            {"title": "Changed", "value": "$CHANGED"},
            {"title": "OK", "value": "$OK"},
            {"title": "Failed", "value": "$FAILED"},
            {"title": "Skipped", "value": "$SKIPPED"}
          ]
        },
        {
          "type": "TextBlock",
          "text": "Planned tasks:",
          "weight": "Bolder"
        },
        {
          "type": "TextBlock",
          "text": "$TASKS_ESCAPED",
          "fontType": "Monospace",
          "wrap": true
        }$ACTION_BLOCK
      ]
    }
  }]
}
EOF
)

curl -fsSL -X POST -H 'Content-Type: application/json' -d "$PAYLOAD" "$TEAMS_WEBHOOK_URL"
echo ""
echo "Teams notification sent."
