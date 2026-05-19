package handler

import (
	"context"
	"testing"
	"time"

	mockpresence "parkir-pintar/services/presence/_mock/presence"
	pb "parkir-pintar/services/presence/gen/presence/v1"
	"parkir-pintar/services/presence/internal/presence/model"
	"parkir-pintar/services/presence/pkg/apperror"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	validSessionID     = "110e8400-e29b-41d4-a716-446655440001"
	validReservationID = "220e8400-e29b-41d4-a716-446655440002"
	validDriverID      = "330e8400-e29b-41d4-a716-446655440003"
)

func newServer(uc *mockpresence.MockPresenceUsecase) *PresenceServer {
	return &PresenceServer{uc: uc}
}

func grpcCode(err error) codes.Code {
	if s, ok := status.FromError(err); ok {
		return s.Code()
	}
	return codes.Unknown
}

// ── CheckIn — validation ──────────────────────────────────────────────────────

func TestCheckIn_InvalidReservationID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	srv := newServer(mockpresence.NewMockPresenceUsecase(ctrl))

	_, err := srv.CheckIn(context.Background(), &pb.CheckInRequest{
		ReservationId: "not-a-uuid",
		DriverId:      validDriverID,
	})

	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, grpcCode(err))
	assert.Contains(t, status.Convert(err).Message(), "reservation_id")
}

func TestCheckIn_InvalidDriverID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	srv := newServer(mockpresence.NewMockPresenceUsecase(ctrl))

	_, err := srv.CheckIn(context.Background(), &pb.CheckInRequest{
		ReservationId: validReservationID,
		DriverId:      "bad-driver",
	})

	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, grpcCode(err))
	assert.Contains(t, status.Convert(err).Message(), "driver_id")
}

// ── CheckIn — usecase error mapping ──────────────────────────────────────────

func TestCheckIn_NotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc := mockpresence.NewMockPresenceUsecase(ctrl)
	uc.EXPECT().CheckIn(gomock.Any(), gomock.Any()).
		Return(nil, apperror.New("not_found", "reservation not found"))

	_, err := newServer(uc).CheckIn(context.Background(), &pb.CheckInRequest{
		ReservationId: validReservationID,
		DriverId:      validDriverID,
	})

	require.Error(t, err)
	assert.Equal(t, codes.NotFound, grpcCode(err))
}

func TestCheckIn_Conflict(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc := mockpresence.NewMockPresenceUsecase(ctrl)
	uc.EXPECT().CheckIn(gomock.Any(), gomock.Any()).
		Return(nil, apperror.New("conflict", "reservation is not in CONFIRMED status"))

	_, err := newServer(uc).CheckIn(context.Background(), &pb.CheckInRequest{
		ReservationId: validReservationID,
		DriverId:      validDriverID,
	})

	require.Error(t, err)
	assert.Equal(t, codes.FailedPrecondition, grpcCode(err))
}

func TestCheckIn_InternalError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc := mockpresence.NewMockPresenceUsecase(ctrl)
	uc.EXPECT().CheckIn(gomock.Any(), gomock.Any()).
		Return(nil, apperror.New("db_error", "failed to create session"))

	_, err := newServer(uc).CheckIn(context.Background(), &pb.CheckInRequest{
		ReservationId: validReservationID,
		DriverId:      validDriverID,
	})

	require.Error(t, err)
	assert.Equal(t, codes.Internal, grpcCode(err))
}

// ── CheckIn — success ─────────────────────────────────────────────────────────

func TestCheckIn_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	now := time.Now()

	uc := mockpresence.NewMockPresenceUsecase(ctrl)
	uc.EXPECT().CheckIn(gomock.Any(), gomock.Any()).
		Return(&model.CheckInResponse{
			SessionID:   validSessionID,
			CheckedInAt: now,
			Status:      model.SessionStatusActive,
		}, nil)

	res, err := newServer(uc).CheckIn(context.Background(), &pb.CheckInRequest{
		ReservationId: validReservationID,
		DriverId:      validDriverID,
		CheckedInAt:   timestamppb.New(now),
	})

	require.NoError(t, err)
	assert.Equal(t, validSessionID, res.SessionId)
	assert.Equal(t, pb.SessionStatus_SESSION_STATUS_ACTIVE, res.Status)
	assert.NotNil(t, res.CheckedInAt)
}

// ── CheckOut — validation ─────────────────────────────────────────────────────

func TestCheckOut_InvalidSessionID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	srv := newServer(mockpresence.NewMockPresenceUsecase(ctrl))

	_, err := srv.CheckOut(context.Background(), &pb.CheckOutRequest{
		SessionId: "not-a-uuid",
		DriverId:  validDriverID,
	})

	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, grpcCode(err))
	assert.Contains(t, status.Convert(err).Message(), "session_id")
}

func TestCheckOut_InvalidDriverID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	srv := newServer(mockpresence.NewMockPresenceUsecase(ctrl))

	_, err := srv.CheckOut(context.Background(), &pb.CheckOutRequest{
		SessionId: validSessionID,
		DriverId:  "bad-driver",
	})

	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, grpcCode(err))
}

// ── CheckOut — usecase error mapping ─────────────────────────────────────────

func TestCheckOut_NotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc := mockpresence.NewMockPresenceUsecase(ctrl)
	uc.EXPECT().CheckOut(gomock.Any(), gomock.Any()).
		Return(nil, apperror.New("not_found", "session not found"))

	_, err := newServer(uc).CheckOut(context.Background(), &pb.CheckOutRequest{
		SessionId: validSessionID,
		DriverId:  validDriverID,
	})

	require.Error(t, err)
	assert.Equal(t, codes.NotFound, grpcCode(err))
}

func TestCheckOut_Conflict(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc := mockpresence.NewMockPresenceUsecase(ctrl)
	uc.EXPECT().CheckOut(gomock.Any(), gomock.Any()).
		Return(nil, apperror.New("conflict", "session is not in ACTIVE status"))

	_, err := newServer(uc).CheckOut(context.Background(), &pb.CheckOutRequest{
		SessionId: validSessionID,
		DriverId:  validDriverID,
	})

	require.Error(t, err)
	assert.Equal(t, codes.FailedPrecondition, grpcCode(err))
}

// ── CheckOut — success ────────────────────────────────────────────────────────

func TestCheckOut_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	now := time.Now()

	uc := mockpresence.NewMockPresenceUsecase(ctrl)
	uc.EXPECT().CheckOut(gomock.Any(), gomock.Any()).
		Return(&model.CheckOutResponse{
			SessionID:    validSessionID,
			InvoiceID:    "inv-001",
			CheckedOutAt: now,
			Status:       model.SessionStatusCompleted,
			TotalIDR:     15000,
			QRCodeURL:    "https://qr.example.com/stub",
		}, nil)

	res, err := newServer(uc).CheckOut(context.Background(), &pb.CheckOutRequest{
		SessionId:    validSessionID,
		DriverId:     validDriverID,
		CheckedOutAt: timestamppb.New(now),
	})

	require.NoError(t, err)
	assert.Equal(t, validSessionID, res.SessionId)
	assert.Equal(t, "inv-001", res.InvoiceId)
	assert.Equal(t, int64(15000), res.TotalIdr)
	assert.Equal(t, "https://qr.example.com/stub", res.QrCodeUrl)
	assert.Equal(t, pb.SessionStatus_SESSION_STATUS_COMPLETED, res.Status)
}

// ── GetSession — validation ───────────────────────────────────────────────────

func TestGetSession_InvalidSessionID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	srv := newServer(mockpresence.NewMockPresenceUsecase(ctrl))

	_, err := srv.GetSession(context.Background(), &pb.GetSessionRequest{
		SessionId: "not-a-uuid",
	})

	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, grpcCode(err))
}

// ── GetSession — usecase error mapping ───────────────────────────────────────

func TestGetSession_NotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc := mockpresence.NewMockPresenceUsecase(ctrl)
	uc.EXPECT().GetSession(gomock.Any(), validSessionID).
		Return(nil, apperror.New("not_found", "session not found"))

	_, err := newServer(uc).GetSession(context.Background(), &pb.GetSessionRequest{
		SessionId: validSessionID,
	})

	require.Error(t, err)
	assert.Equal(t, codes.NotFound, grpcCode(err))
}

// ── GetSession — success ──────────────────────────────────────────────────────

func TestGetSession_Success_Active(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	now := time.Now()

	uc := mockpresence.NewMockPresenceUsecase(ctrl)
	uc.EXPECT().GetSession(gomock.Any(), validSessionID).
		Return(&model.GetSessionResponse{
			SessionID:     validSessionID,
			ReservationID: validReservationID,
			DriverID:      validDriverID,
			SpotID:        "spot-1",
			Status:        model.SessionStatusActive,
			CheckedInAt:   now,
			CheckedOutAt:  nil,
		}, nil)

	res, err := newServer(uc).GetSession(context.Background(), &pb.GetSessionRequest{
		SessionId: validSessionID,
	})

	require.NoError(t, err)
	assert.Equal(t, validSessionID, res.SessionId)
	assert.Equal(t, validReservationID, res.ReservationId)
	assert.Equal(t, pb.SessionStatus_SESSION_STATUS_ACTIVE, res.Status)
	assert.Nil(t, res.CheckedOutAt)
}

func TestGetSession_Success_Completed(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	now := time.Now()
	checkedOut := now.Add(2 * time.Hour)

	uc := mockpresence.NewMockPresenceUsecase(ctrl)
	uc.EXPECT().GetSession(gomock.Any(), validSessionID).
		Return(&model.GetSessionResponse{
			SessionID:    validSessionID,
			Status:       model.SessionStatusCompleted,
			CheckedInAt:  now,
			CheckedOutAt: &checkedOut,
		}, nil)

	res, err := newServer(uc).GetSession(context.Background(), &pb.GetSessionRequest{
		SessionId: validSessionID,
	})

	require.NoError(t, err)
	assert.Equal(t, pb.SessionStatus_SESSION_STATUS_COMPLETED, res.Status)
	assert.NotNil(t, res.CheckedOutAt)
}

// ── Additional internal error paths ──────────────────────────────────────────

func TestCheckOut_InternalError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc := mockpresence.NewMockPresenceUsecase(ctrl)
	uc.EXPECT().CheckOut(gomock.Any(), gomock.Any()).
		Return(nil, apperror.New("db_error", "failed to complete checkout"))

	_, err := newServer(uc).CheckOut(context.Background(), &pb.CheckOutRequest{
		SessionId: validSessionID,
		DriverId:  validDriverID,
	})

	require.Error(t, err)
	assert.Equal(t, codes.Internal, grpcCode(err))
}

func TestGetSession_InternalError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc := mockpresence.NewMockPresenceUsecase(ctrl)
	uc.EXPECT().GetSession(gomock.Any(), validSessionID).
		Return(nil, apperror.New("db_error", "failed to query session"))

	_, err := newServer(uc).GetSession(context.Background(), &pb.GetSessionRequest{
		SessionId: validSessionID,
	})

	require.Error(t, err)
	assert.Equal(t, codes.Internal, grpcCode(err))
}
