package usecase

import (
	"context"
	"testing"

	mockpayment "parkir-pintar/services/payment/_mock/payment"
	"parkir-pintar/services/payment/internal/payment/model"
	"parkir-pintar/services/payment/pkg/apperror"

	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

const (
	testPaymentID = "110e8400-e29b-41d4-a716-446655440001"
	testIdemKey   = "220e8400-e29b-41d4-a716-446655440002"
	testRefID     = "330e8400-e29b-41d4-a716-446655440003"
	testDriverID  = "440e8400-e29b-41d4-a716-446655440004"
)

func newUsecase(repo *mockpayment.MockPaymentRepository, nc *nats.Conn) *PaymentUsecase {
	return &PaymentUsecase{repo: repo, natsConn: nc, webhookSecret: "secret", gateway: nil}
}

func validCreateReq() model.CreatePaymentRequest {
	return model.CreatePaymentRequest{
		IdempotencyKey: testIdemKey,
		ReferenceID:    testRefID,
		PaymentType:    model.PaymentTypeBookingFee,
		AmountIDR:      5000,
		DriverID:       testDriverID,
		Method:         model.PaymentMethodQRIS,
	}
}

// ── CreatePayment ─────────────────────────────────────────────────────────────

func TestCreatePayment_IdempotencyReplay(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	existing := &model.Payment{
		ID:        testPaymentID,
		QRCodeURL: "https://qr.example.com",
		Status:    model.PaymentStatusPending,
	}

	repo := mockpayment.NewMockPaymentRepository(ctrl)
	repo.EXPECT().GetByIdempotencyKey(gomock.Any(), testIdemKey).Return(existing, nil)

	res, appErr := newUsecase(repo, nil).CreatePayment(context.Background(), validCreateReq())

	require.Nil(t, appErr)
	assert.Equal(t, testPaymentID, res.PaymentID)
	assert.Equal(t, "https://qr.example.com", res.QRCodeURL)
}

func TestCreatePayment_IdempotencyDBError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mockpayment.NewMockPaymentRepository(ctrl)
	repo.EXPECT().GetByIdempotencyKey(gomock.Any(), testIdemKey).
		Return(nil, apperror.New("db_error", "failed to query payment by idempotency key"))

	_, appErr := newUsecase(repo, nil).CreatePayment(context.Background(), validCreateReq())

	require.NotNil(t, appErr)
	assert.Equal(t, "db_error", appErr.ErrorCode)
}

func TestCreatePayment_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	created := &model.Payment{
		ID:        testPaymentID,
		QRCodeURL: "https://payment-gateway.example.com/qris/" + testPaymentID,
		Status:    model.PaymentStatusPending,
	}

	repo := mockpayment.NewMockPaymentRepository(ctrl)
	repo.EXPECT().GetByIdempotencyKey(gomock.Any(), testIdemKey).Return(nil, nil)
	repo.EXPECT().CreatePayment(gomock.Any(), gomock.Any()).Return(created, nil)

	res, appErr := newUsecase(repo, nil).CreatePayment(context.Background(), validCreateReq())

	require.Nil(t, appErr)
	assert.Equal(t, testPaymentID, res.PaymentID)
	assert.Equal(t, model.PaymentStatusPending, res.Status)
	assert.NotEmpty(t, res.QRCodeURL)
}

func TestCreatePayment_CreateFails(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mockpayment.NewMockPaymentRepository(ctrl)
	repo.EXPECT().GetByIdempotencyKey(gomock.Any(), testIdemKey).Return(nil, nil)
	repo.EXPECT().CreatePayment(gomock.Any(), gomock.Any()).
		Return(nil, apperror.New("db_error", "failed to create payment"))

	_, appErr := newUsecase(repo, nil).CreatePayment(context.Background(), validCreateReq())

	require.NotNil(t, appErr)
	assert.Equal(t, "db_error", appErr.ErrorCode)
}

// ── GetPaymentStatus ──────────────────────────────────────────────────────────

func TestGetPaymentStatus_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mockpayment.NewMockPaymentRepository(ctrl)
	repo.EXPECT().GetByID(gomock.Any(), testPaymentID).Return(&model.Payment{
		ID:          testPaymentID,
		ReferenceID: testRefID,
		PaymentType: model.PaymentTypeBookingFee,
		Status:      model.PaymentStatusSuccess,
		GatewayRef:  "GW-REF-123",
		AmountIDR:   5000,
	}, nil)

	res, appErr := newUsecase(repo, nil).GetPaymentStatus(context.Background(), testPaymentID)

	require.Nil(t, appErr)
	assert.Equal(t, testPaymentID, res.PaymentID)
	assert.Equal(t, model.PaymentStatusSuccess, res.Status)
	assert.Equal(t, "GW-REF-123", res.GatewayRef)
}

func TestGetPaymentStatus_NotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mockpayment.NewMockPaymentRepository(ctrl)
	repo.EXPECT().GetByID(gomock.Any(), testPaymentID).
		Return(nil, apperror.New("not_found", "payment not found"))

	_, appErr := newUsecase(repo, nil).GetPaymentStatus(context.Background(), testPaymentID)

	require.NotNil(t, appErr)
	assert.Equal(t, "not_found", appErr.ErrorCode)
}

// ── HandleCallback ────────────────────────────────────────────────────────────

func TestHandleCallback_Success_BookingFee(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Use a real NATS connection in test mode (no server needed — we just test the logic)
	// We'll use a mock for the repo and skip NATS publish by using a nil conn
	// For a real test, use nats.NewInMemory() or a test server
	repo := mockpayment.NewMockPaymentRepository(ctrl)
	repo.EXPECT().GetByReferenceAndType(gomock.Any(), testRefID, model.PaymentTypeBookingFee).
		Return(&model.Payment{ID: testPaymentID, ReferenceID: testRefID}, nil)
	repo.EXPECT().UpdateStatus(gomock.Any(), testPaymentID, model.PaymentStatusSuccess, "GW-REF-123").
		Return(nil)

	// Connect to embedded NATS for publish test
	ns, err := nats.Connect(nats.DefaultURL)
	if err != nil {
		t.Skip("NATS not available, skipping publish test")
	}
	defer ns.Close()

	appErr := newUsecase(repo, ns).HandleCallback(context.Background(), model.WebhookCallbackRequest{
		GatewayRef:  "GW-REF-123",
		ReferenceID: testRefID,
		PaymentType: "BOOKING_FEE",
		Status:      "SUCCESS",
		AmountIDR:   5000,
	})

	require.Nil(t, appErr)
}

func TestHandleCallback_UnknownPaymentType(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mockpayment.NewMockPaymentRepository(ctrl)

	appErr := newUsecase(repo, nil).HandleCallback(context.Background(), model.WebhookCallbackRequest{
		ReferenceID: testRefID,
		PaymentType: "UNKNOWN_TYPE",
		Status:      "SUCCESS",
	})

	require.NotNil(t, appErr)
	assert.Equal(t, "validation_error", appErr.ErrorCode)
}

func TestHandleCallback_UnknownStatus(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mockpayment.NewMockPaymentRepository(ctrl)
	repo.EXPECT().GetByReferenceAndType(gomock.Any(), testRefID, model.PaymentTypeBookingFee).
		Return(&model.Payment{ID: testPaymentID}, nil)

	appErr := newUsecase(repo, nil).HandleCallback(context.Background(), model.WebhookCallbackRequest{
		ReferenceID: testRefID,
		PaymentType: "BOOKING_FEE",
		Status:      "UNKNOWN",
	})

	require.NotNil(t, appErr)
	assert.Equal(t, "validation_error", appErr.ErrorCode)
}

func TestHandleCallback_PaymentNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mockpayment.NewMockPaymentRepository(ctrl)
	repo.EXPECT().GetByReferenceAndType(gomock.Any(), testRefID, model.PaymentTypeBookingFee).
		Return(nil, apperror.New("not_found", "payment not found for reference"))

	appErr := newUsecase(repo, nil).HandleCallback(context.Background(), model.WebhookCallbackRequest{
		ReferenceID: testRefID,
		PaymentType: "BOOKING_FEE",
		Status:      "SUCCESS",
	})

	require.NotNil(t, appErr)
	assert.Equal(t, "not_found", appErr.ErrorCode)
}

func TestHandleCallback_UpdateFails(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mockpayment.NewMockPaymentRepository(ctrl)
	repo.EXPECT().GetByReferenceAndType(gomock.Any(), testRefID, model.PaymentTypeBookingFee).
		Return(&model.Payment{ID: testPaymentID}, nil)
	repo.EXPECT().UpdateStatus(gomock.Any(), testPaymentID, model.PaymentStatusFailed, "").
		Return(apperror.New("db_error", "failed to update payment status"))

	appErr := newUsecase(repo, nil).HandleCallback(context.Background(), model.WebhookCallbackRequest{
		ReferenceID: testRefID,
		PaymentType: "BOOKING_FEE",
		Status:      "FAILED",
	})

	require.NotNil(t, appErr)
	assert.Equal(t, "db_error", appErr.ErrorCode)
}
