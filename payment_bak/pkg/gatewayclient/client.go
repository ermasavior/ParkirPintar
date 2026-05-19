package gatewayclient

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// CreateQRISRequest is sent to the payment gateway to register a payment.
type CreateQRISRequest struct {
	PaymentID   string `json:"payment_id"`
	ReferenceID string `json:"reference_id"`
	PaymentType string `json:"payment_type"` // "BOOKING_FEE" | "PARKING_FEE"
	AmountIDR   int64  `json:"amount_idr"`
}

// Gateway is the interface the payment usecase depends on.
// Allows mocking in tests.
type Gateway interface {
	CreateQRIS(req CreateQRISRequest) (string, error)
}

// Client is a concrete HTTP client for the payment gateway.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// New creates a new gateway client pointing at baseURL (e.g. "http://payment-gateway:8088").
func New(baseURL string) Gateway {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// CreateQRIS registers a payment with the gateway and returns the QR code URL.
func (c *Client) CreateQRIS(req CreateQRISRequest) (string, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("gatewayclient: marshal request: %w", err)
	}

	resp, err := c.httpClient.Post(
		c.baseURL+"/qris/create",
		"application/json",
		bytes.NewReader(body),
	)
	if err != nil {
		return "", fmt.Errorf("gatewayclient: POST /qris/create: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("gatewayclient: unexpected status %d", resp.StatusCode)
	}

	var result struct {
		QRCodeURL string `json:"qr_code_url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("gatewayclient: decode response: %w", err)
	}

	return result.QRCodeURL, nil
}
