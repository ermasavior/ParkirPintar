package usecase

import (
	"context"
	"testing"
	"time"

	mockpresence "parkir-pintar/services/presence/_mock/presence"
	mockbillingclient "parkir-pintar/services/presence/_mock/pkg/billingclient"
	"parkir-pintar/services/presence/internal/presence/model"
	"parkir-pintar/services/presence/pkg/apperror"
	"parkir-pintar/services/presence/pkg/billingclient"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

const (
	testSessionID     = "110e8400-e29b-41d4-a716-446655440001"
	testReservationID = "220e8400-e29b-41d4-a716-446655440002"
	testDriverID      = "330e8400-e29b-41d4-a716-446655440003"
	testSpotID        = "440e8400-e29b-41d4-a716-446655440004"
)

func newUsecase(repo *mockpresence.MockPresenceRepository, ctrl *gomock.Controller) *PresenceUsecase {
	bc := mockbillingclient.NewMockBillingService(ctrl)
	bc.EXPECT().CalculateAndCreateInvoice(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(&billingclient.CreateInvoiceResult{
			InvoiceID: "stub-invoice-id",
			TotalIDR:  15000,
			QRCodeURL: "https://qr.example.com/stub",
		}, nil).AnyTimes()
	return &PresenceUsecase{repo: repo, billingClient: bc}
}

func validCheckInReq() model.CheckInRequest {
	return model.CheckInRequest{
		ReservationID: testReservationID,
		DriverID:      testDriverID,
		CheckedInAt:   time.Now(),
	}
}

func validCheckOutReq() model.CheckOutRequest {
	return model.CheckOutRequest{
		SessionID:    testSessionID,
		DriverID:     testDriverID,
		CheckedOutAt: time.Now(),
	}
}

// ── CheckIn ───────────────────────────────────────────────────────────────────

func TestCheckIn_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	expiresAt := time.Now().Add(1 * time.Hour)
	now := time.Now()

	repo := mockpresence.NewMockPresenceRepository(ctrl)
	repo.EXPECT().GetReservationForCheckIn(gomock.Any(), testReservationID, testDriverID).
		Return(&model.Reservation{
			ID:        testReservationID,
			DriverID:  testDriverID,
			SpotID:    testSpotID,
			Status:    model.ReservationStatusConfirmed,
			ExpiresAt: &expiresAt,
		}, nil)
	repo.EXPECT().CreateSessionAndMarkCheckedIn(gomock.Any(), testReservationID, testDriverID, testSpotID, gomock.Any()).
		Return(&model.Session{
			ID:            testSessionID,
			ReservationID: testReservationID,
			DriverID:      testDriverID,
			SpotID:        testSpotID,
			Status:        model.SessionStatusActive,
			CheckedInAt:   now,
		}, nil)

	res, appErr := newUsecase(repo, ctrl).CheckIn(context.Background(), validCheckInReq())

	require.Nil(t, appErr)
	assert.Equal(t, testSessionID, res.SessionID)
	assert.Equal(t, model.SessionStatusActive, res.Status)
}

func TestCheckIn_ReservationNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mockpresence.NewMockPresenceRepository(ctrl)
	repo.EXPECT().GetReservationForCheckIn(gomock.Any(), testReservationID, testDriverID).
		Return(nil, apperror.New("not_found", "reservation not found or does not belong to driver"))

	_, appErr := newUsecase(repo, ctrl).CheckIn(context.Background(), validCheckInReq())

	require.NotNil(t, appErr)
	assert.Equal(t, "not_found", appErr.ErrorCode)
}

func TestCheckIn_ReservationNotConfirmed(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mockpresence.NewMockPresenceRepository(ctrl)
	repo.EXPECT().GetReservationForCheckIn(gomock.Any(), testReservationID, testDriverID).
		Return(&model.Reservation{
			ID:       testReservationID,
			DriverID: testDriverID,
			SpotID:   testSpotID,
			Status:   model.ReservationStatusCheckedIn, // already checked in
		}, nil)

	_, appErr := newUsecase(repo, ctrl).CheckIn(context.Background(), validCheckInReq())

	require.NotNil(t, appErr)
	assert.Equal(t, "conflict", appErr.ErrorCode)
	assert.Contains(t, appErr.Message, "booking fee")
}

func TestCheckIn_ReservationExpired(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	expiredAt := time.Now().Add(-1 * time.Hour) // in the past

	repo := mockpresence.NewMockPresenceRepository(ctrl)
	repo.EXPECT().GetReservationForCheckIn(gomock.Any(), testReservationID, testDriverID).
		Return(&model.Reservation{
			ID:        testReservationID,
			DriverID:  testDriverID,
			SpotID:    testSpotID,
			Status:    model.ReservationStatusConfirmed,
			ExpiresAt: &expiredAt,
		}, nil)

	_, appErr := newUsecase(repo, ctrl).CheckIn(context.Background(), validCheckInReq())

	require.NotNil(t, appErr)
	assert.Equal(t, "conflict", appErr.ErrorCode)
	assert.Contains(t, appErr.Message, "expired")
}

func TestCheckIn_CreateSessionFails(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	expiresAt := time.Now().Add(1 * time.Hour)

	repo := mockpresence.NewMockPresenceRepository(ctrl)
	repo.EXPECT().GetReservationForCheckIn(gomock.Any(), testReservationID, testDriverID).
		Return(&model.Reservation{
			ID:        testReservationID,
			DriverID:  testDriverID,
			SpotID:    testSpotID,
			Status:    model.ReservationStatusConfirmed,
			ExpiresAt: &expiresAt,
		}, nil)
	repo.EXPECT().CreateSessionAndMarkCheckedIn(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, apperror.New("db_error", "failed to create session"))

	_, appErr := newUsecase(repo, ctrl).CheckIn(context.Background(), validCheckInReq())

	require.NotNil(t, appErr)
	assert.Equal(t, "db_error", appErr.ErrorCode)
}

// ── CheckOut ──────────────────────────────────────────────────────────────────

func TestCheckOut_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	now := time.Now()

	repo := mockpresence.NewMockPresenceRepository(ctrl)
	repo.EXPECT().GetSessionForCheckOut(gomock.Any(), testSessionID, testDriverID).
		Return(&model.Session{
			ID:            testSessionID,
			ReservationID: testReservationID,
			DriverID:      testDriverID,
			SpotID:        testSpotID,
			Status:        model.SessionStatusActive,
			CheckedInAt:   now.Add(-2 * time.Hour),
		}, nil)
	repo.EXPECT().CompleteCheckOut(gomock.Any(), testSessionID, testSpotID, testReservationID, gomock.Any()).
		Return(nil)

	res, appErr := newUsecase(repo, ctrl).CheckOut(context.Background(), validCheckOutReq())

	require.Nil(t, appErr)
	assert.Equal(t, testSessionID, res.SessionID)
	assert.Equal(t, model.SessionStatusCompleted, res.Status)
	assert.NotEmpty(t, res.InvoiceID)
	assert.NotEmpty(t, res.QRCodeURL)
}

func TestCheckOut_SessionNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mockpresence.NewMockPresenceRepository(ctrl)
	repo.EXPECT().GetSessionForCheckOut(gomock.Any(), testSessionID, testDriverID).
		Return(nil, apperror.New("not_found", "session not found or does not belong to driver"))

	_, appErr := newUsecase(repo, ctrl).CheckOut(context.Background(), validCheckOutReq())

	require.NotNil(t, appErr)
	assert.Equal(t, "not_found", appErr.ErrorCode)
}

func TestCheckOut_SessionNotActive(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mockpresence.NewMockPresenceRepository(ctrl)
	repo.EXPECT().GetSessionForCheckOut(gomock.Any(), testSessionID, testDriverID).
		Return(&model.Session{
			ID:     testSessionID,
			Status: model.SessionStatusCompleted, // already checked out
		}, nil)

	_, appErr := newUsecase(repo, ctrl).CheckOut(context.Background(), validCheckOutReq())

	require.NotNil(t, appErr)
	assert.Equal(t, "conflict", appErr.ErrorCode)
	assert.Contains(t, appErr.Message, "ACTIVE")
}

func TestCheckOut_CompleteCheckOutFails(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	now := time.Now()

	repo := mockpresence.NewMockPresenceRepository(ctrl)
	repo.EXPECT().GetSessionForCheckOut(gomock.Any(), testSessionID, testDriverID).
		Return(&model.Session{
			ID:            testSessionID,
			ReservationID: testReservationID,
			SpotID:        testSpotID,
			Status:        model.SessionStatusActive,
			CheckedInAt:   now.Add(-1 * time.Hour),
		}, nil)
	repo.EXPECT().CompleteCheckOut(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(apperror.New("db_error", "failed to update session"))

	_, appErr := newUsecase(repo, ctrl).CheckOut(context.Background(), validCheckOutReq())

	require.NotNil(t, appErr)
	assert.Equal(t, "db_error", appErr.ErrorCode)
}

// ── GetSession ────────────────────────────────────────────────────────────────

func TestGetSession_Success_Active(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	now := time.Now()

	repo := mockpresence.NewMockPresenceRepository(ctrl)
	repo.EXPECT().GetSessionByID(gomock.Any(), testSessionID).
		Return(&model.Session{
			ID:            testSessionID,
			ReservationID: testReservationID,
			DriverID:      testDriverID,
			SpotID:        testSpotID,
			Status:        model.SessionStatusActive,
			CheckedInAt:   now,
			CheckedOutAt:  nil,
		}, nil)

	res, appErr := newUsecase(repo, ctrl).GetSession(context.Background(), testSessionID)

	require.Nil(t, appErr)
	assert.Equal(t, testSessionID, res.SessionID)
	assert.Equal(t, testReservationID, res.ReservationID)
	assert.Equal(t, model.SessionStatusActive, res.Status)
	assert.Nil(t, res.CheckedOutAt)
}

func TestGetSession_Success_Completed(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	now := time.Now()
	checkedOut := now.Add(2 * time.Hour)

	repo := mockpresence.NewMockPresenceRepository(ctrl)
	repo.EXPECT().GetSessionByID(gomock.Any(), testSessionID).
		Return(&model.Session{
			ID:            testSessionID,
			ReservationID: testReservationID,
			DriverID:      testDriverID,
			SpotID:        testSpotID,
			Status:        model.SessionStatusCompleted,
			CheckedInAt:   now,
			CheckedOutAt:  &checkedOut,
		}, nil)

	res, appErr := newUsecase(repo, ctrl).GetSession(context.Background(), testSessionID)

	require.Nil(t, appErr)
	assert.Equal(t, model.SessionStatusCompleted, res.Status)
	assert.NotNil(t, res.CheckedOutAt)
}

func TestGetSession_NotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mockpresence.NewMockPresenceRepository(ctrl)
	repo.EXPECT().GetSessionByID(gomock.Any(), testSessionID).
		Return(nil, apperror.New("not_found", "session not found"))

	_, appErr := newUsecase(repo, ctrl).GetSession(context.Background(), testSessionID)

	require.NotNil(t, appErr)
	assert.Equal(t, "not_found", appErr.ErrorCode)
}

func TestGetSession_DBError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mockpresence.NewMockPresenceRepository(ctrl)
	repo.EXPECT().GetSessionByID(gomock.Any(), testSessionID).
		Return(nil, apperror.New("db_error", "failed to query session"))

	_, appErr := newUsecase(repo, ctrl).GetSession(context.Background(), testSessionID)

	require.NotNil(t, appErr)
	assert.Equal(t, "db_error", appErr.ErrorCode)
}

// ── Zero-time fallback paths ──────────────────────────────────────────────────

func TestCheckIn_ZeroCheckedInAt_UsesNow(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	expiresAt := time.Now().Add(1 * time.Hour)

	repo := mockpresence.NewMockPresenceRepository(ctrl)
	repo.EXPECT().GetReservationForCheckIn(gomock.Any(), testReservationID, testDriverID).
		Return(&model.Reservation{
			ID:        testReservationID,
			DriverID:  testDriverID,
			SpotID:    testSpotID,
			Status:    model.ReservationStatusConfirmed,
			ExpiresAt: &expiresAt,
		}, nil)
	repo.EXPECT().CreateSessionAndMarkCheckedIn(gomock.Any(), testReservationID, testDriverID, testSpotID, gomock.Any()).
		Return(&model.Session{
			ID:          testSessionID,
			Status:      model.SessionStatusActive,
			CheckedInAt: time.Now(),
		}, nil)

	res, appErr := newUsecase(repo, ctrl).CheckIn(context.Background(), model.CheckInRequest{
		ReservationID: testReservationID,
		DriverID:      testDriverID,
		CheckedInAt:   time.Time{}, // zero — falls back to time.Now()
	})

	require.Nil(t, appErr)
	assert.Equal(t, testSessionID, res.SessionID)
}

func TestCheckOut_ZeroCheckedOutAt_UsesNow(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	now := time.Now()

	repo := mockpresence.NewMockPresenceRepository(ctrl)
	repo.EXPECT().GetSessionForCheckOut(gomock.Any(), testSessionID, testDriverID).
		Return(&model.Session{
			ID:            testSessionID,
			ReservationID: testReservationID,
			SpotID:        testSpotID,
			Status:        model.SessionStatusActive,
			CheckedInAt:   now.Add(-1 * time.Hour),
		}, nil)
	repo.EXPECT().CompleteCheckOut(gomock.Any(), testSessionID, testSpotID, testReservationID, gomock.Any()).
		Return(nil)

	res, appErr := newUsecase(repo, ctrl).CheckOut(context.Background(), model.CheckOutRequest{
		SessionID:    testSessionID,
		DriverID:     testDriverID,
		CheckedOutAt: time.Time{}, // zero — falls back to time.Now()
	})

	require.Nil(t, appErr)
	assert.Equal(t, testSessionID, res.SessionID)
}
