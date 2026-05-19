package usecase

import (
	"context"
	"testing"
	"time"

	mockbilling "parkir-pintar/services/billing/_mock/billing"
	mockpaymentclient "parkir-pintar/services/billing/_mock/pkg/paymentclient"
	paymentpb "parkir-pintar/services/billing/gen/payment/v1"
	"parkir-pintar/services/billing/internal/billing/model"
	"parkir-pintar/services/billing/pkg/apperror"
	"parkir-pintar/services/billing/pkg/paymentclient"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

const (
	testInvoiceID     = "110e8400-e29b-41d4-a716-446655440001"
	testIdemKey       = "220e8400-e29b-41d4-a716-446655440002"
	testSessionID     = "330e8400-e29b-41d4-a716-446655440003"
	testReservationID = "440e8400-e29b-41d4-a716-446655440004"
	testDriverID      = "550e8400-e29b-41d4-a716-446655440005"
)

// newUsecase creates a usecase with no payment client — for tests that don't reach the payment call
func newUsecase(repo *mockbilling.MockBillingRepository) *BillingUsecase {
	return &BillingUsecase{repo: repo, paymentClient: nil}
}

// newUsecaseWithPayment creates a usecase with a mock payment client
func newUsecaseWithPayment(repo *mockbilling.MockBillingRepository, pc *mockpaymentclient.MockPaymentService) *BillingUsecase {
	return &BillingUsecase{repo: repo, paymentClient: pc}
}

// defaultPaymentMock returns a mock that succeeds for any CreatePayment call
func defaultPaymentMock(ctrl *gomock.Controller) *mockpaymentclient.MockPaymentService {
	pc := mockpaymentclient.NewMockPaymentService(ctrl)
	pc.EXPECT().CreatePayment(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(&paymentclient.CreatePaymentResult{
			PaymentID: "stub-payment-id",
			QRCodeURL: "https://qr.example.com/stub",
		}, nil).AnyTimes()
	return pc
}

func validCreateReq() model.CreateInvoiceRequest {
	now := time.Now()
	return model.CreateInvoiceRequest{
		IdempotencyKey: testIdemKey,
		SessionID:      testSessionID,
		ReservationID:  testReservationID,
		DriverID:       testDriverID,
		CheckedInAt:    now.Add(-2 * time.Hour),
		CheckedOutAt:   now,
	}
}

// ── CalculateAndCreateInvoice ─────────────────────────────────────────────────

func TestCalculateAndCreateInvoice_IdempotencyReplay(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	existing := &model.Invoice{
		ID:              testInvoiceID,
		BookingFeeIDR:   5000,
		ParkingFeeIDR:   10000,
		OvernightFeeIDR: 0,
		TotalIDR:        15000,
		QRCodeURL:       "https://qr.example.com",
	}

	repo := mockbilling.NewMockBillingRepository(ctrl)
	repo.EXPECT().GetByIdempotencyKey(gomock.Any(), testIdemKey).Return(existing, nil)

	res, appErr := newUsecase(repo).CalculateAndCreateInvoice(context.Background(), validCreateReq())

	require.Nil(t, appErr)
	assert.Equal(t, testInvoiceID, res.InvoiceID)
	assert.Equal(t, int64(15000), res.TotalIDR)
	assert.Equal(t, "https://qr.example.com", res.QRCodeURL)
}

func TestCalculateAndCreateInvoice_IdempotencyDBError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mockbilling.NewMockBillingRepository(ctrl)
	repo.EXPECT().GetByIdempotencyKey(gomock.Any(), testIdemKey).
		Return(nil, apperror.New("db_error", "failed to query invoice by idempotency key"))

	_, appErr := newUsecase(repo).CalculateAndCreateInvoice(context.Background(), validCreateReq())

	require.NotNil(t, appErr)
	assert.Equal(t, "db_error", appErr.ErrorCode)
}

func TestCalculateAndCreateInvoice_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	now := time.Now()
	created := &model.Invoice{
		ID:              testInvoiceID,
		BookingFeeIDR:   5000,
		ParkingFeeIDR:   10000,
		OvernightFeeIDR: 0,
		TotalIDR:        15000,
		QRCodeURL:       "https://qr.example.com/stub",
		CreatedAt:       now,
		Status:          model.InvoiceStatusPendingPayment,
	}

	repo := mockbilling.NewMockBillingRepository(ctrl)
	repo.EXPECT().GetByIdempotencyKey(gomock.Any(), testIdemKey).Return(nil, nil)
	repo.EXPECT().CreateInvoice(gomock.Any(), gomock.Any()).Return(created, nil)

	res, appErr := newUsecaseWithPayment(repo, defaultPaymentMock(ctrl)).CalculateAndCreateInvoice(context.Background(), validCreateReq())

	require.Nil(t, appErr)
	assert.Equal(t, testInvoiceID, res.InvoiceID)
	assert.Equal(t, int64(5000), res.BookingFeeIDR)
	assert.NotEmpty(t, res.QRCodeURL)
}

func TestCalculateAndCreateInvoice_CreateFails(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mockbilling.NewMockBillingRepository(ctrl)
	repo.EXPECT().GetByIdempotencyKey(gomock.Any(), testIdemKey).Return(nil, nil)
	repo.EXPECT().CreateInvoice(gomock.Any(), gomock.Any()).
		Return(nil, apperror.New("db_error", "failed to create invoice"))

	_, appErr := newUsecaseWithPayment(repo, defaultPaymentMock(ctrl)).CalculateAndCreateInvoice(context.Background(), validCreateReq())

	require.NotNil(t, appErr)
	assert.Equal(t, "db_error", appErr.ErrorCode)
}

func TestCalculateAndCreateInvoice_OvernightFee(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	checkedIn := time.Date(2026, 5, 17, 23, 0, 0, 0, time.Local)
	checkedOut := time.Date(2026, 5, 18, 1, 0, 0, 0, time.Local)

	created := &model.Invoice{
		ID:              testInvoiceID,
		BookingFeeIDR:   5000,
		ParkingFeeIDR:   10000,
		OvernightFeeIDR: 20000,
		TotalIDR:        35000,
		QRCodeURL:       "https://qr.example.com/stub",
		Status:          model.InvoiceStatusPendingPayment,
	}

	repo := mockbilling.NewMockBillingRepository(ctrl)
	repo.EXPECT().GetByIdempotencyKey(gomock.Any(), testIdemKey).Return(nil, nil)
	repo.EXPECT().CreateInvoice(gomock.Any(), gomock.Any()).Return(created, nil)

	res, appErr := newUsecaseWithPayment(repo, defaultPaymentMock(ctrl)).CalculateAndCreateInvoice(context.Background(), model.CreateInvoiceRequest{
		IdempotencyKey: testIdemKey,
		SessionID:      testSessionID,
		ReservationID:  testReservationID,
		DriverID:       testDriverID,
		CheckedInAt:    checkedIn,
		CheckedOutAt:   checkedOut,
	})

	require.Nil(t, appErr)
	assert.Equal(t, int64(20000), res.OvernightFeeIDR)
	assert.Equal(t, int64(35000), res.TotalIDR)
}

// ── CalculateAndCreateInvoice — payment service errors ───────────────────────

func TestCalculateAndCreateInvoice_PaymentServiceUnavailable(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mockbilling.NewMockBillingRepository(ctrl)
	repo.EXPECT().GetByIdempotencyKey(gomock.Any(), testIdemKey).Return(nil, nil)

	pc := mockpaymentclient.NewMockPaymentService(ctrl)
	pc.EXPECT().CreatePayment(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, apperror.New("payment_service_unavailable", "payment service is temporarily unavailable"))

	_, appErr := newUsecaseWithPayment(repo, pc).CalculateAndCreateInvoice(context.Background(), validCreateReq())

	require.NotNil(t, appErr)
	assert.Equal(t, "payment_service_unavailable", appErr.ErrorCode)
}

func TestCalculateAndCreateInvoice_PaymentServiceError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mockbilling.NewMockBillingRepository(ctrl)
	repo.EXPECT().GetByIdempotencyKey(gomock.Any(), testIdemKey).Return(nil, nil)

	pc := mockpaymentclient.NewMockPaymentService(ctrl)
	pc.EXPECT().CreatePayment(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, apperror.New("payment_service_error", "failed to create payment: rpc error"))

	_, appErr := newUsecaseWithPayment(repo, pc).CalculateAndCreateInvoice(context.Background(), validCreateReq())

	require.NotNil(t, appErr)
	assert.Equal(t, "payment_service_error", appErr.ErrorCode)
}

// ── GetInvoice ────────────────────────────────────────────────────────────────

func TestGetInvoice_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mockbilling.NewMockBillingRepository(ctrl)
	repo.EXPECT().GetByID(gomock.Any(), testInvoiceID).Return(&model.Invoice{
		ID:              testInvoiceID,
		SessionID:       testSessionID,
		ReservationID:   testReservationID,
		Status:          model.InvoiceStatusPendingPayment,
		BookingFeeIDR:   5000,
		ParkingFeeIDR:   10000,
		OvernightFeeIDR: 0,
		TotalIDR:        15000,
		CreatedAt:       time.Now(),
	}, nil)

	res, appErr := newUsecase(repo).GetInvoice(context.Background(), testInvoiceID)

	require.Nil(t, appErr)
	assert.Equal(t, testInvoiceID, res.InvoiceID)
	assert.Equal(t, model.InvoiceStatusPendingPayment, res.Status)
	assert.Equal(t, int64(15000), res.TotalIDR)
}

func TestGetInvoice_NotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mockbilling.NewMockBillingRepository(ctrl)
	repo.EXPECT().GetByID(gomock.Any(), testInvoiceID).
		Return(nil, apperror.New("not_found", "invoice not found"))

	_, appErr := newUsecase(repo).GetInvoice(context.Background(), testInvoiceID)

	require.NotNil(t, appErr)
	assert.Equal(t, "not_found", appErr.ErrorCode)
}

func TestGetInvoice_DBError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mockbilling.NewMockBillingRepository(ctrl)
	repo.EXPECT().GetByID(gomock.Any(), testInvoiceID).
		Return(nil, apperror.New("db_error", "failed to query invoice"))

	_, appErr := newUsecase(repo).GetInvoice(context.Background(), testInvoiceID)

	require.NotNil(t, appErr)
	assert.Equal(t, "db_error", appErr.ErrorCode)
}

// ── RetryPayment ──────────────────────────────────────────────────────────────

func TestRetryPayment_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mockbilling.NewMockBillingRepository(ctrl)
	repo.EXPECT().GetByIDForDriver(gomock.Any(), testInvoiceID, testDriverID).Return(&model.Invoice{
		ID:       testInvoiceID,
		Status:   model.InvoiceStatusPaymentFailed,
		TotalIDR: 15000,
	}, nil)
	repo.EXPECT().UpdateStatusToPendingPayment(gomock.Any(), testInvoiceID).Return(nil)

	res, appErr := newUsecaseWithPayment(repo, defaultPaymentMock(ctrl)).RetryPayment(context.Background(), model.RetryPaymentRequest{
		InvoiceID: testInvoiceID,
		DriverID:  testDriverID,
	})

	require.Nil(t, appErr)
	assert.NotEmpty(t, res.QRCodeURL)
	assert.NotEmpty(t, res.PaymentID)
}

func TestRetryPayment_InvoiceNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mockbilling.NewMockBillingRepository(ctrl)
	repo.EXPECT().GetByIDForDriver(gomock.Any(), testInvoiceID, testDriverID).
		Return(nil, apperror.New("not_found", "invoice not found or does not belong to driver"))

	_, appErr := newUsecase(repo).RetryPayment(context.Background(), model.RetryPaymentRequest{
		InvoiceID: testInvoiceID,
		DriverID:  testDriverID,
	})

	require.NotNil(t, appErr)
	assert.Equal(t, "not_found", appErr.ErrorCode)
}

func TestRetryPayment_NotPaymentFailed(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mockbilling.NewMockBillingRepository(ctrl)
	repo.EXPECT().GetByIDForDriver(gomock.Any(), testInvoiceID, testDriverID).
		Return(&model.Invoice{
			ID:     testInvoiceID,
			Status: model.InvoiceStatusPaid,
		}, nil)

	_, appErr := newUsecase(repo).RetryPayment(context.Background(), model.RetryPaymentRequest{
		InvoiceID: testInvoiceID,
		DriverID:  testDriverID,
	})

	require.NotNil(t, appErr)
	assert.Equal(t, "conflict", appErr.ErrorCode)
	assert.Contains(t, appErr.Message, "PAYMENT_FAILED")
}

func TestRetryPayment_UpdateFails(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mockbilling.NewMockBillingRepository(ctrl)
	repo.EXPECT().GetByIDForDriver(gomock.Any(), testInvoiceID, testDriverID).
		Return(&model.Invoice{
			ID:     testInvoiceID,
			Status: model.InvoiceStatusPaymentFailed,
		}, nil)
	repo.EXPECT().UpdateStatusToPendingPayment(gomock.Any(), testInvoiceID).
		Return(apperror.New("db_error", "failed to update invoice status"))

	_, appErr := newUsecase(repo).RetryPayment(context.Background(), model.RetryPaymentRequest{
		InvoiceID: testInvoiceID,
		DriverID:  testDriverID,
	})

	require.NotNil(t, appErr)
	assert.Equal(t, "db_error", appErr.ErrorCode)
}

func TestRetryPayment_PaymentServiceUnavailable(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mockbilling.NewMockBillingRepository(ctrl)
	repo.EXPECT().GetByIDForDriver(gomock.Any(), testInvoiceID, testDriverID).Return(&model.Invoice{
		ID:       testInvoiceID,
		Status:   model.InvoiceStatusPaymentFailed,
		TotalIDR: 15000,
	}, nil)
	repo.EXPECT().UpdateStatusToPendingPayment(gomock.Any(), testInvoiceID).Return(nil)

	pc := mockpaymentclient.NewMockPaymentService(ctrl)
	pc.EXPECT().CreatePayment(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, apperror.New("payment_service_unavailable", "payment service is temporarily unavailable"))

	_, appErr := newUsecaseWithPayment(repo, pc).RetryPayment(context.Background(), model.RetryPaymentRequest{
		InvoiceID: testInvoiceID,
		DriverID:  testDriverID,
	})

	require.NotNil(t, appErr)
	assert.Equal(t, "payment_service_unavailable", appErr.ErrorCode)
}

// ── HandleParkingPaymentDone ──────────────────────────────────────────────────

func TestHandleParkingPaymentDone_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mockbilling.NewMockBillingRepository(ctrl)
	repo.EXPECT().GetByID(gomock.Any(), testInvoiceID).Return(&model.Invoice{
		ID:     testInvoiceID,
		Status: model.InvoiceStatusPendingPayment,
	}, nil)
	repo.EXPECT().UpdateInvoiceStatus(gomock.Any(), testInvoiceID, model.InvoiceStatusPaid).Return(nil)

	appErr := newUsecase(repo).HandleParkingPaymentDone(context.Background(), testInvoiceID, "SUCCESS")

	assert.Nil(t, appErr)
}

func TestHandleParkingPaymentDone_Failed(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mockbilling.NewMockBillingRepository(ctrl)
	repo.EXPECT().GetByID(gomock.Any(), testInvoiceID).Return(&model.Invoice{
		ID:     testInvoiceID,
		Status: model.InvoiceStatusPendingPayment,
	}, nil)
	repo.EXPECT().UpdateInvoiceStatus(gomock.Any(), testInvoiceID, model.InvoiceStatusPaymentFailed).Return(nil)

	appErr := newUsecase(repo).HandleParkingPaymentDone(context.Background(), testInvoiceID, "FAILED")

	assert.Nil(t, appErr)
}

func TestHandleParkingPaymentDone_Expired(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mockbilling.NewMockBillingRepository(ctrl)
	repo.EXPECT().GetByID(gomock.Any(), testInvoiceID).Return(&model.Invoice{
		ID:     testInvoiceID,
		Status: model.InvoiceStatusPendingPayment,
	}, nil)
	repo.EXPECT().UpdateInvoiceStatus(gomock.Any(), testInvoiceID, model.InvoiceStatusPaymentFailed).Return(nil)

	appErr := newUsecase(repo).HandleParkingPaymentDone(context.Background(), testInvoiceID, "EXPIRED")

	assert.Nil(t, appErr)
}

func TestHandleParkingPaymentDone_InvoiceNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mockbilling.NewMockBillingRepository(ctrl)
	repo.EXPECT().GetByID(gomock.Any(), testInvoiceID).
		Return(nil, apperror.New("not_found", "invoice not found"))

	appErr := newUsecase(repo).HandleParkingPaymentDone(context.Background(), testInvoiceID, "SUCCESS")

	require.NotNil(t, appErr)
	assert.Equal(t, "not_found", appErr.ErrorCode)
}

func TestHandleParkingPaymentDone_UnknownStatus(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mockbilling.NewMockBillingRepository(ctrl)
	repo.EXPECT().GetByID(gomock.Any(), testInvoiceID).Return(&model.Invoice{
		ID:     testInvoiceID,
		Status: model.InvoiceStatusPendingPayment,
	}, nil)

	appErr := newUsecase(repo).HandleParkingPaymentDone(context.Background(), testInvoiceID, "UNKNOWN")

	require.NotNil(t, appErr)
	assert.Equal(t, "validation_error", appErr.ErrorCode)
}

func TestHandleParkingPaymentDone_UpdateFails(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mockbilling.NewMockBillingRepository(ctrl)
	repo.EXPECT().GetByID(gomock.Any(), testInvoiceID).Return(&model.Invoice{
		ID:     testInvoiceID,
		Status: model.InvoiceStatusPendingPayment,
	}, nil)
	repo.EXPECT().UpdateInvoiceStatus(gomock.Any(), testInvoiceID, model.InvoiceStatusPaid).
		Return(apperror.New("db_error", "failed to update invoice status"))

	appErr := newUsecase(repo).HandleParkingPaymentDone(context.Background(), testInvoiceID, "SUCCESS")

	require.NotNil(t, appErr)
	assert.Equal(t, "db_error", appErr.ErrorCode)
}

// ── MockPaymentService — verify interface ─────────────────────────────────────

func TestMockPaymentService_ReturnsResult(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	pc := mockpaymentclient.NewMockPaymentService(ctrl)
	pc.EXPECT().CreatePayment(gomock.Any(), "key", "ref", "driver",
		paymentpb.PaymentType_PAYMENT_TYPE_PARKING_FEE, int64(15000)).
		Return(&paymentclient.CreatePaymentResult{
			PaymentID: "mock-payment-id",
			QRCodeURL: "https://qr.example.com/mock",
		}, nil)

	result, appErr := pc.CreatePayment(context.Background(), "key", "ref", "driver",
		paymentpb.PaymentType_PAYMENT_TYPE_PARKING_FEE, 15000)

	require.Nil(t, appErr)
	assert.Equal(t, "mock-payment-id", result.PaymentID)
	assert.NotEmpty(t, result.QRCodeURL)
}
