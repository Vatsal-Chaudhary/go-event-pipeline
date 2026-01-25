package fraud

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type LambdaClient struct {
	endpoint   string
	httpClient *http.Client
}

type FraudRequest struct {
	IPAddress string `json:"ip_address"`
	UserID    string `json:"user_id"`
}

type FraudResponse struct {
	RiskScore int `json:"risk_score"`
}

func NewLambdaClient(endpoint string) *LambdaClient {
	return &LambdaClient{
		endpoint:   endpoint,
		httpClient: &http.Client{},
	}
}

func (l *LambdaClient) CheckFraud(ctx context.Context, ipAddress, userID string) (int, error) {
	request := FraudRequest{
		IPAddress: ipAddress,
		UserID:    userID,
	}

	jsonData, err := json.Marshal(request)
	if err != nil {
		return 0, fmt.Errorf("marshal request: %w", err)
	}

	// Lambda Runtime Interface Emulator expects invocations at this endpoint
	url := fmt.Sprintf("%s/2015-03-31/functions/function/invocations", l.endpoint)

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonData))
	if err != nil {
		return 0, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := l.httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("invoke lambda: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("lambda returned status %d: %s", resp.StatusCode, string(body))
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
