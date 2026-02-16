package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/redis/go-redis/v9"
)

type FraudRequest struct {
	IPAddress string `json:"ip_address"`
	UserID    string `json:"user_id"`
}

type FraudResponse struct {
	RiskScore int `json:"risk_score"`
}

type IPThresholdChecker struct {
	rdb       *redis.Client
	ttlSec    int64
	threshold int64
}

var incrWithTTLScript = redis.NewScript(`
local count = redis.call("INCR", KEYS[1])
if count == 1 then
  redis.call("EXPIRE", KEYS[1], ARGV[1])
end
return count
`)

var checker *IPThresholdChecker

func NewIPThresholdChecker() (*IPThresholdChecker, error) {
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		addr = "localhost:6379"
	}

	ttlSec := int64(300)
	if value := os.Getenv("FRAUD_IP_WINDOW_SEC"); value != "" {
		parsed, err := strconv.ParseInt(value, 10, 64)
		if err != nil || parsed <= 0 {
			return nil, fmt.Errorf("invalid FRAUD_IP_WINDOW_SEC=%q", value)
		}
		ttlSec = parsed
	}

	threshold := int64(100)
	if value := os.Getenv("FRAUD_IP_THRESHOLD"); value != "" {
		parsed, err := strconv.ParseInt(value, 10, 64)
		if err != nil || parsed <= 0 {
			return nil, fmt.Errorf("invalid FRAUD_IP_THRESHOLD=%q", value)
		}
		threshold = parsed
	}

	rdb := redis.NewClient(&redis.Options{Addr: addr})
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		return nil, fmt.Errorf("redis ping: %w", err)
	}

	return &IPThresholdChecker{
		rdb:       rdb,
		ttlSec:    ttlSec,
		threshold: threshold,
	}, nil
}

func (c *IPThresholdChecker) IsFraudIP(ctx context.Context, ip string) (bool, int64, error) {
	key := "fraud:ip:" + ip
	count, err := incrWithTTLScript.Run(ctx, c.rdb, []string{key}, c.ttlSec).Int64()
	if err != nil {
		return false, 0, fmt.Errorf("increment ip counter: %w", err)
	}

	return count > c.threshold, count, nil
}

func HandleRequest(ctx context.Context, request FraudRequest) (FraudResponse, error) {
	if request.IPAddress == "" {
		return FraudResponse{RiskScore: 0}, nil
	}

	isFraud, count, err := checker.IsFraudIP(ctx, request.IPAddress)
	if err != nil {
		log.Printf("fraud check fallback for ip=%s user=%s: %v", request.IPAddress, request.UserID, err)
		return FraudResponse{RiskScore: 0}, nil
	}

	if isFraud {
		log.Printf("fraud threshold exceeded ip=%s count=%d threshold=%d", request.IPAddress, count, checker.threshold)
		return FraudResponse{RiskScore: 90}, nil
	}

	log.Printf("fraud check passed ip=%s count=%d threshold=%d", request.IPAddress, count, checker.threshold)
	return FraudResponse{RiskScore: 10}, nil

}

func main() {
	var err error
	checker, err = NewIPThresholdChecker()
	if err != nil {
		log.Fatalf("initialize fraud checker: %v", err)
	}

	log.Printf("fraud checker initialized: threshold=%d window_sec=%d", checker.threshold, checker.ttlSec)
	lambda.Start(HandleRequest)
}
