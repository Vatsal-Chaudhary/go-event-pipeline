#!/usr/bin/env bash

set -euo pipefail

COLLECTOR_URL="${COLLECTOR_URL:-http://localhost:3000}"
FRAUD_THRESHOLD="${FRAUD_THRESHOLD:-100}"
SAFE_IP="${SAFE_IP:-203.0.113.10}"
HOT_IP="${HOT_IP:-198.51.100.77}"

RUN_ID="$(date +%s)"
CAMPAIGN_ID="${CAMPAIGN_ID:-camp-video-${RUN_ID}}"
USER_PREFIX="user-video-${RUN_ID}"
TOTAL_HOT_REQUESTS=$((FRAUD_THRESHOLD + 1))

usage() {
  cat <<EOF
Usage: ./test_ingest.sh [flags]

Flags:
  --collector-url <url>    Collector base URL (default: http://localhost:3000)
  --campaign-id <id>       Campaign ID to use (default: generated)
  --fraud-threshold <n>    Threshold used by worker (default: 100)
  --safe-ip <ip>           Safe IP for accepted/duplicate path
  --hot-ip <ip>            Hot IP for fraud path
  -h, --help               Show help

Example:
  ./test_ingest.sh --collector-url http://localhost:3000
EOF
}

log() {
  printf "[ingest] %s\n" "$*"
}

fail() {
  printf "[ingest:fail] %s\n" "$*" >&2
  exit 1
}

parse_args() {
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --collector-url)
        COLLECTOR_URL="$2"
        shift 2
        ;;
      --campaign-id)
        CAMPAIGN_ID="$2"
        shift 2
        ;;
      --fraud-threshold)
        FRAUD_THRESHOLD="$2"
        shift 2
        ;;
      --safe-ip)
        SAFE_IP="$2"
        shift 2
        ;;
      --hot-ip)
        HOT_IP="$2"
        shift 2
        ;;
      -h|--help)
        usage
        exit 0
        ;;
      *)
        fail "unknown argument: $1"
        ;;
    esac
  done
}

send_event() {
  local event_id="$1"
  local ip="$2"
  local user_id="$3"
  local event_type="$4"

  local payload
  payload=$(cat <<EOF
{"event_id":"${event_id}","event_type":"${event_type}","user_id":"${user_id}","campaign_id":"${CAMPAIGN_ID}","geo_country":"US","device_type":"mobile"}
EOF
)

  local status
  status=$(curl -s -o /tmp/event_pipeline_ingest_resp.txt -w "%{http_code}" \
    -X POST "${COLLECTOR_URL}/event" \
    -H "Content-Type: application/json" \
    -H "X-Forwarded-For: ${ip}" \
    -d "$payload")

  [[ "$status" == "202" ]] || fail "collector returned ${status} for ${event_id}, body=$(cat /tmp/event_pipeline_ingest_resp.txt)"
}

main() {
  parse_args "$@"
  TOTAL_HOT_REQUESTS=$((FRAUD_THRESHOLD + 1))

  command -v curl >/dev/null 2>&1 || fail "curl not found"

  log "collector health check"
  curl -fsS "${COLLECTOR_URL}/health" >/dev/null || fail "collector health check failed"

  log "campaign_id=${CAMPAIGN_ID}"
  log "case 1: accepted path"
  send_event "evt-${RUN_ID}-accepted-1" "$SAFE_IP" "${USER_PREFIX}-accepted" "impression"

  log "case 2: duplicate path (same event_id twice)"
  send_event "evt-${RUN_ID}-dup-1" "$SAFE_IP" "${USER_PREFIX}-dup" "click"
  send_event "evt-${RUN_ID}-dup-1" "$SAFE_IP" "${USER_PREFIX}-dup" "click"

  log "case 3: fraud path (${TOTAL_HOT_REQUESTS} events from ${HOT_IP})"
  for i in $(seq 1 "$TOTAL_HOT_REQUESTS"); do
    send_event "evt-${RUN_ID}-hot-${i}" "$HOT_IP" "${USER_PREFIX}-hot-${i}" "impression"
  done

  printf "%s\n" "${CAMPAIGN_ID}" > /tmp/event_pipeline_last_campaign_id.txt

  cat <<EOF

Ingest complete.
- campaign_id: ${CAMPAIGN_ID}
- safe_ip: ${SAFE_IP}
- hot_ip: ${HOT_IP}
- hot_ip_requests: ${TOTAL_HOT_REQUESTS}

Saved campaign_id to: /tmp/event_pipeline_last_campaign_id.txt
Use this for retrieval test.
EOF
}

main "$@"
