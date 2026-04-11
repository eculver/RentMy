#!/usr/bin/env bash
#
# Autonomous Coding Agent Runner
# Inspired by: https://github.com/anthropics/claude-quickstarts/tree/main/autonomous-coding
#
# Each task creates its own branch. Prefers Graphite (`gt create`) for stacked
# PRs, but falls back to vanilla git (`git checkout -b`) if gt is unavailable.
#
# Usage:
#   ./scripts/run-agent.sh              # Run until all phases complete
#   ./scripts/run-agent.sh --dry-run    # Show next task without executing
#   ./scripts/run-agent.sh --phase 1    # Run only Phase 1 tasks
#
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
PROGRESS_FILE="$REPO_ROOT/.claude/progress.json"
LOG_DIR="$REPO_ROOT/thoughts/agent-logs"
GT="/opt/homebrew/bin/gt"
MAX_TURNS=200
PAUSE_BETWEEN_SESSIONS=10
USE_GT=false

# Parse flags
DRY_RUN=false
TARGET_PHASE=""
while [[ $# -gt 0 ]]; do
  case $1 in
    --dry-run) DRY_RUN=true; shift ;;
    --phase) TARGET_PHASE="$2"; shift 2 ;;
    *) echo "Unknown option: $1"; exit 1 ;;
  esac
done

# Ensure prerequisites
if [[ ! -f "$PROGRESS_FILE" ]]; then
  echo "ERROR: $PROGRESS_FILE not found. Run Stage A setup first."
  exit 1
fi

if ! python3 -m json.tool "$PROGRESS_FILE" > /dev/null 2>&1; then
  echo "ERROR: $PROGRESS_FILE is not valid JSON."
  exit 1
fi

# Detect Graphite availability (non-fatal — falls back to vanilla git)
if [[ -x "$GT" ]]; then
  # gt binary exists; verify it can operate in this repo
  if "$GT" log --oneline -1 > /dev/null 2>&1; then
    USE_GT=true
  else
    echo "WARNING: Graphite CLI found but repo not initialized. Falling back to vanilla git."
    echo "  To enable: cd $REPO_ROOT && $GT init"
  fi
else
  echo "WARNING: Graphite CLI not found at $GT. Falling back to vanilla git."
  echo "  To install: brew install withgraphite/tap/graphite"
fi

mkdir -p "$LOG_DIR"

# Check if all phases are complete
all_complete() {
  if [[ -n "$TARGET_PHASE" ]]; then
    jq -e ".phases[] | select(.id == $TARGET_PHASE) | .status == \"completed\"" "$PROGRESS_FILE" > /dev/null 2>&1
  else
    jq -e '.phases | all(.status == "completed")' "$PROGRESS_FILE" > /dev/null 2>&1
  fi
}

# Get next task info
next_task_info() {
  python3 -c "
import json, sys

with open('$PROGRESS_FILE') as f:
    data = json.load(f)

for phase in data['phases']:
    if phase['status'] == 'completed':
        continue
    if '$TARGET_PHASE' and phase['id'] != int('${TARGET_PHASE:-0}' or '0'):
        continue
    for task in phase['tasks']:
        if task['status'] == 'pending':
            # Check dependencies
            all_deps_met = True
            for dep in task.get('dependencies', []):
                for p in data['phases']:
                    for t in p['tasks']:
                        if t['id'] == dep and t['status'] != 'completed':
                            all_deps_met = False
            if all_deps_met:
                print(f\"Phase {phase['id']} | Task {task['id']}: {task['name']} ({task.get('domain', 'unknown')})\")
                sys.exit(0)
        elif task['status'] == 'in_progress':
            print(f\"RECOVERY: Phase {phase['id']} | Task {task['id']}: {task['name']} (was in_progress)\")
            sys.exit(0)

print('No pending tasks found.')
sys.exit(1)
"
}

# Main loop
SESSION_COUNT=0

echo "=========================================="
echo "  RentMy Autonomous Coding Agent"
echo "  (Graphite stacked branches)"
echo "=========================================="
echo "Progress file: $PROGRESS_FILE"
echo "Target phase:  ${TARGET_PHASE:-all}"
if $USE_GT; then
  echo "Branching:     Graphite ($GT, $($GT --version))"
else
  echo "Branching:     vanilla git (Graphite unavailable)"
fi
echo ""

while true; do
  # Check completion
  if all_complete; then
    echo ""
    echo "All ${TARGET_PHASE:+Phase $TARGET_PHASE }tasks complete!"
    echo "Total sessions: $SESSION_COUNT"

    # Offer to submit the full stack
    echo ""
    if $USE_GT; then
      echo "To submit all stacked PRs:"
      echo "  $GT submit --stack --no-edit"
    else
      echo "Branches were created with vanilla git."
      echo "Push any remaining branches with: git push -u origin <branch>"
      echo "When Graphite is enabled, import existing branches with: $GT init"
    fi
    exit 0
  fi

  # Show next task
  NEXT_TASK=$(next_task_info 2>&1) || {
    echo "No actionable tasks found. Check progress.json for blocked tasks."
    exit 1
  }

  SESSION_COUNT=$((SESSION_COUNT + 1))
  TIMESTAMP=$(date +%Y%m%d-%H%M%S)
  LOG_FILE="$LOG_DIR/session-${SESSION_COUNT}-${TIMESTAMP}.log"

  echo ""
  echo "--- Session $SESSION_COUNT ---"
  echo "Next: $NEXT_TASK"
  echo "Log:  $LOG_FILE"

  if $DRY_RUN; then
    echo "(dry run — not executing)"
    exit 0
  fi

  # Build the branching instruction for the agent
  if $USE_GT; then
    BRANCH_INSTRUCTION="Use /opt/homebrew/bin/gt for branch management (Graphite mode)."
  else
    BRANCH_INSTRUCTION="Graphite is unavailable — use vanilla git for branching (git checkout -b, git push -u origin)."
  fi

  # Run Claude Code session
  claude --print \
    "Read CLAUDE.md, then follow the Session Workflow to implement the next task. One task only. ${BRANCH_INSTRUCTION}" \
    --max-turns "$MAX_TURNS" \
    --verbose \
    2>&1 | tee "$LOG_FILE"

  EXIT_CODE=${PIPESTATUS[0]}

  echo ""
  echo "Session $SESSION_COUNT finished (exit code: $EXIT_CODE)"

  # Validate progress.json after session
  if ! python3 -m json.tool "$PROGRESS_FILE" > /dev/null 2>&1; then
    echo "WARNING: progress.json is invalid after session. Stopping."
    exit 1
  fi

  # Pause between sessions
  echo "Pausing ${PAUSE_BETWEEN_SESSIONS}s before next session..."
  sleep "$PAUSE_BETWEEN_SESSIONS"
done
