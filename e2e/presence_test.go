//go:build e2e

package e2e_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	presencev1 "parkir-pintar/services/presence/gen/presence/v1"
	reservationpb "parkir-pintar/services/reservation/gen/reservation/v1"
)

// ── Presence tests ────────────────────────────────────────────────

// confirmReservation creates a reservation and drives it to CONFIRMED status
// by triggering a successful booking-fee webhook callback.
// Returns the reservation_id and driver_id.
func confirmReservation(t *testing.T, s *Suite) (reservationID, driverID string) {
	t.Helper()
	driverID = uuid.New().String()

	res, err := s.Reservation.CreateReservation(context.Background(), &reservationpb.CreateReservationRequest{
		IdempotencyKey: uuid.New().String(),
		DriverId:       driverID,
		VehicleType:    reservationpb.VehicleType_VEHICLE_TYPE_CAR,
		Mode:           reservationpb.AssignmentMode_ASSIGNMENT_MODE_SYSTEM_ASSIGNED,
	})
	require.NoError(t, err)
	reservationID = res.ReservationId

	// Trigger the booking-fee payment success webhook so the reservation
	// transitions from PENDING_PAYMENT → CONFIRMED.
	s.triggerWebhookCallback(t, reservationID, "BOOKING_FEE", "SUCCESS", 5000)

	// Poll until the reservation reaches CONFIRMED status.
	waitForReservationStatus(t, s, reservationID, reservationpb.ReservationStatus_RESERVATION_STATUS_CONFIRMED, 10*time.Second)
	return reservationID, driverID
}

// TestE2E_CheckIn_Success verifies a driver can check in with a CONFIRMED
// reservation and receives an active session in return.
func TestE2E_CheckIn_Success(t *testing.T) {
	s := NewSuite(t)
	reservationID, driverID := confirmReservation(t, s)

	res, err := s.Presence.CheckIn(context.Background(), &presencev1.CheckInRequest{
		ReservationId: reservationID,
		DriverId:      driverID,
		CheckedInAt:   timestamppb.New(time.Now()),
	})

	require.NoError(t, err)
	assert.NotEmpty(t, res.SessionId)
	assert.Equal(t, presencev1.SessionStatus_SESSION_STATUS_ACTIVE, res.Status)
	assert.NotNil(t, res.CheckedInAt)
}

// TestE2E_CheckIn_UnconfirmedReservation verifies that checking in against a
// reservation that is still PENDING_PAYMENT is rejected.
func TestE2E_CheckIn_UnconfirmedReservation(t *testing.T) {
	s := NewSuite(t)

	// Create a reservation but do NOT confirm it (no webhook callback).
	createRes, err := s.Reservation.CreateReservation(context.Background(), &reservationpb.CreateReservationRequest{
		IdempotencyKey: uuid.New().String(),
		DriverId:       uuid.New().String(),
		VehicleType:    reservationpb.VehicleType_VEHICLE_TYPE_CAR,
		Mode:           reservationpb.AssignmentMode_ASSIGNMENT_MODE_SYSTEM_ASSIGNED,
	})
	require.NoError(t, err)

	_, err = s.Presence.CheckIn(context.Background(), &presencev1.CheckInRequest{
		ReservationId: createRes.ReservationId,
		DriverId:      uuid.New().String(),
		CheckedInAt:   timestamppb.New(time.Now()),
	})

	require.Error(t, err)
	code := status.Code(err)
	assert.True(t,
		code == codes.FailedPrecondition || code == codes.InvalidArgument || code == codes.NotFound,
		"expected a rejection error, got %v", code,
	)
}

// TestE2E_CheckIn_WrongDriver verifies that a driver cannot check in using
// another driver's reservation.
func TestE2E_CheckIn_WrongDriver(t *testing.T) {
	s := NewSuite(t)
	reservationID, _ := confirmReservation(t, s)

	_, err := s.Presence.CheckIn(context.Background(), &presencev1.CheckInRequest{
		ReservationId: reservationID,
		DriverId:      uuid.New().String(), // different driver
		CheckedInAt:   timestamppb.New(time.Now()),
	})

	require.Error(t, err)
	code := status.Code(err)
	assert.True(t,
		code == codes.PermissionDenied || code == codes.NotFound || code == codes.InvalidArgument,
		"expected a rejection error for wrong driver, got %v", code,
	)
}

// TestE2E_GetSession verifies that a session created by CheckIn can be fetched back.
func TestE2E_GetSession(t *testing.T) {
	s := NewSuite(t)
	reservationID, driverID := confirmReservation(t, s)

	checkInRes, err := s.Presence.CheckIn(context.Background(), &presencev1.CheckInRequest{
		ReservationId: reservationID,
		DriverId:      driverID,
		CheckedInAt:   timestamppb.New(time.Now()),
	})
	require.NoError(t, err)

	session, err := s.Presence.GetSession(context.Background(), &presencev1.GetSessionRequest{
		SessionId: checkInRes.SessionId,
	})

	require.NoError(t, err)
	assert.Equal(t, checkInRes.SessionId, session.SessionId)
	assert.Equal(t, reservationID, session.ReservationId)
	assert.Equal(t, driverID, session.DriverId)
	assert.Equal(t, presencev1.SessionStatus_SESSION_STATUS_ACTIVE, session.Status)
	assert.Nil(t, session.CheckedOutAt, "checked_out_at should be unset for an active session")
}

// TestE2E_CheckOut_Success verifies a driver can check out of an active session
// and receives an invoice with a positive total.
func TestE2E_CheckOut_Success(t *testing.T) {
	s := NewSuite(t)
	reservationID, driverID := confirmReservation(t, s)

	checkedInAt := time.Now().Add(-2 * time.Hour)
	checkInRes, err := s.Presence.CheckIn(context.Background(), &presencev1.CheckInRequest{
		ReservationId: reservationID,
		DriverId:      driverID,
		CheckedInAt:   timestamppb.New(checkedInAt),
	})
	require.NoError(t, err)

	checkOutRes, err := s.Presence.CheckOut(context.Background(), &presencev1.CheckOutRequest{
		SessionId:    checkInRes.SessionId,
		DriverId:     driverID,
		CheckedOutAt: timestamppb.New(time.Now()),
	})

	require.NoError(t, err)
	assert.Equal(t, checkInRes.SessionId, checkOutRes.SessionId)
	assert.NotEmpty(t, checkOutRes.InvoiceId)
	assert.Greater(t, checkOutRes.TotalIdr, int64(0))
	assert.NotEmpty(t, checkOutRes.QrCodeUrl)
	assert.Equal(t, presencev1.SessionStatus_SESSION_STATUS_COMPLETED, checkOutRes.Status)
}

// TestE2E_CheckOut_SessionCompleted verifies that checking out an already
// completed session is rejected.
func TestE2E_CheckOut_SessionCompleted(t *testing.T) {
	s := NewSuite(t)
	reservationID, driverID := confirmReservation(t, s)

	checkedInAt := time.Now().Add(-1 * time.Hour)
	checkInRes, err := s.Presence.CheckIn(context.Background(), &presencev1.CheckInRequest{
		ReservationId: reservationID,
		DriverId:      driverID,
		CheckedInAt:   timestamppb.New(checkedInAt),
	})
	require.NoError(t, err)

	// First checkout — should succeed.
	_, err = s.Presence.CheckOut(context.Background(), &presencev1.CheckOutRequest{
		SessionId:    checkInRes.SessionId,
		DriverId:     driverID,
		CheckedOutAt: timestamppb.New(time.Now()),
	})
	require.NoError(t, err)

	// Second checkout on the same session — must be rejected.
	_, err = s.Presence.CheckOut(context.Background(), &presencev1.CheckOutRequest{
		SessionId:    checkInRes.SessionId,
		DriverId:     driverID,
		CheckedOutAt: timestamppb.New(time.Now()),
	})
	require.Error(t, err)
	code := status.Code(err)
	assert.True(t,
		code == codes.FailedPrecondition || code == codes.AlreadyExists || code == codes.InvalidArgument,
		"expected rejection for double checkout, got %v", code,
	)
}

// TestE2E_GetSession_AfterCheckOut verifies that a completed session reflects
// the checked_out_at timestamp and COMPLETED status.
func TestE2E_GetSession_AfterCheckOut(t *testing.T) {
	s := NewSuite(t)
	reservationID, driverID := confirmReservation(t, s)

	checkedInAt := time.Now().Add(-90 * time.Minute)
	checkInRes, err := s.Presence.CheckIn(context.Background(), &presencev1.CheckInRequest{
		ReservationId: reservationID,
		DriverId:      driverID,
		CheckedInAt:   timestamppb.New(checkedInAt),
	})
	require.NoError(t, err)

	_, err = s.Presence.CheckOut(context.Background(), &presencev1.CheckOutRequest{
		SessionId:    checkInRes.SessionId,
		DriverId:     driverID,
		CheckedOutAt: timestamppb.New(time.Now()),
	})
	require.NoError(t, err)

	session, err := s.Presence.GetSession(context.Background(), &presencev1.GetSessionRequest{
		SessionId: checkInRes.SessionId,
	})

	require.NoError(t, err)
	assert.Equal(t, presencev1.SessionStatus_SESSION_STATUS_COMPLETED, session.Status)
	assert.NotNil(t, session.CheckedOutAt, "checked_out_at must be set after checkout")
}
