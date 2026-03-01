package fraud

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/redis/go-redis/v9"
)

type Scorer interface {
	CheckFraud(ctx context.Context, ipAddress, userID string) (int, error)
}

type Config struct {
	RedisAddr     string
	IPWindowSec   int
	IPThreshold   int
	FraudScore    int
	NonFraudScore int
}

type InProcessScorer struct {
	ipWindowSec   int
	ipThreshold   int
	fraudScore    int
	nonFraudScore int
	redisClient   *redis.Client
}

var incrWithTTLScript = redis.NewScript(`
local count = redis.call("INCR", KEYS[1])
if count == 1 then
  redis.call("EXPIRE", KEYS[1], ARGV[1])
end
return count
`)

func NewScorer(cfg Config) (Scorer, error) {
	if cfg.IPWindowSec <= 0 {
		cfg.IPWindowSec = 300
	}
	if cfg.IPThreshold <= 0 {
		cfg.IPThreshold = 100
	}
	if cfg.FraudScore <= 0 {
		cfg.FraudScore = 90
	}
	if cfg.NonFraudScore <= 0 {
		cfg.NonFraudScore = 10
	}

	if cfg.RedisAddr == "" {
		return nil, fmt.Errorf("REDIS_ADDR is required for fraud scorer")
	}
	client := redis.NewClient(&redis.Options{Addr: cfg.RedisAddr})
	if err := client.Ping(context.Background()).Err(); err != nil {
		return nil, fmt.Errorf("ping redis for fraud scorer: %w", err)
	}

	return &InProcessScorer{
		ipWindowSec:   cfg.IPWindowSec,
		ipThreshold:   cfg.IPThreshold,
		fraudScore:    cfg.FraudScore,
		nonFraudScore: cfg.NonFraudScore,
		redisClient:   client,
	}, nil
}

func (s *InProcessScorer) CheckFraud(ctx context.Context, ipAddress, _ string) (int, error) {
	if strings.TrimSpace(ipAddress) == "" {
		return 0, nil
	}

	key := "fraud:ip:" + ipAddress
	count, err := incrWithTTLScript.Run(ctx, s.redisClient, []string{key}, strconv.Itoa(s.ipWindowSec)).Int64()
	if err != nil {
		return 0, fmt.Errorf("increment ip counter: %w", err)
	}

	if count > int64(s.ipThreshold) {
		return s.fraudScore, nil
	}

	return s.nonFraudScore, nil
}
