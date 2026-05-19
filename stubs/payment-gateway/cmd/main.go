package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"os"
	"sync"
	"time"
)

// paymentRecord stores the context needed to fire the callback
type paymentRecord struct {
	PaymentID   string
	ReferenceID string
	PaymentType string // "BOOKING_FEE" | "PARKING_FEE"
	AmountIDR   int64
}

// CreateQRISRequest is the inbound request from the Payment Service
type CreateQRISRequest struct {
	PaymentID   string `json:"payment_id"`
	ReferenceID string `json:"reference_id"`
	PaymentType string `json:"payment_type"` // "BOOKING_FEE" | "PARKING_FEE"
	AmountIDR   int64  `json:"amount_idr"`
}

// CreateQRISResponse is returned to the Payment Service
type CreateQRISResponse struct {
	QRCodeURL string `json:"qr_code_url"`
}

var (
	store         = map[string]*paymentRecord{} // keyed by payment_id
	mu            sync.RWMutex
	webhookSecret = getEnv("WEBHOOK_SECRET", "change-me-in-production")
	callbackURL   = getEnv("PAYMENT_CALLBACK_URL", "http://payment-service:8087/webhook/payment/callback")
	port          = getEnv("PORT", "8088")
	baseURL       = getEnv("BASE_URL", "http://localhost:8088")
)

func main() {
	mux := http.NewServeMux()

	// Payment Service calls this to register a payment and get a QR URL
	mux.HandleFunc("POST /qris/create", handleCreateQRIS)

	// Browser opens this to see the payment page
	mux.HandleFunc("GET /qris/{payment_id}", handleQRISPage)

	// Browser submits this to trigger the callback
	mux.HandleFunc("POST /qris/{payment_id}/pay", handlePay)

	slog.Info("payment gateway stub starting", slog.String("port", port))
	if err := http.ListenAndServe(fmt.Sprintf(":%s", port), mux); err != nil {
		slog.Error("server error", slog.String("error", err.Error()))
		os.Exit(1)
	}
}

// handleCreateQRIS registers a payment and returns a QR code URL.
// Called by the Payment Service when creating a new payment.
//
// POST /qris/create
// Body: { payment_id, reference_id, payment_type, amount_idr }
// Response: { qr_code_url }
func handleCreateQRIS(w http.ResponseWriter, r *http.Request) {
	var req CreateQRISRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	mu.Lock()
	store[req.PaymentID] = &paymentRecord{
		PaymentID:   req.PaymentID,
		ReferenceID: req.ReferenceID,
		PaymentType: req.PaymentType,
		AmountIDR:   req.AmountIDR,
	}
	mu.Unlock()

	qrURL := fmt.Sprintf("%s/qris/%s", baseURL, req.PaymentID)
	slog.Info("QRIS created",
		slog.String("payment_id", req.PaymentID),
		slog.String("qr_url", qrURL),
	)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(CreateQRISResponse{QRCodeURL: qrURL})
}

// handleQRISPage renders a simple HTML page simulating the QRIS payment screen.
//
// GET /qris/{payment_id}
func handleQRISPage(w http.ResponseWriter, r *http.Request) {
	paymentID := r.PathValue("payment_id")

	mu.RLock()
	rec, ok := store[paymentID]
	mu.RUnlock()

	if !ok {
		http.Error(w, "payment not found", http.StatusNotFound)
		return
	}

	tmpl := template.Must(template.New("qris").Parse(qrisPageHTML))
	w.Header().Set("Content-Type", "text/html")
	tmpl.Execute(w, map[string]any{
		"PaymentID":   rec.PaymentID,
		"ReferenceID": rec.ReferenceID,
		"PaymentType": rec.PaymentType,
		"AmountIDR":   rec.AmountIDR,
	})
}

// handlePay triggers the webhook callback to the Payment Service.
//
// POST /qris/{payment_id}/pay?status=SUCCESS|FAILED|EXPIRED
func handlePay(w http.ResponseWriter, r *http.Request) {
	paymentID := r.PathValue("payment_id")
	status := r.URL.Query().Get("status")
	if status == "" {
		status = "SUCCESS"
	}

	mu.RLock()
	rec, ok := store[paymentID]
	mu.RUnlock()

	if !ok {
		http.Error(w, "payment not found", http.StatusNotFound)
		return
	}

	payload := map[string]any{
		"gateway_ref":  fmt.Sprintf("GW-STUB-%s", paymentID[:8]),
		"reference_id": rec.ReferenceID,
		"payment_type": rec.PaymentType,
		"status":       status,
		"amount_idr":   rec.AmountIDR,
		"paid_at":      time.Now().UTC().Format(time.RFC3339),
	}

	body, _ := json.Marshal(payload)
	sig := computeHMAC(body, webhookSecret)

	req, err := http.NewRequest("POST", callbackURL, bytes.NewReader(body))
	if err != nil {
		slog.Error("failed to build callback request", slog.String("error", err.Error()))
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Webhook-Signature", sig)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		slog.Error("callback failed", slog.String("error", err.Error()))
		http.Error(w, "callback failed: "+err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	slog.Info("callback sent",
		slog.String("payment_id", paymentID),
		slog.String("status", status),
		slog.Int("response_code", resp.StatusCode),
	)

	// Remove from store after successful callback
	mu.Lock()
	delete(store, paymentID)
	mu.Unlock()

	// Redirect back to a confirmation page
	tmpl := template.Must(template.New("done").Parse(donepageHTML))
	w.Header().Set("Content-Type", "text/html")
	tmpl.Execute(w, map[string]any{
		"Status":    status,
		"PaymentID": paymentID,
	})
}

func computeHMAC(body []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

func getEnv(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

const qrisPageHTML = `<!DOCTYPE html>
<html>
<head>
  <title>ParkirPintar — QRIS Payment Stub</title>
  <style>
    body { font-family: sans-serif; max-width: 480px; margin: 60px auto; padding: 0 20px; }
    .card { border: 1px solid #ddd; border-radius: 8px; padding: 24px; }
    .amount { font-size: 2em; font-weight: bold; color: #1a1a1a; margin: 16px 0; }
    .type { color: #666; font-size: 0.9em; text-transform: uppercase; letter-spacing: 1px; }
    .qr-placeholder { background: #f5f5f5; border: 2px dashed #ccc; border-radius: 8px;
                      height: 200px; display: flex; align-items: center; justify-content: center;
                      color: #999; font-size: 0.9em; margin: 20px 0; }
    .btn { display: inline-block; padding: 12px 24px; border-radius: 6px; border: none;
           font-size: 1em; cursor: pointer; text-decoration: none; margin: 4px; }
    .btn-success { background: #22c55e; color: white; }
    .btn-failed  { background: #ef4444; color: white; }
    .btn-expired { background: #f59e0b; color: white; }
    .ref { font-size: 0.75em; color: #999; margin-top: 16px; word-break: break-all; }
  </style>
</head>
<body>
  <div class="card">
    <div class="type">{{.PaymentType}}</div>
    <div class="amount">IDR {{.AmountIDR}}</div>
    <div class="qr-placeholder">[ QRIS Code Stub ]<br/>Scan with banking app</div>
    <p>Simulate payment outcome:</p>
    <form method="POST" action="/qris/{{.PaymentID}}/pay" style="display:inline">
      <input type="hidden" name="status" value="SUCCESS">
      <button class="btn btn-success" formaction="/qris/{{.PaymentID}}/pay?status=SUCCESS">✓ Pay (SUCCESS)</button>
    </form>
    <form method="POST" action="/qris/{{.PaymentID}}/pay?status=FAILED" style="display:inline">
      <button class="btn btn-failed">✗ Fail (FAILED)</button>
    </form>
    <form method="POST" action="/qris/{{.PaymentID}}/pay?status=EXPIRED" style="display:inline">
      <button class="btn btn-expired">⏱ Expire (EXPIRED)</button>
    </form>
    <div class="ref">payment_id: {{.PaymentID}}<br/>reference_id: {{.ReferenceID}}</div>
  </div>
</body>
</html>`

const donepageHTML = `<!DOCTYPE html>
<html>
<head>
  <title>Payment {{.Status}}</title>
  <style>
    body { font-family: sans-serif; max-width: 480px; margin: 60px auto; text-align: center; }
    .status { font-size: 2em; font-weight: bold; margin: 20px 0; }
    .success { color: #22c55e; }
    .failed  { color: #ef4444; }
    .expired { color: #f59e0b; }
  </style>
</head>
<body>
  <div class="status {{if eq .Status "SUCCESS"}}success{{else if eq .Status "FAILED"}}failed{{else}}expired{{end}}">
    {{if eq .Status "SUCCESS"}}✓ Payment Successful{{else if eq .Status "FAILED"}}✗ Payment Failed{{else}}⏱ Payment Expired{{end}}
  </div>
  <p>Callback sent to Payment Service.</p>
  <p style="color:#999;font-size:0.8em">payment_id: {{.PaymentID}}</p>
</body>
</html>`
