package handler

import (
	"context"
	"testing"

	mockpayment "parkir-pintar/services/payment/_mock/payment"
	pb "parkir-pintar/services/payment/gen/payment/v1"
	"parkir-pintar/services/payment/internal/payment/model"
	"parkir-pintar/services/payment/pkg/apperror"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	validPaymentID = "110e8400-e29b-41d4-a716-446655440001"
	validIdemKey   = "220e8400-e29b-41d4-a716-446655440002"
	validRefID     = "330e8400-e29b-41d4-a716-446655440003"
	validDriverID  = "440e8400-e29b-41d4-a716-446655440004"
)

func newServer(uc *mockpayment.MockPaymentUsecase) *PaymentServer {
	return &PaymentServer{uc: uc}
}

func grpcCode(err error) codes.Code {
	if s, ok := status.FromError(err); ok {
		return s.Code()
	}
	return codes.Unknown
}

func validCreateReq() *pb.CreatePaymentRequest {
	return &pb.CreatePaymentRequest{
		IdempotencyKey: validIdemKey,
		ReferenceId:    validRefID,
		DriverId:       validDriverID,
		PaymentType:    pb.PaymentType_PAYMENT_TYPE_BOOKING_FEE,
		AmountIdr:      5000,
		Method:         pb.PaymentMethod_PAYMENT_METHOD_QRIS,
	}
}

// ── CreatePayment — validation ────────────────────────────────────────────────

func TestCreatePayment_InvalidIdempotencyKey(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	req := validCreateReq()
	req.IdempotencyKey = "not-a-uuid"

	_, err := newServer(mockpayment.NewMockPaymentUsecase(ctrl)).CreatePayment(context.Background(), req)

	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, grpcCode(err))
	assert.Contains(t, status.Convert(err).Message(), "idempotency_key")
}

func TestCreatePayment_InvalidReferenceID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	req := validCreateReq()
	req.ReferenceId = "bad"

	_, err := newServer(mockpayment.NewMockPaymentUsecase(ctrl)).CreatePayment(context.Background(), req)

	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, grpcCode(err))
}

func TestCreatePayment_InvalidDriverID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	req := validCreateReq()
	req.DriverId = "bad"

	_, err := newServer(mockpayment.NewMockPaymentUsecase(ctrl)).CreatePayment(context.Background(), req)

	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, grpcCode(err))
}

func TestCreatePayment_ZeroAmount(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	req := validCreateReq()
	req.AmountIdr = 0

	_, err := newServer(mockpayment.NewMockPaymentUsecase(ctrl)).CreatePayment(context.Background(), req)

	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, grpcCode(err))
}

func TestCreatePayment_UnspecifiedPaymentType(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	req := validCreateReq()
	req.PaymentType = pb.PaymentType_PAYMENT_TYPE_UNSPECIFIED

	_, err := newServer(mockpayment.NewMockPaymentUsecase(ctrl)).CreatePayment(context.Background(), req)

	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, grpcCode(err))
}

func TestCreatePayment_UnspecifiedMethod(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	req := validCreateReq()
	req.Method = pb.PaymentMethod_PAYMENT_METHOD_UNSPECIFIED

	_, err := newServer(mockpayment.NewMockPaymentUsecase(ctrl)).CreatePayment(context.Background(), req)

	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, grpcCode(err))
}

// ── CreatePayment — usecase error / success ───────────────────────────────────

func TestCreatePayment_InternalError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc := mockpayment.NewMockPaymentUsecase(ctrl)
	uc.EXPECT().CreatePayment(gomock.Any(), gomock.Any()).
		Return(nil, apperror.New("db_error", "failed to create payment"))

	_, err := newServer(uc).CreatePayment(context.Background(), validCreateReq())

	require.Error(t, err)
	assert.Equal(t, codes.Internal, grpcCode(err))
}

func TestCreatePayment_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc := mockpayment.NewMockPaymentUsecase(ctrl)
	uc.EXPECT().CreatePayment(gomock.Any(), gomock.Any()).
		Return(&model.CreatePaymentResponse{
			PaymentID: validPaymentID,
			QRCodeURL: "https://qr.example.com",
			Status:    model.PaymentStatusPending,
		}, nil)

	res, err := newServer(uc).CreatePayment(context.Background(), validCreateReq())

	require.NoError(t, err)
	assert.Equal(t, validPaymentID, res.PaymentId)
	assert.Equal(t, "https://qr.example.com", res.QrCodeUrl)
	assert.Equal(t, pb.PaymentStatus_PAYMENT_STATUS_PENDING, res.Status)
}

// ── GetPaymentStatus — validation ─────────────────────────────────────────────

func TestGetPaymentStatus_InvalidPaymentID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	_, err := newServer(mockpayment.NewMockPaymentUsecase(ctrl)).GetPaymentStatus(context.Background(), &pb.GetPaymentStatusRequest{
		PaymentId: "not-a-uuid",
	})

	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, grpcCode(err))
}

// ── GetPaymentStatus — usecase error / success ────────────────────────────────

func TestGetPaymentStatus_NotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc := mockpayment.NewMockPaymentUsecase(ctrl)
	uc.EXPECT().GetPaymentStatus(gomock.Any(), validPaymentID).
		Return(nil, apperror.New("not_found", "payment not found"))

	_, err := newServer(uc).GetPaymentStatus(context.Background(), &pb.GetPaymentStatusRequest{PaymentId: validPaymentID})

	require.Error(t, err)
	assert.Equal(t, codes.NotFound, grpcCode(err))
}

func TestGetPaymentStatus_InternalError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc := mockpayment.NewMockPaymentUsecase(ctrl)
	uc.EXPECT().GetPaymentStatus(gomock.Any(), validPaymentID).
		Return(nil, apperror.New("db_error", "failed to query payment"))

	_, err := newServer(uc).GetPaymentStatus(context.Background(), &pb.GetPaymentStatusRequest{PaymentId: validPaymentID})

	require.Error(t, err)
	assert.Equal(t, codes.Internal, grpcCode(err))
}

func TestGetPaymentStatus_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc := mockpayment.NewMockPaymentUsecase(ctrl)
	uc.EXPECT().GetPaymentStatus(gomock.Any(), validPaymentID).
		Return(&model.GetPaymentStatusResponse{
			PaymentID:   validPaymentID,
			ReferenceID: validRefID,
			PaymentType: model.PaymentTypeBookingFee,
			Status:      model.PaymentStatusSuccess,
			GatewayRef:  "GW-REF-123",
			AmountIDR:   5000,
		}, nil)

	res, err := newServer(uc).GetPaymentStatus(context.Background(), &pb.GetPaymentStatusRequest{PaymentId: validPaymentID})

	require.NoError(t, err)
	assert.Equal(t, validPaymentID, res.PaymentId)
	assert.Equal(t, pb.PaymentStatus_PAYMENT_STATUS_SUCCESS, res.Status)
	assert.Equal(t, "GW-REF-123", res.GatewayRef)
	assert.Equal(t, int64(5000), res.AmountIdr)
}
