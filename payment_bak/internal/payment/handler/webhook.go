package handler

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"

	"parkir-pintar/services/payment/internal/payment/model"
	"parkir-pintar/services/payment/internal/payment/usecase"
	"parkir-pintar/services/payment/pkg/httpresponse"
	"parkir-pintar/services/payment/pkg/logger"
)

type Handler struct {
	uc            usecase.Payment
	webhookSecret string
}

func NewWebhookHandler(uc usecase.Payment, webhookSecret string) *Handler {
	return &Handler{uc: uc, webhookSecret: webhookSecret}
}

// HandleCallback processes POST /webhook/payment/callback for Payment Gateway callback
func (h *Handler) HandleCallback(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Read body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		logger.Error(ctx, "webhook: failed to read body", slog.String("error", err.Error()))
		httpresponse.FailedBadRequest(w, "bad_request", "failed to read request body")
		return
	}
	defer r.Body.Close()

	// Validate HMAC-SHA256 signature
	sig := r.Header.Get("X-Webhook-Signature")
	if !h.validSignature(body, sig) {
		logger.Error(ctx, "webhook: invalid signature")
		httpresponse.FailedUnauthorized(w, "unauthorized", "invalid webhook signature")
		return
	}

	// Parse payload
	var req model.WebhookCallbackRequest
	if err := json.Unmarshal(body, &req); err != nil {
		logger.Error(ctx, "webhook: failed to parse payload", slog.String("error", err.Error()))
		httpresponse.FailedBadRequest(w, "bad_request", "invalid JSON payload")
		return
	}

	// Process callback
	if appErr := h.uc.HandleCallback(ctx, req); appErr != nil {
		logger.Error(ctx, "webhook: HandleCallback failed",
			slog.String("error", appErr.Error()),
			slog.String("reference_id", req.ReferenceID),
		)
		switch appErr.ErrorCode {
		case "db_error", "nats_error":
			// Transient errors — return 500 so the gateway retries
			httpresponse.FailedInternalServerError(w, appErr.ErrorCode, appErr.Message)
		default:
			// Business logic errors (not_found, validation_error) — return 200
			// to stop gateway retries; the error is already logged above
			httpresponse.Success(w, nil, appErr.Message)
		}
		return
	}

	httpresponse.Success(w, nil, "callback processed successfully")
}

// validSignature validates the HMAC-SHA256 signature from the gateway
func (h *Handler) validSignature(body []byte, sig string) bool {
	if h.webhookSecret == "" {
		// No secret configured — skip validation (dev/test only)
		return true
	}
	mac := hmac.New(sha256.New, []byte(h.webhookSecret))
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(sig))
}
