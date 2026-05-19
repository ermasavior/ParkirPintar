package handler

import (
	"context"
	"testing"
	"time"

	mockbilling "parkir-pintar/services/billing/_mock/billing"
	pb "parkir-pintar/services/billing/gen/billing/v1"
	"parkir-pintar/services/billing/internal/billing/model"
	"parkir-pintar/services/billing/pkg/apperror"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	validInvoiceID     = "110e8400-e29b-41d4-a716-446655440001"
	validIdemKey       = "220e8400-e29b-41d4-a716-446655440002"
	validSessionID     = "330e8400-e29b-41d4-a716-446655440003"
	validReservationID = "440e8400-e29b-41d4-a716-446655440004"
	validDriverID      = "550e8400-e29b-41d4-a716-446655440005"
)

func newServer(uc *mockbilling.MockBillingUsecase) *BillingServer {
	return &BillingServer{uc: uc}
}

func grpcCode(err error) codes.Code {
	if s, ok := status.FromError(err); ok {
		return s.Code()
	}
	return codes.Unknown
}

func validCreateReq() *pb.CreateInvoiceRequest {
	now := time.Now()
	return &pb.CreateInvoiceRequest{
		IdempotencyKey: validIdemKey,
		SessionId:      validSessionID,
		ReservationId:  validReservationID,
		DriverId:       validDriverID,
		CheckedInAt:    timestamppb.New(now.Add(-2 * time.Hour)),
		CheckedOutAt:   timestamppb.New(now),
	}
}

// ── CalculateAndCreateInvoice — validation ────────────────────────────────────

func TestCalculateAndCreateInvoice_InvalidIdempotencyKey(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	req := validCreateReq()
	req.IdempotencyKey = "not-a-uuid"

	_, err := newServer(mockbilling.NewMockBillingUsecase(ctrl)).CalculateAndCreateInvoice(context.Background(), req)

	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, grpcCode(err))
	assert.Contains(t, status.Convert(err).Message(), "idempotency_key")
}

func TestCalculateAndCreateInvoice_InvalidSessionID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	req := validCreateReq()
	req.SessionId = "bad"

	_, err := newServer(mockbilling.NewMockBillingUsecase(ctrl)).CalculateAndCreateInvoice(context.Background(), req)

	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, grpcCode(err))
}

func TestCalculateAndCreateInvoice_InvalidReservationID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	req := validCreateReq()
	req.ReservationId = "bad"

	_, err := newServer(mockbilling.NewMockBillingUsecase(ctrl)).CalculateAndCreateInvoice(context.Background(), req)

	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, grpcCode(err))
}

func TestCalculateAndCreateInvoice_MissingCheckedInAt(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	req := validCreateReq()
	req.CheckedInAt = nil

	_, err := newServer(mockbilling.NewMockBillingUsecase(ctrl)).CalculateAndCreateInvoice(context.Background(), req)

	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, grpcCode(err))
}

func TestCalculateAndCreateInvoice_MissingCheckedOutAt(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	req := validCreateReq()
	req.CheckedOutAt = nil

	_, err := newServer(mockbilling.NewMockBillingUsecase(ctrl)).CalculateAndCreateInvoice(context.Background(), req)

	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, grpcCode(err))
}

// ── CalculateAndCreateInvoice — usecase error mapping ────────────────────────

func TestCalculateAndCreateInvoice_InternalError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc := mockbilling.NewMockBillingUsecase(ctrl)
	uc.EXPECT().CalculateAndCreateInvoice(gomock.Any(), gomock.Any()).
		Return(nil, apperror.New("db_error", "failed to create invoice"))

	_, err := newServer(uc).CalculateAndCreateInvoice(context.Background(), validCreateReq())

	require.Error(t, err)
	assert.Equal(t, codes.Internal, grpcCode(err))
}

// ── CalculateAndCreateInvoice — success ──────────────────────────────────────

func TestCalculateAndCreateInvoice_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc := mockbilling.NewMockBillingUsecase(ctrl)
	uc.EXPECT().CalculateAndCreateInvoice(gomock.Any(), gomock.Any()).
		Return(&model.CreateInvoiceResponse{
			InvoiceID:       validInvoiceID,
			BookingFeeIDR:   5000,
			ParkingFeeIDR:   10000,
			OvernightFeeIDR: 0,
			TotalIDR:        15000,
			QRCodeURL:       "https://qr.example.com",
		}, nil)

	res, err := newServer(uc).CalculateAndCreateInvoice(context.Background(), validCreateReq())

	require.NoError(t, err)
	assert.Equal(t, validInvoiceID, res.InvoiceId)
	assert.Equal(t, int64(5000), res.BookingFeeIdr)
	assert.Equal(t, int64(10000), res.ParkingFeeIdr)
	assert.Equal(t, int64(15000), res.TotalIdr)
	assert.Equal(t, "https://qr.example.com", res.QrCodeUrl)
}

// ── GetInvoice — validation ───────────────────────────────────────────────────

func TestGetInvoice_InvalidInvoiceID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	_, err := newServer(mockbilling.NewMockBillingUsecase(ctrl)).GetInvoice(context.Background(), &pb.GetInvoiceRequest{
		InvoiceId: "not-a-uuid",
	})

	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, grpcCode(err))
}

// ── GetInvoice — usecase error mapping ───────────────────────────────────────

func TestGetInvoice_NotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc := mockbilling.NewMockBillingUsecase(ctrl)
	uc.EXPECT().GetInvoice(gomock.Any(), validInvoiceID).
		Return(nil, apperror.New("not_found", "invoice not found"))

	_, err := newServer(uc).GetInvoice(context.Background(), &pb.GetInvoiceRequest{InvoiceId: validInvoiceID})

	require.Error(t, err)
	assert.Equal(t, codes.NotFound, grpcCode(err))
}

func TestGetInvoice_InternalError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc := mockbilling.NewMockBillingUsecase(ctrl)
	uc.EXPECT().GetInvoice(gomock.Any(), validInvoiceID).
		Return(nil, apperror.New("db_error", "failed to query invoice"))

	_, err := newServer(uc).GetInvoice(context.Background(), &pb.GetInvoiceRequest{InvoiceId: validInvoiceID})

	require.Error(t, err)
	assert.Equal(t, codes.Internal, grpcCode(err))
}

// ── GetInvoice — success ──────────────────────────────────────────────────────

func TestGetInvoice_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	now := time.Now()
	uc := mockbilling.NewMockBillingUsecase(ctrl)
	uc.EXPECT().GetInvoice(gomock.Any(), validInvoiceID).
		Return(&model.GetInvoiceResponse{
			InvoiceID:       validInvoiceID,
			SessionID:       validSessionID,
			ReservationID:   validReservationID,
			Status:          model.InvoiceStatusPendingPayment,
			BookingFeeIDR:   5000,
			ParkingFeeIDR:   10000,
			OvernightFeeIDR: 0,
			TotalIDR:        15000,
			CreatedAt:       now,
		}, nil)

	res, err := newServer(uc).GetInvoice(context.Background(), &pb.GetInvoiceRequest{InvoiceId: validInvoiceID})

	require.NoError(t, err)
	assert.Equal(t, validInvoiceID, res.InvoiceId)
	assert.Equal(t, pb.InvoiceStatus_INVOICE_STATUS_PENDING_PAYMENT, res.Status)
	assert.Equal(t, int64(15000), res.TotalIdr)
	assert.NotNil(t, res.CreatedAt)
}

// ── RetryPayment — validation ─────────────────────────────────────────────────

func TestRetryPayment_InvalidInvoiceID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	_, err := newServer(mockbilling.NewMockBillingUsecase(ctrl)).RetryPayment(context.Background(), &pb.RetryPaymentRequest{
		InvoiceId: "not-a-uuid",
		DriverId:  validDriverID,
	})

	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, grpcCode(err))
}

func TestRetryPayment_InvalidDriverID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	_, err := newServer(mockbilling.NewMockBillingUsecase(ctrl)).RetryPayment(context.Background(), &pb.RetryPaymentRequest{
		InvoiceId: validInvoiceID,
		DriverId:  "bad-driver",
	})

	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, grpcCode(err))
}

// ── RetryPayment — usecase error mapping ─────────────────────────────────────

func TestRetryPayment_NotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc := mockbilling.NewMockBillingUsecase(ctrl)
	uc.EXPECT().RetryPayment(gomock.Any(), gomock.Any()).
		Return(nil, apperror.New("not_found", "invoice not found"))

	_, err := newServer(uc).RetryPayment(context.Background(), &pb.RetryPaymentRequest{
		InvoiceId: validInvoiceID,
		DriverId:  validDriverID,
	})

	require.Error(t, err)
	assert.Equal(t, codes.NotFound, grpcCode(err))
}

func TestRetryPayment_Conflict(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc := mockbilling.NewMockBillingUsecase(ctrl)
	uc.EXPECT().RetryPayment(gomock.Any(), gomock.Any()).
		Return(nil, apperror.New("conflict", "invoice is not in PAYMENT_FAILED status"))

	_, err := newServer(uc).RetryPayment(context.Background(), &pb.RetryPaymentRequest{
		InvoiceId: validInvoiceID,
		DriverId:  validDriverID,
	})

	require.Error(t, err)
	assert.Equal(t, codes.FailedPrecondition, grpcCode(err))
}

func TestRetryPayment_InternalError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc := mockbilling.NewMockBillingUsecase(ctrl)
	uc.EXPECT().RetryPayment(gomock.Any(), gomock.Any()).
		Return(nil, apperror.New("db_error", "failed to update invoice status"))

	_, err := newServer(uc).RetryPayment(context.Background(), &pb.RetryPaymentRequest{
		InvoiceId: validInvoiceID,
		DriverId:  validDriverID,
	})

	require.Error(t, err)
	assert.Equal(t, codes.Internal, grpcCode(err))
}

// ── RetryPayment — success ────────────────────────────────────────────────────

func TestRetryPayment_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc := mockbilling.NewMockBillingUsecase(ctrl)
	uc.EXPECT().RetryPayment(gomock.Any(), gomock.Any()).
		Return(&model.RetryPaymentResponse{
			PaymentID: "pay-001",
			QRCodeURL: "https://qr.example.com/retry",
		}, nil)

	res, err := newServer(uc).RetryPayment(context.Background(), &pb.RetryPaymentRequest{
		InvoiceId: validInvoiceID,
		DriverId:  validDriverID,
	})

	require.NoError(t, err)
	assert.Equal(t, "pay-001", res.PaymentId)
	assert.Equal(t, "https://qr.example.com/retry", res.QrCodeUrl)
}

func TestCalculateAndCreateInvoice_InvalidDriverID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	req := validCreateReq()
	req.DriverId = "bad-driver"

	_, err := newServer(mockbilling.NewMockBillingUsecase(ctrl)).CalculateAndCreateInvoice(context.Background(), req)

	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, grpcCode(err))
	assert.Contains(t, status.Convert(err).Message(), "driver_id")
}
