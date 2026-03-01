#!/usr/bin/env bash

set -euo pipefail

TEST_ENV="${TEST_ENV:-compose}"

COLLECTOR_URL="${COLLECTOR_URL:-http://localhost:3000}"
QUERY_URL="${QUERY_URL:-http://localhost:3002}"

POSTGRES_CONTAINER="${POSTGRES_CONTAINER:-postgres}"
POSTGRES_USER="${POSTGRES_USER:-postgres}"
POSTGRES_DB="${POSTGRES_DB:-analytics}"

REDIS_HOST="${REDIS_HOST:-localhost}"
REDIS_PORT="${REDIS_PORT:-6379}"
REDIS_CONTAINER="${REDIS_CONTAINER:-redis}"
STRICT_REDIS_CHECK="${STRICT_REDIS_CHECK:-true}"

MINIO_CONTAINER="${MINIO_CONTAINER:-minio}"
MINIO_ENDPOINT="${MINIO_ENDPOINT:-http://127.0.0.1:9000}"
MINIO_ACCESS_KEY="${MINIO_ACCESS_KEY:-minioadmin}"
MINIO_SECRET_KEY="${MINIO_SECRET_KEY:-minioadmin}"
MINIO_BUCKET="${MINIO_BUCKET:-event-archive}"

K8S_NAMESPACE="${K8S_NAMESPACE:-event-pipeline}"

FRAUD_THRESHOLD="${FRAUD_THRESHOLD:-100}"
default_hot_ip() {
  local seed
  seed="$(date +%s)"
  local octet_a octet_b
  octet_a=$((seed % 250 + 1))
  octet_b=$(((seed / 251) % 250 + 1))
  printf "198.51.%d.%d" "$octet_a" "$octet_b"
}

HOT_IP="${HOT_IP:-$(default_hot_ip)}"
SAFE_IP="${SAFE_IP:-203.0.113.10}"

RUN_ID="$(date +%s)"
CAMPAIGN_ID="camp-test-${RUN_ID}"
PROBE_CAMPAIGN_ID="camp-probe-${RUN_ID}"
USER_PREFIX="user-test-${RUN_ID}"

TOTAL_HOT_REQUESTS=$((FRAUD_THRESHOLD + 1))
EXPECTED_DB_ROWS=$((1 + 1 + FRAUD_THRESHOLD))
HOT_KEY="fraud:ip:${HOT_IP}"

GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

log() {
  printf "%b\n" "${GREEN}[test]${NC} $*"
}

warn() {
  printf "%b\n" "${YELLOW}[warn]${NC} $*"
}

fail() {
  printf "%b\n" "${RED}[fail]${NC} $*"
  exit 1
}

is_true() {
  [[ "${1,,}" == "true" || "${1}" == "1" || "${1,,}" == "yes" ]]
}

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || fail "required command not found: $1"
}

usage() {
  cat <<EOF
Usage: ./test.sh [flags]

Flags:
  --mode <compose|k8s>        Test environment mode
  --collector-url <url>       Collector base URL
  --query-url <url>           Query-service base URL
  -h, --help                  Show this help

Examples:
  ./test.sh
  ./test.sh --mode k8s --collector-url http://collector.local --query-url http://query.local
  ./test.sh --mode k8s --collector-url http://localhost:3000 --query-url http://localhost:3002
EOF
}

parse_args() {
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --mode)
        TEST_ENV="$2"
        shift 2
        ;;
      --collector-url)
        COLLECTOR_URL="$2"
        shift 2
        ;;
      --query-url)
        QUERY_URL="$2"
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

is_k8s() {
  [[ "$TEST_ENV" == "k8s" ]]
}

k8s_first_pod() {
  local label="$1"
  kubectl -n "$K8S_NAMESPACE" get pods -l "$label" -o jsonpath='{.items[0].metadata.name}'
}

redis_exec() {
  if is_k8s; then
    local redis_pod
    redis_pod="$(k8s_first_pod app=redis)"
    [[ -n "$redis_pod" ]] || return 1
    kubectl -n "$K8S_NAMESPACE" exec "$redis_pod" -- redis-cli "$@"
    return 0
  fi

  if command -v redis-cli >/dev/null 2>&1; then
    redis-cli -h "$REDIS_HOST" -p "$REDIS_PORT" "$@"
    return 0
  fi

  if docker ps --format '{{.Names}}' | grep -q "^${REDIS_CONTAINER}$"; then
    docker exec "$REDIS_CONTAINER" redis-cli "$@"
    return 0
  fi

  return 1
}

redis_del_key() {
  local key="$1"
  redis_exec DEL "$key" >/dev/null
}

redis_get_key() {
  local key="$1"
  redis_exec GET "$key"
}

db_scalar() {
  local sql="$1"
  if is_k8s; then
    local postgres_pod
    postgres_pod="$(k8s_first_pod app=postgres)"
    [[ -n "$postgres_pod" ]] || return 1
    kubectl -n "$K8S_NAMESPACE" exec "$postgres_pod" -- psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -tAc "$sql" | tr -d '[:space:]'
    return 0
  fi

  docker exec -i "$POSTGRES_CONTAINER" psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -tAc "$sql" | tr -d '[:space:]'
}

send_event_with_campaign() {
  local event_id="$1"
  local ip="$2"
  local user_id="$3"
  local event_type="$4"
  local campaign_id="$5"

  local payload
  payload=$(cat <<EOF
{"event_id":"${event_id}","event_type":"${event_type}","user_id":"${user_id}","campaign_id":"${campaign_id}","geo_country":"US","device_type":"mobile"}
EOF
)

  local status
  status=$(curl -s -o /tmp/event_pipeline_resp.txt -w "%{http_code}" \
    -X POST "${COLLECTOR_URL}/event" \
    -H "Content-Type: application/json" \
    -H "X-Forwarded-For: ${ip}" \
    -d "$payload")

  if [[ "$status" != "202" ]]; then
    fail "collector returned HTTP ${status} for event_id=${event_id}, body=$(cat /tmp/event_pipeline_resp.txt)"
  fi
}

send_event() {
  send_event_with_campaign "$1" "$2" "$3" "$4" "$CAMPAIGN_ID"
}

minio_count_prefix() {
  local prefix="$1"
  if is_k8s; then
    fail "minio_count_prefix is not supported in k8s mode"
  fi

  docker exec "$MINIO_CONTAINER" sh -c "mc alias set local ${MINIO_ENDPOINT} ${MINIO_ACCESS_KEY} ${MINIO_SECRET_KEY} >/dev/null 2>&1 && mc find local/${MINIO_BUCKET}/${prefix} --name '*.ndjson.gz' 2>/dev/null | wc -l" | tr -d '[:space:]'
}

wait_for_db_rows() {
  local expected="$1"
  local attempts=90

  for ((i=1; i<=attempts; i++)); do
    local current
    current=$(db_scalar "SELECT COUNT(*) FROM raw_events WHERE campaign_id='${CAMPAIGN_ID}';")
    if [[ "$current" == "$expected" ]]; then
      log "DB row target reached: ${current}/${expected}"
      return 0
    fi
    sleep 2
  done

  local final
  final=$(db_scalar "SELECT COUNT(*) FROM raw_events WHERE campaign_id='${CAMPAIGN_ID}';")
  fail "timed out waiting DB rows. expected=${expected}, got=${final}"
}

main() {
  parse_args "$@"

  [[ "$TEST_ENV" == "compose" || "$TEST_ENV" == "k8s" ]] || fail "invalid --mode: $TEST_ENV (use compose or k8s)"

  require_cmd curl
  if is_k8s; then
    require_cmd kubectl
    kubectl get namespace "$K8S_NAMESPACE" >/dev/null 2>&1 || fail "k8s namespace not found: $K8S_NAMESPACE"
  else
    require_cmd docker
  fi

  log "Run ID: ${RUN_ID}"
  log "Campaign ID: ${CAMPAIGN_ID}"
  log "Expecting ${EXPECTED_DB_ROWS} persisted rows"

  log "Preflight: collector health"
  curl -fsS "${COLLECTOR_URL}/health" >/dev/null || fail "collector health check failed at ${COLLECTOR_URL}/health"

  log "Preflight: query-service health"
  curl -fsS "${QUERY_URL}/health" >/dev/null || fail "query-service health check failed at ${QUERY_URL}/health"

  log "Resetting hot IP counter in Redis: ${HOT_KEY}"
  if ! redis_del_key "$HOT_KEY"; then
    if is_true "$STRICT_REDIS_CHECK"; then
      fail "could not reset Redis key ${HOT_KEY}; Redis check is required"
    fi
    warn "could not reset Redis key ${HOT_KEY}; hot IP counter may include prior data"
  fi

  log "Preflight: worker end-to-end processing probe"
  local probe_before probe_after
  probe_before=$(db_scalar "SELECT COUNT(*) FROM raw_events WHERE campaign_id='${PROBE_CAMPAIGN_ID}';")
  send_event_with_campaign "evt-${RUN_ID}-probe-1" "$SAFE_IP" "${USER_PREFIX}-probe" "impression" "$PROBE_CAMPAIGN_ID"

  local probe_ok="false"
  for _ in $(seq 1 15); do
    probe_after=$(db_scalar "SELECT COUNT(*) FROM raw_events WHERE campaign_id='${PROBE_CAMPAIGN_ID}';")
    if [[ "$probe_after" == $((probe_before + 1)) ]]; then
      probe_ok="true"
      break
    fi
    sleep 2
  done

  if [[ "$probe_ok" != "true" ]]; then
    fail "worker probe event was not persisted. Check worker logs (fraud endpoint / Redis / DB) before running full test"
  fi

  local minio_enabled="false"
  local raw_before="0"
  local dup_before="0"
  local fraud_before="0"
  local accepted_before="0"

  if ! is_k8s && docker ps --format '{{.Names}}' | grep -q "^${MINIO_CONTAINER}$"; then
    minio_enabled="true"
    log "Capturing MinIO prefix counts before test"
    raw_before=$(minio_count_prefix "raw")
    dup_before=$(minio_count_prefix "duplicate")
    fraud_before=$(minio_count_prefix "fraud")
    accepted_before=$(minio_count_prefix "accepted")
  elif is_k8s; then
    warn "minio object checks are skipped in k8s mode"
  else
    warn "minio container not running; archive prefix assertions skipped"
  fi

  log "Case 1: accepted path (unique event, safe IP)"
  send_event "evt-${RUN_ID}-accepted-1" "$SAFE_IP" "${USER_PREFIX}-accepted" "impression"

  log "Case 2: duplicate path (same event_id twice)"
  send_event "evt-${RUN_ID}-dup-1" "$SAFE_IP" "${USER_PREFIX}-dup" "click"
  send_event "evt-${RUN_ID}-dup-1" "$SAFE_IP" "${USER_PREFIX}-dup" "click"

  log "Case 3: fraud threshold path (${TOTAL_HOT_REQUESTS} events from ${HOT_IP})"
  for i in $(seq 1 "$TOTAL_HOT_REQUESTS"); do
    send_event "evt-${RUN_ID}-hot-${i}" "$HOT_IP" "${USER_PREFIX}-hot-${i}" "impression"
  done

  log "Waiting for worker to process and persist"
  wait_for_db_rows "$EXPECTED_DB_ROWS"

  log "Waiting for archive flush window"
  sleep 8

  local db_total hot_persisted risk_gt80_count
  db_total=$(db_scalar "SELECT COUNT(*) FROM raw_events WHERE campaign_id='${CAMPAIGN_ID}';")
  hot_persisted=$(db_scalar "SELECT COUNT(*) FROM raw_events WHERE campaign_id='${CAMPAIGN_ID}' AND ip_address='${HOT_IP}';")
  risk_gt80_count=$(db_scalar "SELECT COUNT(*) FROM raw_events WHERE campaign_id='${CAMPAIGN_ID}' AND risk_score > 80;")

  [[ "$db_total" == "$EXPECTED_DB_ROWS" ]] || fail "unexpected DB row count: expected=${EXPECTED_DB_ROWS}, got=${db_total}"
  [[ "$hot_persisted" == "$FRAUD_THRESHOLD" ]] || fail "unexpected persisted count for hot IP: expected=${FRAUD_THRESHOLD}, got=${hot_persisted}"
  [[ "$risk_gt80_count" == "0" ]] || fail "found persisted rows with risk_score > 80 (should be 0): got=${risk_gt80_count}"

  log "DB assertions passed"

  log "Validating query-service endpoints"
  curl -fsS "${QUERY_URL}/campaigns" >/tmp/query_campaigns_resp.txt || fail "query-service /campaigns request failed"
  grep -q '"data"' /tmp/query_campaigns_resp.txt || fail "query-service /campaigns response missing data field"

  local query_status
  query_status=$(curl -s -o /tmp/query_campaign_resp.txt -w "%{http_code}" "${QUERY_URL}/campaigns/${CAMPAIGN_ID}")
  [[ "$query_status" == "200" ]] || fail "query-service /campaigns/${CAMPAIGN_ID} returned HTTP ${query_status}, body=$(cat /tmp/query_campaign_resp.txt)"
  grep -q "${CAMPAIGN_ID}" /tmp/query_campaign_resp.txt || fail "query-service /campaigns/${CAMPAIGN_ID} response missing campaign id"

  {
    local hot_count
    hot_count="$(redis_get_key "$HOT_KEY" 2>/dev/null | tr -d '[:space:]')"
    if [[ -z "$hot_count" ]]; then
      if is_true "$STRICT_REDIS_CHECK"; then
        fail "hot key ${HOT_KEY} not found in Redis"
      fi
      warn "hot key ${HOT_KEY} not found in Redis (may have expired)"
    else
      if [[ "$hot_count" != "$TOTAL_HOT_REQUESTS" ]]; then
        if is_true "$STRICT_REDIS_CHECK"; then
          fail "hot key count mismatch expected=${TOTAL_HOT_REQUESTS}, got=${hot_count}"
        fi
        warn "hot key count expected=${TOTAL_HOT_REQUESTS}, got=${hot_count}"
      fi
      log "Redis hot-key value: ${HOT_KEY}=${hot_count}"
    fi
  } || {
    if is_true "$STRICT_REDIS_CHECK"; then
      fail "could not read hot key ${HOT_KEY} from Redis"
    fi
    warn "could not read hot key ${HOT_KEY} from Redis"
  }

  if [[ "$minio_enabled" == "true" ]]; then
    local raw_after dup_after fraud_after accepted_after
    raw_after=$(minio_count_prefix "raw")
    dup_after=$(minio_count_prefix "duplicate")
    fraud_after=$(minio_count_prefix "fraud")
    accepted_after=$(minio_count_prefix "accepted")

    local raw_delta dup_delta fraud_delta accepted_delta
    raw_delta=$((raw_after - raw_before))
    dup_delta=$((dup_after - dup_before))
    fraud_delta=$((fraud_after - fraud_before))
    accepted_delta=$((accepted_after - accepted_before))

    [[ "$raw_delta" -gt 0 ]] || fail "MinIO raw/ prefix did not increase"
    [[ "$dup_delta" -gt 0 ]] || fail "MinIO duplicate/ prefix did not increase"
    [[ "$fraud_delta" -gt 0 ]] || fail "MinIO fraud/ prefix did not increase"
    [[ "$accepted_delta" -gt 0 ]] || fail "MinIO accepted/ prefix did not increase"

    log "MinIO assertions passed (delta): raw=${raw_delta}, duplicate=${dup_delta}, fraud=${fraud_delta}, accepted=${accepted_delta}"
  fi

  log "All checks passed"
  cat <<EOF

Results:
- campaign_id: ${CAMPAIGN_ID}
- expected persisted rows: ${EXPECTED_DB_ROWS}
- actual persisted rows: ${db_total}
- hot IP persisted rows (should be ${FRAUD_THRESHOLD}): ${hot_persisted}
- persisted rows with risk_score > 80 (should be 0): ${risk_gt80_count}

EOF
}

main "$@"
