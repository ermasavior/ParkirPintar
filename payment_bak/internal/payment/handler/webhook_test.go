package handler

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	mockpayment "parkir-pintar/services/payment/_mock/payment"
	"parkir-pintar/services/payment/internal/payment/model"
	"parkir-pintar/services/payment/pkg/apperror"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

const testSecret = "test-webhook-secret"

func sign(body []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

func makeRequest(t *testing.T, payload interface{}, secret string) *http.Request {
	t.Helper()
	body, err := json.Marshal(payload)
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, "/webhook/payment/callback", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if secret != "" {
		req.Header.Set("X-Webhook-Signature", sign(body, secret))
	}
	return req
}

func validPayload() model.WebhookCallbackRequest {
	return model.WebhookCallbackRequest{
		GatewayRef:  "GW-REF-123",
		ReferenceID: "550e8400-e29b-41d4-a716-446655440001",
		PaymentType: "BOOKING_FEE",
		Status:      "SUCCESS",
		AmountIDR:   5000,
	}
}

// ── Signature validation ──────────────────────────────────────────────────────

func TestHandleCallback_InvalidSignature(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc := mockpayment.NewMockPaymentUsecase(ctrl)
	h := NewWebhookHandler(uc, testSecret)

	req := makeRequest(t, validPayload(), "wrong-secret")
	w := httptest.NewRecorder()

	h.HandleCallback(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), `"error":"unauthorized"`)
}

func TestHandleCallback_MissingSignature(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc := mockpayment.NewMockPaymentUsecase(ctrl)
	h := NewWebhookHandler(uc, testSecret)

	req := makeRequest(t, validPayload(), "")
	w := httptest.NewRecorder()

	h.HandleCallback(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), `"error":"unauthorized"`)
}

// ── Bad request ───────────────────────────────────────────────────────────────

func TestHandleCallback_InvalidJSON(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc := mockpayment.NewMockPaymentUsecase(ctrl)
	h := NewWebhookHandler(uc, "") // no secret = skip sig validation

	body := []byte("not-json")
	req := httptest.NewRequest(http.MethodPost, "/webhook/payment/callback", bytes.NewReader(body))
	w := httptest.NewRecorder()

	h.HandleCallback(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandleCallback_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc := mockpayment.NewMockPaymentUsecase(ctrl)
	uc.EXPECT().HandleCallback(gomock.Any(), gomock.Any()).Return(nil)

	h := NewWebhookHandler(uc, "")

	req := makeRequest(t, validPayload(), "")
	w := httptest.NewRecorder()

	h.HandleCallback(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"status":"success"`)
}

// ── Business logic error returns 200, transient error returns 500 ────────────

func TestHandleCallback_BusinessLogicError_Returns200(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc := mockpayment.NewMockPaymentUsecase(ctrl)
	uc.EXPECT().HandleCallback(gomock.Any(), gomock.Any()).
		Return(apperror.New("not_found", "payment not found"))

	h := NewWebhookHandler(uc, "")

	req := makeRequest(t, validPayload(), "")
	w := httptest.NewRecorder()

	h.HandleCallback(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"status":"success"`)
}

func TestHandleCallback_TransientError_Returns500(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc := mockpayment.NewMockPaymentUsecase(ctrl)
	uc.EXPECT().HandleCallback(gomock.Any(), gomock.Any()).
		Return(apperror.New("db_error", "failed to update payment status"))

	h := NewWebhookHandler(uc, "")

	req := makeRequest(t, validPayload(), "")
	w := httptest.NewRecorder()

	h.HandleCallback(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), `"error":"db_error"`)
}

func TestHandleCallback_NATSError_Returns500(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc := mockpayment.NewMockPaymentUsecase(ctrl)
	uc.EXPECT().HandleCallback(gomock.Any(), gomock.Any()).
		Return(apperror.New("nats_error", "failed to publish payment event"))

	h := NewWebhookHandler(uc, "")

	req := makeRequest(t, validPayload(), "")
	w := httptest.NewRecorder()

	h.HandleCallback(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), `"error":"nats_error"`)
}

// ── No secret configured — skip validation ────────────────────────────────────

func TestHandleCallback_NoSecret_SkipsValidation(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc := mockpayment.NewMockPaymentUsecase(ctrl)
	uc.EXPECT().HandleCallback(gomock.Any(), gomock.Any()).Return(nil)

	h := NewWebhookHandler(uc, "")

	req := makeRequest(t, validPayload(), "")
	w := httptest.NewRecorder()

	h.HandleCallback(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}
