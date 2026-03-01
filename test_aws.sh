#!/usr/bin/env bash

set -euo pipefail

COLLECTOR_URL="${COLLECTOR_URL:-http://localhost:3000}"
QUERY_URL="${QUERY_URL:-http://localhost:3002}"

K8S_NAMESPACE="${K8S_NAMESPACE:-event-pipeline}"

AWS_REGION="${AWS_REGION:-ap-south-1}"
ARCHIVE_BACKEND="${ARCHIVE_BACKEND:-auto}"
S3_BUCKET="${S3_BUCKET:-}"

REDIS_ADDR="${REDIS_ADDR:-}"
DB_URL="${DB_URL:-}"

FRAUD_THRESHOLD="${FRAUD_THRESHOLD:-}"
SAFE_IP="${SAFE_IP:-203.0.113.10}"
HOT_IP="${HOT_IP:-198.51.100.77}"

STRICT_REDIS_CHECK="${STRICT_REDIS_CHECK:-false}"
STRICT_S3_CHECK="${STRICT_S3_CHECK:-false}"
STRICT_DB_CHECK="${STRICT_DB_CHECK:-false}"

RUN_ID="$(date +%s)"
CAMPAIGN_ID="${CAMPAIGN_ID:-camp-aws-test-${RUN_ID}}"
PROBE_CAMPAIGN_ID="camp-aws-probe-${RUN_ID}"
USER_PREFIX="user-aws-test-${RUN_ID}"

TOTAL_HOT_REQUESTS=0
EXPECTED_DB_ROWS=0
HOT_KEY=""

usage() {
  cat <<EOF
Usage: ./test_aws.sh [flags]

Flags:
  --collector-url <url>         Collector URL
  --query-url <url>             Query URL
  --namespace <ns>              Kubernetes namespace (default: event-pipeline)
  --campaign-id <id>            Campaign ID (default: generated)
  --fraud-threshold <n>         Fraud threshold (auto from configmap if available)
  --safe-ip <ip>                Safe IP for accepted and duplicate test
  --hot-ip <ip>                 Hot IP for fraud test
  --archive-backend <auto|s3|minio|none>
  --s3-bucket <name>            S3 bucket for archive checks
  --aws-region <region>         AWS region for S3 checks
  --redis-addr <host:port>      Redis endpoint (auto from configmap if available)
  --db-url <dsn>                DB URL for direct SQL assertions
  --strict-redis-check <bool>   Fail if Redis check cannot run
  --strict-s3-check <bool>      Fail if S3 check cannot run
  --strict-db-check <bool>      Fail if DB check cannot run
  -h, --help                    Show help

Examples:
  ./test_aws.sh --collector-url https://collector.example.com --query-url https://query.example.com
  ./test_aws.sh --collector-url http://localhost:3000 --query-url http://localhost:3002 --strict-s3-check true
EOF
}

log() {
  printf "[aws-test] %s\n" "$*"
}

warn() {
  printf "[aws-test:warn] %s\n" "$*"
}

fail() {
  printf "[aws-test:fail] %s\n" "$*" >&2
  exit 1
}

is_true() {
  [[ "${1,,}" == "true" || "$1" == "1" || "${1,,}" == "yes" ]]
}

parse_args() {
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --collector-url)
        COLLECTOR_URL="$2"
        shift 2
        ;;
      --query-url)
        QUERY_URL="$2"
        shift 2
        ;;
      --namespace)
        K8S_NAMESPACE="$2"
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
      --archive-backend)
        ARCHIVE_BACKEND="$2"
        shift 2
        ;;
      --s3-bucket)
        S3_BUCKET="$2"
        shift 2
        ;;
      --aws-region)
        AWS_REGION="$2"
        shift 2
        ;;
      --redis-addr)
        REDIS_ADDR="$2"
        shift 2
        ;;
      --db-url)
        DB_URL="$2"
        shift 2
        ;;
      --strict-redis-check)
        STRICT_REDIS_CHECK="$2"
        shift 2
        ;;
      --strict-s3-check)
        STRICT_S3_CHECK="$2"
        shift 2
        ;;
      --strict-db-check)
        STRICT_DB_CHECK="$2"
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

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || fail "required command not found: $1"
}

k8s_available() {
  command -v kubectl >/dev/null 2>&1 && kubectl get namespace "$K8S_NAMESPACE" >/dev/null 2>&1
}

k8s_cm_value() {
  local key="$1"
  kubectl -n "$K8S_NAMESPACE" get configmap event-pipeline-config -o "jsonpath={.data.${key}}" 2>/dev/null || true
}

k8s_secret_value() {
  local key="$1"
  local encoded
  encoded=$(kubectl -n "$K8S_NAMESPACE" get secret event-pipeline-secrets -o "jsonpath={.data.${key}}" 2>/dev/null || true)
  if [[ -n "$encoded" ]]; then
    printf "%s" "$encoded" | base64 -d
  fi
}

discover_runtime_config() {
  if ! k8s_available; then
    warn "kubectl namespace access not available, using provided flags/env only"
    return
  fi

  local cm_backend cm_bucket cm_redis cm_threshold
  cm_backend="$(k8s_cm_value WORKER_ARCHIVE_BACKEND)"
  cm_bucket="$(k8s_cm_value S3_BUCKET)"
  cm_redis="$(k8s_cm_value REDIS_ADDR)"
  cm_threshold="$(k8s_cm_value WORKER_FRAUD_IP_THRESHOLD)"

  if [[ "$ARCHIVE_BACKEND" == "auto" && -n "$cm_backend" ]]; then
    ARCHIVE_BACKEND="$cm_backend"
  fi

  if [[ -z "$S3_BUCKET" && -n "$cm_bucket" ]]; then
    S3_BUCKET="$cm_bucket"
  fi

  if [[ -z "$REDIS_ADDR" && -n "$cm_redis" ]]; then
    REDIS_ADDR="$cm_redis"
  fi

  if [[ -z "$FRAUD_THRESHOLD" && -n "$cm_threshold" ]]; then
    FRAUD_THRESHOLD="$cm_threshold"
  fi

  if [[ -z "$DB_URL" ]]; then
    DB_URL="$(k8s_secret_value DB_URL || true)"
  fi
}

normalize_runtime_defaults() {
  if [[ -z "$FRAUD_THRESHOLD" ]]; then
    FRAUD_THRESHOLD="100"
  fi
  if [[ "$ARCHIVE_BACKEND" == "auto" ]]; then
    ARCHIVE_BACKEND="none"
  fi

  TOTAL_HOT_REQUESTS=$((FRAUD_THRESHOLD + 1))
  EXPECTED_DB_ROWS=$((1 + 1 + FRAUD_THRESHOLD))
  HOT_KEY="fraud:ip:${HOT_IP}"
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
  status=$(curl -s -o /tmp/event_pipeline_aws_resp.txt -w "%{http_code}" \
    -X POST "${COLLECTOR_URL}/event" \
    -H "Content-Type: application/json" \
    -H "X-Forwarded-For: ${ip}" \
    -d "$payload")

  [[ "$status" == "202" ]] || fail "collector returned ${status} for ${event_id}, body=$(cat /tmp/event_pipeline_aws_resp.txt)"
}

send_event() {
  send_event_with_campaign "$1" "$2" "$3" "$4" "$CAMPAIGN_ID"
}

query_campaign_status() {
  local campaign_id="$1"
  curl -s -o /tmp/event_pipeline_aws_campaign_resp.json -w "%{http_code}" "${QUERY_URL}/campaigns/${campaign_id}"
}

wait_for_campaign() {
  local campaign_id="$1"
  local attempts=90
  for ((i=1; i<=attempts; i++)); do
    local status
    status="$(query_campaign_status "$campaign_id")"
    if [[ "$status" == "200" ]]; then
      return 0
    fi
    sleep 2
  done
  return 1
}

redis_exec() {
  if [[ -n "$REDIS_ADDR" ]]; then
    local host port
    host="${REDIS_ADDR%:*}"
    port="${REDIS_ADDR##*:}"

    if command -v redis-cli >/dev/null 2>&1; then
      redis-cli -h "$host" -p "$port" "$@"
      return 0
    fi

    if k8s_available; then
      local redis_pod
      redis_pod="$(kubectl -n "$K8S_NAMESPACE" get pods -l app=redis -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true)"
      if [[ -n "$redis_pod" ]]; then
        kubectl -n "$K8S_NAMESPACE" exec "$redis_pod" -- redis-cli -h "$host" -p "$port" "$@"
        return 0
      fi
    fi

    return 1
  fi

  if k8s_available; then
    local redis_pod
    redis_pod="$(kubectl -n "$K8S_NAMESPACE" get pods -l app=redis -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true)"
    if [[ -n "$redis_pod" ]]; then
      kubectl -n "$K8S_NAMESPACE" exec "$redis_pod" -- redis-cli "$@"
      return 0
    fi
  fi

  return 1
}

db_scalar() {
  local sql="$1"
  [[ -n "$DB_URL" ]] || return 1
  command -v psql >/dev/null 2>&1 || return 1
  psql "$DB_URL" -tAc "$sql" 2>/dev/null | tr -d '[:space:]'
}

s3_count_prefix() {
  local prefix="$1"
  local count
  count=$(aws s3api list-objects-v2 --region "$AWS_REGION" --bucket "$S3_BUCKET" --prefix "$prefix/" --query 'length(Contents)' --output text 2>/dev/null || true)
  if [[ -z "$count" || "$count" == "None" ]]; then
    count="0"
  fi
  printf "%s" "$count"
}

check_redis() {
  log "Redis check"
  if ! redis_exec GET "$HOT_KEY" >/dev/null 2>&1; then
    if is_true "$STRICT_REDIS_CHECK"; then
      fail "redis check enabled but unable to connect"
    fi
    warn "redis check skipped (unreachable from test runner)"
    return
  fi

  local value
  value="$(redis_exec GET "$HOT_KEY" 2>/dev/null | tr -d '[:space:]')"
  if [[ -z "$value" ]]; then
    if is_true "$STRICT_REDIS_CHECK"; then
      fail "redis key ${HOT_KEY} missing"
    fi
    warn "redis key ${HOT_KEY} missing (possibly expired quickly)"
    return
  fi

  if [[ "$value" != "$TOTAL_HOT_REQUESTS" ]]; then
    if is_true "$STRICT_REDIS_CHECK"; then
      fail "redis hot-key mismatch expected=${TOTAL_HOT_REQUESTS}, got=${value}"
    fi
    warn "redis hot-key mismatch expected=${TOTAL_HOT_REQUESTS}, got=${value}"
    return
  fi

  log "Redis check passed: ${HOT_KEY}=${value}"
}

reset_redis_hot_key() {
  log "Redis preflight reset"
  if ! redis_exec DEL "$HOT_KEY" >/dev/null 2>&1; then
    if is_true "$STRICT_REDIS_CHECK"; then
      fail "redis reset enabled but unable to connect"
    fi
    warn "redis preflight reset skipped (unreachable from test runner)"
    return
  fi
  log "Redis preflight reset complete: ${HOT_KEY}"
}

check_db() {
  log "DB check"
  local db_total hot_persisted risk_gt80_count

  db_total="$(db_scalar "SELECT COUNT(*) FROM raw_events WHERE campaign_id='${CAMPAIGN_ID}';" || true)"
  if [[ -z "$db_total" ]]; then
    if is_true "$STRICT_DB_CHECK"; then
      fail "db check enabled but unable to query via DB_URL"
    fi
    warn "db check skipped (DB_URL/psql connectivity unavailable)"
    return
  fi

  hot_persisted="$(db_scalar "SELECT COUNT(*) FROM raw_events WHERE campaign_id='${CAMPAIGN_ID}' AND ip_address='${HOT_IP}';")"
  risk_gt80_count="$(db_scalar "SELECT COUNT(*) FROM raw_events WHERE campaign_id='${CAMPAIGN_ID}' AND risk_score > 80;")"

  [[ "$db_total" == "$EXPECTED_DB_ROWS" ]] || fail "unexpected DB row count: expected=${EXPECTED_DB_ROWS}, got=${db_total}"
  [[ "$hot_persisted" == "$FRAUD_THRESHOLD" ]] || fail "unexpected hot IP persisted count: expected=${FRAUD_THRESHOLD}, got=${hot_persisted}"
  [[ "$risk_gt80_count" == "0" ]] || fail "persisted rows with risk_score > 80 found: ${risk_gt80_count}"

  log "DB check passed"
}

check_s3() {
  if [[ "$ARCHIVE_BACKEND" != "s3" ]]; then
    log "S3 check skipped (archive backend=${ARCHIVE_BACKEND})"
    return
  fi

  if ! command -v aws >/dev/null 2>&1; then
    if is_true "$STRICT_S3_CHECK"; then
      fail "s3 check enabled but aws cli not found"
    fi
    warn "s3 check skipped (aws cli missing)"
    return
  fi

  [[ -n "$S3_BUCKET" ]] || {
    if is_true "$STRICT_S3_CHECK"; then
      fail "s3 check enabled but S3_BUCKET is empty"
    fi
    warn "s3 check skipped (S3_BUCKET missing)"
    return
  }

  local raw_before dup_before fraud_before accepted_before
  raw_before="$(s3_count_prefix raw)"
  dup_before="$(s3_count_prefix duplicate)"
  fraud_before="$(s3_count_prefix fraud)"
  accepted_before="$(s3_count_prefix accepted)"

  log "S3 baseline counts: raw=${raw_before}, duplicate=${dup_before}, fraud=${fraud_before}, accepted=${accepted_before}"

  S3_RAW_BEFORE="$raw_before"
  S3_DUP_BEFORE="$dup_before"
  S3_FRAUD_BEFORE="$fraud_before"
  S3_ACCEPTED_BEFORE="$accepted_before"
}

assert_s3_after() {
  if [[ "$ARCHIVE_BACKEND" != "s3" ]]; then
    return
  fi
  if [[ -z "${S3_RAW_BEFORE:-}" ]]; then
    return
  fi

  local raw_after dup_after fraud_after accepted_after
  raw_after="$(s3_count_prefix raw)"
  dup_after="$(s3_count_prefix duplicate)"
  fraud_after="$(s3_count_prefix fraud)"
  accepted_after="$(s3_count_prefix accepted)"

  local raw_delta dup_delta fraud_delta accepted_delta
  raw_delta=$((raw_after - S3_RAW_BEFORE))
  dup_delta=$((dup_after - S3_DUP_BEFORE))
  fraud_delta=$((fraud_after - S3_FRAUD_BEFORE))
  accepted_delta=$((accepted_after - S3_ACCEPTED_BEFORE))

  [[ "$raw_delta" -gt 0 ]] || fail "S3 raw/ prefix did not increase"
  [[ "$dup_delta" -gt 0 ]] || fail "S3 duplicate/ prefix did not increase"
  [[ "$fraud_delta" -gt 0 ]] || fail "S3 fraud/ prefix did not increase"
  [[ "$accepted_delta" -gt 0 ]] || fail "S3 accepted/ prefix did not increase"

  log "S3 check passed: raw=${raw_delta}, duplicate=${dup_delta}, fraud=${fraud_delta}, accepted=${accepted_delta}"
}

check_query() {
  log "Query API check"

  curl -fsS "${QUERY_URL}/campaigns" >/tmp/event_pipeline_aws_campaigns_resp.json || fail "query-service /campaigns request failed"

  local status
  status="$(query_campaign_status "$CAMPAIGN_ID")"
  [[ "$status" == "200" ]] || fail "query-service /campaigns/${CAMPAIGN_ID} returned ${status}"

  local body
  body="$(cat /tmp/event_pipeline_aws_campaign_resp.json)"
  case "$body" in
    *"\"campaign_id\":\"${CAMPAIGN_ID}\""*) ;;
    *) fail "query response missing expected campaign_id=${CAMPAIGN_ID}" ;;
  esac

  log "Query API check passed"
}

main() {
  parse_args "$@"

  require_cmd curl

  discover_runtime_config
  normalize_runtime_defaults

  log "runtime summary"
  log "collector_url=${COLLECTOR_URL}"
  log "query_url=${QUERY_URL}"
  log "archive_backend=${ARCHIVE_BACKEND}"
  log "s3_bucket=${S3_BUCKET:-<unset>}"
  log "redis_addr=${REDIS_ADDR:-<unset>}"
  log "db_url_set=$([[ -n "$DB_URL" ]] && printf yes || printf no)"
  log "fraud_threshold=${FRAUD_THRESHOLD}"
  log "campaign_id=${CAMPAIGN_ID}"

  log "preflight health checks"
  curl -fsS "${COLLECTOR_URL}/health" >/dev/null || fail "collector health check failed"
  curl -fsS "${QUERY_URL}/health" >/dev/null || fail "query health check failed"

  reset_redis_hot_key
  check_s3

  log "worker probe event"
  send_event_with_campaign "evt-${RUN_ID}-probe-1" "$SAFE_IP" "${USER_PREFIX}-probe" "impression" "$PROBE_CAMPAIGN_ID"
  wait_for_campaign "$PROBE_CAMPAIGN_ID" || fail "probe campaign did not appear via query API"

  log "case 1: accepted path"
  send_event "evt-${RUN_ID}-accepted-1" "$SAFE_IP" "${USER_PREFIX}-accepted" "impression"

  log "case 2: duplicate path"
  send_event "evt-${RUN_ID}-dup-1" "$SAFE_IP" "${USER_PREFIX}-dup" "click"
  send_event "evt-${RUN_ID}-dup-1" "$SAFE_IP" "${USER_PREFIX}-dup" "click"

  log "case 3: fraud threshold path (${TOTAL_HOT_REQUESTS} events from ${HOT_IP})"
  for i in $(seq 1 "$TOTAL_HOT_REQUESTS"); do
    send_event "evt-${RUN_ID}-hot-${i}" "$HOT_IP" "${USER_PREFIX}-hot-${i}" "impression"
  done

  log "waiting for campaign to be queryable"
  wait_for_campaign "$CAMPAIGN_ID" || fail "campaign ${CAMPAIGN_ID} not returned by query-service in time"

  log "waiting for archive flush window"
  sleep 10

  check_query
  check_redis
  if is_true "$STRICT_DB_CHECK"; then
    check_db
  else
    log "DB check skipped (set --strict-db-check true to enable)"
  fi
  assert_s3_after

  cat <<EOF

AWS test complete.
- campaign_id: ${CAMPAIGN_ID}
- probe_campaign_id: ${PROBE_CAMPAIGN_ID}
- expected_db_rows_if_db_checked: ${EXPECTED_DB_ROWS}
- query_response_file: /tmp/event_pipeline_aws_campaign_resp.json
EOF
}

main "$@"
