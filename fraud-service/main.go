package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/redis/go-redis/v9"
)

type FraudRequest struct {
	IPAddress string `json:"ip_address"`
	UserID    string `json:"user_id"`
}

type FraudResponse struct {
	RiskScore int `json:"risk_score"`
}

type Scorer struct {
	redisClient   *redis.Client
	ipWindowSec   int
	ipThreshold   int
	fraudScore    int
	nonFraudScore int
}

var incrWithTTLScript = redis.NewScript(`
local count = redis.call("INCR", KEYS[1])
if count == 1 then
  redis.call("EXPIRE", KEYS[1], ARGV[1])
end
return count
`)

func NewScorer() (*Scorer, error) {
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}

	ipWindowSec := getIntEnv("FRAUD_IP_WINDOW_SEC", 300)
	ipThreshold := getIntEnv("FRAUD_IP_THRESHOLD", 100)
	fraudScore := getIntEnv("FRAUD_SCORE", 90)
	nonFraudScore := getIntEnv("NON_FRAUD_SCORE", 10)

	client := redis.NewClient(&redis.Options{Addr: redisAddr})
	if err := client.Ping(context.Background()).Err(); err != nil {
		return nil, fmt.Errorf("ping redis: %w", err)
	}

	return &Scorer{
		redisClient:   client,
		ipWindowSec:   ipWindowSec,
		ipThreshold:   ipThreshold,
		fraudScore:    fraudScore,
		nonFraudScore: nonFraudScore,
	}, nil
}

func (s *Scorer) Score(ctx context.Context, ipAddress string) (int, error) {
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

func scoreHandler(scorer *Scorer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req FraudRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}

		score, err := scorer.Score(r.Context(), req.IPAddress)
		if err != nil {
			http.Error(w, "failed to score request", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(FraudResponse{RiskScore: score}); err != nil {
			http.Error(w, "failed to encode response", http.StatusInternalServerError)
		}
	}
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("OK"))
}

func getIntEnv(key string, defaultValue int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return defaultValue
	}

	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return defaultValue
	}

	return parsed
}

func main() {
	scorer, err := NewScorer()
	if err != nil {
		log.Fatalf("failed to initialize scorer: %v", err)
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	http.HandleFunc("/score", scoreHandler(scorer))
	http.HandleFunc("/health", healthHandler)

	log.Printf("fraud-service running on :%s", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("http server error: %v", err)
	}
}
