package fraud

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

type Scorer interface {
	CheckFraud(ctx context.Context, ipAddress, userID string) (int, error)
}

type Config struct {
	Mode          string
	Endpoint      string
	RedisAddr     string
	IPWindowSec   int
	IPThreshold   int
	FraudScore    int
	NonFraudScore int
}

type FraudRequest struct {
	IPAddress string `json:"ip_address"`
	UserID    string `json:"user_id"`
}

type FraudResponse struct {
	RiskScore int `json:"risk_score"`
}

type HTTPScorer struct {
	endpoint   string
	httpClient *http.Client
}

type InProcessScorer struct {
	redisAddr     string
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
	mode := strings.ToLower(strings.TrimSpace(cfg.Mode))
	if mode == "" {
		mode = "inprocess"
	}

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

	switch mode {
	case "http":
		if cfg.Endpoint == "" {
			return nil, fmt.Errorf("FRAUD_ENDPOINT is required for FRAUD_MODE=http")
		}
		return &HTTPScorer{
			endpoint: strings.TrimRight(cfg.Endpoint, "/"),
			httpClient: &http.Client{
				Timeout: 5 * time.Second,
			},
		}, nil
	case "inprocess":
		if cfg.RedisAddr == "" {
			return nil, fmt.Errorf("REDIS_ADDR is required for FRAUD_MODE=inprocess")
		}
		client := redis.NewClient(&redis.Options{Addr: cfg.RedisAddr})
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := client.Ping(ctx).Err(); err != nil {
			return nil, fmt.Errorf("ping redis for inprocess scorer: %w", err)
		}

		return &InProcessScorer{
			redisAddr:     cfg.RedisAddr,
			ipWindowSec:   cfg.IPWindowSec,
			ipThreshold:   cfg.IPThreshold,
			fraudScore:    cfg.FraudScore,
			nonFraudScore: cfg.NonFraudScore,
			redisClient:   client,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported FRAUD_MODE: %s", mode)
	}
}

func (h *HTTPScorer) CheckFraud(ctx context.Context, ipAddress, userID string) (int, error) {
	request := FraudRequest{IPAddress: ipAddress, UserID: userID}

	jsonData, err := json.Marshal(request)
	if err != nil {
		return 0, fmt.Errorf("marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/score", h.endpoint)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jsonData))
	if err != nil {
		return 0, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("invoke fraud service: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("fraud service returned status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("read response: %w", err)
	}

	var response FraudResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return 0, fmt.Errorf("unmarshal response: %w", err)
	}

	return response.RiskScore, nil
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
