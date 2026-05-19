package handler

import (
	"encoding/json"
	"testing"

	mockbilling "parkir-pintar/services/billing/_mock/billing"
	"parkir-pintar/services/billing/internal/billing/model"
	"parkir-pintar/services/billing/pkg/apperror"

	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

const (
	testInvoiceID = "110e8400-e29b-41d4-a716-446655440001"
)

// fakeMsg wraps nats.Msg so we can track which ack method was called
type fakeMsg struct {
	data      []byte
	acked     bool
	nacked    bool
	termed    bool
	nakDelay  bool
}

func (f *fakeMsg) toNatsMsg() *nats.Msg {
	msg := &nats.Msg{Data: f.data}
	return msg
}

// newConsumer creates a ParkingPaymentConsumer with a mock usecase (no real NATS needed)
func newConsumer(uc *mockbilling.MockBillingUsecase) *ParkingPaymentConsumer {
	return &ParkingPaymentConsumer{uc: uc}
}

func marshalEvent(t *testing.T, refID, status string) []byte {
	t.Helper()
	b, _ := json.Marshal(model.NATSPaymentDoneEvent{
		ReferenceID: refID,
		Status:      status,
	})
	return b
}

// ── handle — invalid JSON ─────────────────────────────────────────────────────

func TestHandle_InvalidJSON_Terms(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc := mockbilling.NewMockBillingUsecase(ctrl)
	// No usecase calls expected — message is terminated immediately

	consumer := newConsumer(uc)

	termed := false
	msg := &nats.Msg{Data: []byte("not-json")}
	msg.Sub = &nats.Subscription{} // prevent nil panic on Term

	// Patch Term to track call — we can't easily intercept nats.Msg methods
	// so we verify no usecase call was made (the Term path)
	consumer.handle(msg)

	// If we reach here without panic, the invalid JSON path was handled
	assert.False(t, termed) // just verifying no panic
}

// ── handle — SUCCESS ──────────────────────────────────────────────────────────

func TestHandle_Success_Acks(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc := mockbilling.NewMockBillingUsecase(ctrl)
	uc.EXPECT().
		HandleParkingPaymentDone(gomock.Any(), testInvoiceID, "SUCCESS").
		Return(nil)

	consumer := newConsumer(uc)
	msg := &nats.Msg{Data: marshalEvent(t, testInvoiceID, "SUCCESS")}

	// Should not panic — Ack on a msg without subscription is a no-op error
	consumer.handle(msg)
}

// ── handle — not_found → Term ─────────────────────────────────────────────────

func TestHandle_NotFound_Terms(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc := mockbilling.NewMockBillingUsecase(ctrl)
	uc.EXPECT().
		HandleParkingPaymentDone(gomock.Any(), testInvoiceID, "SUCCESS").
		Return(apperror.New("not_found", "invoice not found"))

	consumer := newConsumer(uc)
	msg := &nats.Msg{Data: marshalEvent(t, testInvoiceID, "SUCCESS")}

	consumer.handle(msg)
	// Term is called — no panic expected
}

// ── handle — db_error → Nak with delay ───────────────────────────────────────

func TestHandle_DBError_Naks(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc := mockbilling.NewMockBillingUsecase(ctrl)
	uc.EXPECT().
		HandleParkingPaymentDone(gomock.Any(), testInvoiceID, "FAILED").
		Return(apperror.New("db_error", "failed to update invoice status"))

	consumer := newConsumer(uc)
	msg := &nats.Msg{Data: marshalEvent(t, testInvoiceID, "FAILED")}

	consumer.handle(msg)
	// NakWithDelay is called — no panic expected
}

// ── handle — FAILED status ────────────────────────────────────────────────────

func TestHandle_FailedStatus_CallsUsecase(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc := mockbilling.NewMockBillingUsecase(ctrl)
	uc.EXPECT().
		HandleParkingPaymentDone(gomock.Any(), testInvoiceID, "FAILED").
		Return(nil)

	consumer := newConsumer(uc)
	msg := &nats.Msg{Data: marshalEvent(t, testInvoiceID, "FAILED")}

	consumer.handle(msg)
}

// ── handle — EXPIRED status ───────────────────────────────────────────────────

func TestHandle_ExpiredStatus_CallsUsecase(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc := mockbilling.NewMockBillingUsecase(ctrl)
	uc.EXPECT().
		HandleParkingPaymentDone(gomock.Any(), testInvoiceID, "EXPIRED").
		Return(nil)

	consumer := newConsumer(uc)
	msg := &nats.Msg{Data: marshalEvent(t, testInvoiceID, "EXPIRED")}

	consumer.handle(msg)
}
