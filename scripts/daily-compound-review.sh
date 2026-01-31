#!/bin/bash
# scripts/daily-compound-review.sh
# Runs BEFORE auto-compound.sh to update AGENTS.md with learnings
# Schedule: 22:30 daily

set -e

PROJECT_DIR="/home/dh/dev/CLIProxyAPI"
LOG_DIR="$PROJECT_DIR/logs"

mkdir -p "$LOG_DIR"

cd "$PROJECT_DIR"

echo "$(date): Starting daily compound review..." >> "$LOG_DIR/compound-review.log"

# Ensure we're on main and up to date
git checkout -B main origin/main
git pull origin main

# Review threads and compound learnings using Amp
amp --execute --dangerously-allow-all "Load the compound-engineering skill if available. Look through and read each Amp thread from the last 24 hours. For any thread where we did NOT compound our learnings at the end, do so now - extract the key learnings from that thread and update the relevant AGENTS.md files so we can learn from our work and mistakes. Focus on:
- Patterns discovered (code conventions, architectural decisions)
- Gotchas and bugs encountered
- Useful context about the codebase
- Commands that work well

Commit your changes with message 'chore: compound daily learnings' and push to main."

echo "$(date): Daily compound review complete." >> "$LOG_DIR/compound-review.log"
