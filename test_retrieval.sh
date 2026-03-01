#!/usr/bin/env bash

set -euo pipefail

QUERY_URL="${QUERY_URL:-http://localhost:3002}"
CAMPAIGN_ID="${CAMPAIGN_ID:-}"

usage() {
  cat <<EOF
Usage: ./test_retrieval.sh [flags]

Flags:
  --query-url <url>       Query-service base URL (default: http://localhost:3002)
  --campaign-id <id>      Campaign ID to fetch (default: read from /tmp/event_pipeline_last_campaign_id.txt)
  -h, --help              Show help

Example:
  ./test_retrieval.sh --query-url http://localhost:3002 --campaign-id camp-video-123456789
EOF
}

log() {
  printf "[retrieval] %s\n" "$*"
}

fail() {
  printf "[retrieval:fail] %s\n" "$*" >&2
  exit 1
}

parse_args() {
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --query-url)
        QUERY_URL="$2"
        shift 2
        ;;
      --campaign-id)
        CAMPAIGN_ID="$2"
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

resolve_campaign_id() {
  if [[ -n "$CAMPAIGN_ID" ]]; then
    return
  fi

  if [[ -f /tmp/event_pipeline_last_campaign_id.txt ]]; then
    CAMPAIGN_ID="$(tr -d '[:space:]' < /tmp/event_pipeline_last_campaign_id.txt)"
  fi

  [[ -n "$CAMPAIGN_ID" ]] || fail "campaign id missing. Pass --campaign-id or run ./test_ingest.sh first"
}

main() {
  parse_args "$@"
  resolve_campaign_id

  command -v curl >/dev/null 2>&1 || fail "curl not found"

  log "query-service health check"
  curl -fsS "${QUERY_URL}/health" >/dev/null || fail "query-service health check failed"

  log "GET /campaigns"
  curl -fsS "${QUERY_URL}/campaigns" | tee /tmp/event_pipeline_campaigns_all.json

  log "GET /campaigns/${CAMPAIGN_ID}"
  local status
  status=$(curl -s -o /tmp/event_pipeline_campaign_single.json -w "%{http_code}" "${QUERY_URL}/campaigns/${CAMPAIGN_ID}")
  [[ "$status" == "200" ]] || fail "query-service returned ${status} for campaign ${CAMPAIGN_ID}"
  cat /tmp/event_pipeline_campaign_single.json

  cat <<EOF

Retrieval complete.
- campaign_id: ${CAMPAIGN_ID}
- all campaigns response saved: /tmp/event_pipeline_campaigns_all.json
- single campaign response saved: /tmp/event_pipeline_campaign_single.json
EOF
}

main "$@"
