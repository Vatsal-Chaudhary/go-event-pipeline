#!/usr/bin/env bash

URL="http://localhost:3000/event"

CAMPAIGNS=("campaign_1" "campaign_2" "campaign_3" "campaign_4" "campaign_5")
EVENT_TYPES=("impression" "click" "conversion")
COUNTRIES=("US" "IN" "DE" "FR" "UK")
DEVICES=("mobile" "desktop" "tablet")
SOURCES=("google" "facebook" "instagram" "tiktok" "linkedin")

for i in $(seq 1 100); do
  campaign=${CAMPAIGNS[$RANDOM % ${#CAMPAIGNS[@]}]}
  event_type=${EVENT_TYPES[$RANDOM % ${#EVENT_TYPES[@]}]}
  country=${COUNTRIES[$RANDOM % ${#COUNTRIES[@]}]}
  device=${DEVICES[$RANDOM % ${#DEVICES[@]}]}
  source=${SOURCES[$RANDOM % ${#SOURCES[@]}]}

  if [ "$event_type" == "conversion" ]; then
    meta=$(jq -n \
      --arg source "$source" \
      --arg revenue "$((RANDOM % 200 + 20))" \
      '{source: $source, revenue: ($revenue|tonumber), currency: "USD"}')
  else
    meta=$(jq -n --arg source "$source" '{source: $source}')
  fi

  payload=$(jq -n \
    --arg event_type "$event_type" \
    --arg user_id "user_$((RANDOM % 10))" \
    --arg campaign_id "$campaign" \
    --arg geo_country "$country" \
    --arg device_type "$device" \
    --arg created_at "$(date -u +"%Y-%m-%dT%H:%M:%SZ")" \
    --argjson meta_data "$meta" \
    '{
      event_type: $event_type,
      user_id: $user_id,
      campaign_id: $campaign_id,
      geo_country: $geo_country,
      device_type: $device_type,
      created_at: $created_at,
      meta_data: $meta_data
    }')

  echo "[$i] Sending $event_type for $campaign"

  curl -s -X POST "$URL" \
    -H "Content-Type: application/json" \
    -d "$payload" >/dev/null

  sleep 1
done

