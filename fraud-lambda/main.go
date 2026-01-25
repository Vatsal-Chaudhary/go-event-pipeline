package main

import (
	"context"

	"github.com/aws/aws-lambda-go/lambda"
)

type FraudRequest struct {
	IPAddress string `json:"ip_address"`
	UserID    string `json:"user_id"`
}

type FraudResponse struct {
	RiskScore int `json:"risk_score"`
}

func HandleRequest(ctx context.Context, request FraudRequest) (FraudResponse, error) {
	// Hardcoded fraud detection logic for testing
	// If IP is 10.0.0.99, mark as high risk
	if request.IPAddress == "10.0.0.99" {
		return FraudResponse{
			RiskScore: 100,
		}, nil
	}

	// Default: low risk
	return FraudResponse{
		RiskScore: 0,
	}, nil
}

func main() {
	lambda.Start(HandleRequest)
}
