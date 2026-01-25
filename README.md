# Go Event Pipeline

A production-ready event processing pipeline built with Go, Kafka (Redpanda), Redis, PostgreSQL, and MinIO. Features IP tracking, deduplication, abuse protection, and raw data archiving.

## Architecture

```
HTTP Request → Collector → Kafka (raw-events)
                                ↓
                   ┌────────────┴────────────┐
                   ↓                          ↓
              Archive                    Processor
           (MinIO batch)            (Redis dedup+abuse)
                                             ↓
                              Kafka (processed-events)
                                             ↓
                                         Worker
                                    (Postgres persist)
```

## Services

- **collector-service**: HTTP API for event ingestion with IP extraction
- **archive-service**: Raw event archival to MinIO (100 events / 5s batches)
- **processor-service**: Deduplication + IP abuse detection with Redis
- **worker-service**: Persists validated events to PostgreSQL
- **query-service**: Read API for analytics queries

## Quick Start

### Prerequisites

- Docker & Docker Compose
- Go 1.22+
- PostgreSQL client (psql)
- Redis CLI (optional)

### 1. Clone and Setup

```bash
git clone <repo-url>
cd go-event-pipeline
./setup.sh
```

This will:
- Create `.env` files from examples
- Install Go dependencies
- Start infrastructure (Postgres, Redis, Redpanda, MinIO)
- Apply database migrations

### 2. Start Services

**Option A: Manual (separate terminals)**
```bash
cd collector-service && make run  # Terminal 1
cd archive-service && make run    # Terminal 2
cd processor-service && make run  # Terminal 3
cd worker-service && make run     # Terminal 4
cd query-service && make run      # Terminal 5 (optional)
```

**Option B: Using goreman**
```bash
go install github.com/mattn/goreman@latest
goreman start
```

### 3. Test the Pipeline

```bash
./test_pipeline.sh
```

## Configuration

### Environment Variables

Each service uses a `.env` file. Examples:

**collector-service** (publishes to raw-events):
```env
KAFKA_BROKERS=localhost:19092
KAFKA_TOPIC=raw-events
PORT=9113
```

**processor-service** (dedup + abuse):
```env
KAFKA_INPUT_TOPIC=raw-events
KAFKA_OUTPUT_TOPIC=processed-events
REDIS_ADDR=localhost:6379
DEDUP_TTL_SEC=86400
IP_RATE_LIMIT_PER_MIN=100
```

**worker-service** (persists to DB):
```env
DB_URL=postgres://postgres:password@localhost:5432/analytics?sslmode=disable
KAFKA_TOPICS=processed-events
```

See `.env.example` files for all options.

## Key Features

### IP Tracking
- Extracts client IP from `X-Forwarded-For` or `RemoteAddr`
- Persists IP address with every event
- Enables geo-analytics and abuse detection

### Deduplication
- Redis-based with 24h TTL
- Uses `SETNX` for atomic dedup checks
- Prevents duplicate event processing

### IP Abuse Protection
- Rate limiting per IP (100 events/min default)
- Automatic blacklisting on threshold breach
- Live blacklist updates via Redis CLI

```bash
# Block an IP
redis-cli SADD ip_blacklist "1.2.3.4"

# Unblock an IP
redis-cli SREM ip_blacklist "1.2.3.4"

# View blacklist
redis-cli SMEMBERS ip_blacklist
```

### Raw Data Archiving
- Every event archived to MinIO before processing
- Batches: 100 events OR 5 seconds
- Path: `events/YYYY/MM/DD/batch_<timestamp>.json`
- Supports audit trails and event replay

### Fan-Out Processing
- Archive and Processor use different consumer groups
- Both receive ALL events from `raw-events` topic
- Decoupled: Archive failure doesn't block processing

## API Examples

### Send Single Event
```bash
curl -X POST http://localhost:9113/event \
  -H "Content-Type: application/json" \
  -H "X-Forwarded-For: 203.0.113.42" \
  -d '{
    "event_type": "impression",
    "user_id": "user123",
    "campaign_id": "camp456",
    "geo_country": "US",
    "device_type": "mobile"
  }'
```

### Send Batch
```bash
curl -X POST http://localhost:9113/events/batch \
  -H "Content-Type: application/json" \
  -H "X-Forwarded-For: 198.51.100.50" \
  -d '[
    {"event_type":"impression","user_id":"u1","campaign_id":"c1","geo_country":"US","device_type":"mobile"},
    {"event_type":"click","user_id":"u2","campaign_id":"c1","geo_country":"CA","device_type":"desktop"}
  ]'
```

### Query Stats (Query Service)
```bash
curl http://localhost:9111/campaigns/camp456/stats?hours=24
```

## Monitoring

### Access UIs

- **Redpanda Console**: http://localhost:8080 (Kafka topics)
- **MinIO Console**: http://localhost:9001 (minioadmin/minioadmin)
- **Postgres**: `psql -U postgres -h localhost -d analytics`

### Check Topics
```bash
docker exec redpanda rpk topic list
docker exec redpanda rpk topic consume raw-events --num 10
```

### Check Redis
```bash
redis-cli KEYS "event:id:*"    # Dedup keys
redis-cli KEYS "ip:count:*"    # Rate limit counters
redis-cli SMEMBERS ip_blacklist # Blacklisted IPs
```

### Check Database
```sql
-- Top IPs by event count
SELECT ip_address, COUNT(*) 
FROM raw_events 
GROUP BY ip_address 
ORDER BY COUNT(*) DESC 
LIMIT 10;
```

## Project Structure

```
go-event-pipeline/
├── collector-service/      # HTTP API + IP extraction
├── archive-service/        # MinIO raw event archival
├── processor-service/      # Redis dedup + abuse detection
├── worker-service/         # Postgres persistence
├── query-service/          # Analytics API
├── migrations/             # Database migrations
├── docker-compose.yaml     # Infrastructure services
├── setup.sh                # Quick setup script
├── test_pipeline.sh        # E2E test script
├── Procfile                # For goreman process manager
├── REFACTOR_GUIDE.md       # Detailed architecture guide
└── DELIVERABLES.md         # Summary of refactor changes
```

Each service follows:
```
service-name/
├── Makefile
├── cmd/main.go
├── go.mod
└── internal/
    ├── config/
    ├── handler/
    ├── kafka/
    └── models/
```

## Troubleshooting

### Events not reaching worker?
1. Check processor logs for IP blocks: `docker logs processor-service`
2. Verify Redis: `redis-cli PING`
3. Check Kafka lag: `docker exec redpanda rpk group describe processor-group`

### Archive uploads failing?
1. Check MinIO: `curl http://localhost:9000/minio/health/live`
2. Verify bucket: Access console at http://localhost:9001
3. Check disk space: `df -h`

### High Redis memory?
1. Check key count: `redis-cli DBSIZE`
2. Reduce `DEDUP_TTL_SEC` config
3. Clear old keys: `redis-cli --scan --pattern 'event:id:*' | xargs redis-cli DEL`

## Development

### Run Tests
```bash
cd collector-service && go test ./...
cd worker-service && go test ./...
```

### Build Binaries
```bash
cd collector-service && make build
./bin/collector-service
```

### Clean
```bash
make clean  # In each service directory
docker-compose down -v  # Reset infrastructure
```

## Production Considerations

- **TLS**: Enable for Redis, Kafka, and Postgres
- **Authentication**: Add API keys to collector endpoints
- **Rate Limiting**: Add nginx/API gateway in front of collector
- **Monitoring**: Add Prometheus + Grafana dashboards
- **Alerting**: Monitor dedup hit rate, IP block rate, archive failures
- **Scaling**: Increase Kafka partitions, scale processor/worker horizontally
- **Backup**: MinIO replication, Postgres WAL archiving

## Documentation

- **[REFACTOR_GUIDE.md](REFACTOR_GUIDE.md)**: Complete architecture, config, and operations guide
- **[DELIVERABLES.md](DELIVERABLES.md)**: Summary of refactor changes and implementation details

## License

MIT

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Run tests: `./test_pipeline.sh`
5. Submit a pull request
