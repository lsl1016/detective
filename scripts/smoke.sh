#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

PORT="${SMOKE_PORT:-18082}"
URL="http://127.0.0.1:${PORT}"
TOKEN="smoke-secret"
TMP="$(mktemp -d)"
ORIGINAL_CURRENT="$(cat casepack/current.json)"
SERVER_PID=""

cleanup() {
  printf '%s\n' "$ORIGINAL_CURRENT" > casepack/current.json
  if [[ -n "$SERVER_PID" ]]; then
    kill "$SERVER_PID" >/dev/null 2>&1 || true
    wait "$SERVER_PID" >/dev/null 2>&1 || true
  fi
  rm -rf "$TMP"
}
trap cleanup EXIT

go build -o "$TMP/caseserver" ./caseserver
DETECTIVE_ADDR=":${PORT}" \
DETECTIVE_ADMIN_TOKEN="$TOKEN" \
DETECTIVE_ACCESS_LOG="$TMP/access.jsonl" \
"$TMP/caseserver" >"$TMP/server.log" 2>&1 &
SERVER_PID=$!

for _ in $(seq 1 40); do
  if curl -fsS "$URL/healthz" >/dev/null 2>&1; then
    break
  fi
  sleep 0.25
done
curl -fsS "$URL/healthz" >/dev/null

echo "[1/7] current case"
curl -fsS "$URL/case/current" | grep -q 'CASE-001'

echo "[2/7] current scene is queryable"
curl -fsS -X POST "$URL/tool/scene" -H 'Content-Type: application/json' -d '{"evidence_id":"S1"}' | grep -q '22:17'

echo "[3/7] historical route is absent"
status="$(curl -sS -o /dev/null -w '%{http_code}' "$URL/case/CASE-002")"
[[ "$status" == "404" ]]

echo "[4/7] case_id cannot be smuggled through tool body"
status="$(curl -sS -o /dev/null -w '%{http_code}' -X POST "$URL/tool/scene" -H 'Content-Type: application/json' -d '{"evidence_id":"S1","case_id":"CASE-002"}')"
[[ "$status" == "400" ]]

echo "[5/7] switch to CASE-002"
curl -fsS -X POST "$URL/admin/current" -H 'Content-Type: application/json' -H "X-Admin-Token: $TOKEN" -d '{"case_id":"CASE-002","reset_log":true}' | grep -q 'CASE-002'

echo "[6/7] CASE-001 NPC disappeared and CASE-002 NPC is available"
status="$(curl -sS -o /dev/null -w '%{http_code}' -X POST "$URL/tool/interview" -H 'Content-Type: application/json' -d '{"npc_name":"许岚"}')"
[[ "$status" == "404" ]]
curl -fsS -X POST "$URL/tool/interview" -H 'Content-Type: application/json' -d '{"npc_name":"邵宁"}' | grep -q '设备管理员'

echo "[7/7] evaluator selfcheck"
go run ./evaluator -mode selfcheck >/dev/null

echo "smoke PASS"
