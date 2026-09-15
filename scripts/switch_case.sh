#!/usr/bin/env bash
set -euo pipefail

CASE_ID="${1:?usage: scripts/switch_case.sh CASE-002 [--reset-log]}"
RESET=false
if [[ "${2:-}" == "--reset-log" ]]; then
  RESET=true
fi
URL="${CASE_SERVER_URL:-http://127.0.0.1:18081}"
TOKEN="${DETECTIVE_ADMIN_TOKEN:-dev-admin}"

curl -fsS -X POST "$URL/admin/current" \
  -H 'Content-Type: application/json' \
  -H "X-Admin-Token: $TOKEN" \
  -d "{\"case_id\":\"$CASE_ID\",\"reset_log\":$RESET}"
echo
