//go:build e2e

package e2e_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"

	billingpb "parkir-pintar/services/billing/gen/billing/v1"
	presencev1 "parkir-pintar/services/presence/gen/presence/v1"
	reservationpb "parkir-pintar/services/reservation/gen/reservation/v1"
	searchpb "parkir-pintar/services/search/gen/search/v1"
)

// TestE2E_FullHappyPath exercises the complete parking flow end-to-end:
//
//  1. Driver checks availability (Search)
//  2. Driver creates a reservation (Reservation)
//  3. Booking fee payment succeeds via webhook → reservation CONFIRMED
//  4. Driver checks in (Presence)
//  5. Driver checks out (Presence) → invoice created
//  6. Parking fee payment succeeds via webhook → invoice PAID
//  7. Invoice is verified as PAID (Billing)
func TestE2E_FullHappyPath(t *testing.T) {
	s := NewSuite(t)
	driverID := uuid.New().String()

	// ── Step 1: Check availability ────────────────────────────────
	avail, err := s.Search.GetAvailability(context.Background(), &searchpb.GetAvailabilityRequest{
		VehicleType: searchpb.VehicleType_VEHICLE_TYPE_CAR,
	})
	require.NoError(t, err)
	assert.Greater(t, avail.TotalAvailable, int32(0), "parking lot must have available spots")

	// ── Step 2: Create reservation ────────────────────────────────
	createRes, err := s.Reservation.CreateReservation(context.Background(), &reservationpb.CreateReservationRequest{
		IdempotencyKey: uuid.New().String(),
		DriverId:       driverID,
		VehicleType:    reservationpb.VehicleType_VEHICLE_TYPE_CAR,
		Mode:           reservationpb.AssignmentMode_ASSIGNMENT_MODE_SYSTEM_ASSIGNED,
	})
	require.NoError(t, err)
	assert.NotEmpty(t, createRes.ReservationId)
	assert.NotEmpty(t, createRes.SpotId)
	assert.NotEmpty(t, createRes.QrCodeUrl)
	assert.Equal(t, reservationpb.ReservationStatus_RESERVATION_STATUS_PENDING_PAYMENT, createRes.Status)
	reservationID := createRes.ReservationId

	// ── Step 3: Booking fee payment succeeds ──────────────────────
	s.triggerWebhookCallback(t, reservationID, "BOOKING_FEE", "SUCCESS", 5000)
	waitForReservationStatus(t, s, reservationID, reservationpb.ReservationStatus_RESERVATION_STATUS_CONFIRMED, 10*time.Second)

	// Verify reservation fields after confirmation.
	reservation, err := s.Reservation.GetReservation(context.Background(), &reservationpb.GetReservationRequest{
		ReservationId: reservationID,
	})
	require.NoError(t, err)
	assert.Equal(t, reservationpb.ReservationStatus_RESERVATION_STATUS_CONFIRMED, reservation.Status)
	assert.NotNil(t, reservation.ConfirmedAt)
	assert.NotNil(t, reservation.ExpiresAt)

	// ── Step 4: Check in ──────────────────────────────────────────
	checkedInAt := time.Now()
	checkInRes, err := s.Presence.CheckIn(context.Background(), &presencev1.CheckInRequest{
		ReservationId: reservationID,
		DriverId:      driverID,
		CheckedInAt:   timestamppb.New(checkedInAt),
	})
	require.NoError(t, err)
	assert.NotEmpty(t, checkInRes.SessionId)
	assert.Equal(t, presencev1.SessionStatus_SESSION_STATUS_ACTIVE, checkInRes.Status)
	sessionID := checkInRes.SessionId

	// Verify reservation transitions to CHECKED_IN.
	waitForReservationStatus(t, s, reservationID, reservationpb.ReservationStatus_RESERVATION_STATUS_CHECKED_IN, 10*time.Second)

	// ── Step 5: Check out (2 hours later) ─────────────────────────
	checkedOutAt := checkedInAt.Add(2 * time.Hour)
	checkOutRes, err := s.Presence.CheckOut(context.Background(), &presencev1.CheckOutRequest{
		SessionId:    sessionID,
		DriverId:     driverID,
		CheckedOutAt: timestamppb.New(checkedOutAt),
	})
	require.NoError(t, err)
	assert.NotEmpty(t, checkOutRes.InvoiceId)
	assert.Greater(t, checkOutRes.TotalIdr, int64(0))
	assert.NotEmpty(t, checkOutRes.QrCodeUrl)
	assert.Equal(t, presencev1.SessionStatus_SESSION_STATUS_COMPLETED, checkOutRes.Status)
	invoiceID := checkOutRes.InvoiceId

	// Verify the invoice is initially PENDING_PAYMENT with positive fees.
	// Exact fee arithmetic is covered by billing unit tests; here we only
	// assert structural correctness and that the amounts are non-zero.
	invoice, err := s.Billing.GetInvoice(context.Background(), &billingpb.GetInvoiceRequest{
		InvoiceId: invoiceID,
	})
	require.NoError(t, err)
	assert.Equal(t, billingpb.InvoiceStatus_INVOICE_STATUS_PENDING_PAYMENT, invoice.Status)
	assert.Greater(t, invoice.ParkingFeeIdr, int64(0))
	assert.Equal(t, int64(5000), invoice.BookingFeeIdr)
	assert.Equal(t, int64(0), invoice.OvernightFeeIdr)
	assert.Greater(t, invoice.TotalIdr, int64(0))

	// ── Step 6: Parking fee payment succeeds ─────────────────────
	s.triggerWebhookCallback(t, invoiceID, "PARKING_FEE", "SUCCESS", invoice.TotalIdr)

	// ── Step 7: Invoice is PAID ───────────────────────────────────
	waitForInvoiceStatus(t, s, invoiceID, billingpb.InvoiceStatus_INVOICE_STATUS_PAID, 10*time.Second)

	// Verify reservation is COMPLETED.
	waitForReservationStatus(t, s, reservationID, reservationpb.ReservationStatus_RESERVATION_STATUS_COMPLETED, 10*time.Second)
}

// TestE2E_FullHappyPath_Overnight verifies the overnight fee is applied when
// the parking session crosses midnight.
func TestE2E_FullHappyPath_Overnight(t *testing.T) {
	s := NewSuite(t)
	driverID := uuid.New().String()

	// Create and confirm a reservation.
	createRes, err := s.Reservation.CreateReservation(context.Background(), &reservationpb.CreateReservationRequest{
		IdempotencyKey: uuid.New().String(),
		DriverId:       driverID,
		VehicleType:    reservationpb.VehicleType_VEHICLE_TYPE_CAR,
		Mode:           reservationpb.AssignmentMode_ASSIGNMENT_MODE_SYSTEM_ASSIGNED,
	})
	require.NoError(t, err)
	reservationID := createRes.ReservationId

	s.triggerWebhookCallback(t, reservationID, "BOOKING_FEE", "SUCCESS", 5000)
	waitForReservationStatus(t, s, reservationID, reservationpb.ReservationStatus_RESERVATION_STATUS_CONFIRMED, 10*time.Second)

	// Check in at 23:00 UTC yesterday, check out at 02:00 UTC today — crosses
	// exactly one UTC midnight regardless of the test machine's local timezone.
	now := time.Now().UTC()
	utcMidnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	checkedInAt := utcMidnight.Add(-1 * time.Hour)  // 23:00 UTC yesterday
	checkedOutAt := utcMidnight.Add(2 * time.Hour)  // 02:00 UTC today

	checkInRes, err := s.Presence.CheckIn(context.Background(), &presencev1.CheckInRequest{
		ReservationId: reservationID,
		DriverId:      driverID,
		CheckedInAt:   timestamppb.New(checkedInAt),
	})
	require.NoError(t, err)

	checkOutRes, err := s.Presence.CheckOut(context.Background(), &presencev1.CheckOutRequest{
		SessionId:    checkInRes.SessionId,
		DriverId:     driverID,
		CheckedOutAt: timestamppb.New(checkedOutAt),
	})
	require.NoError(t, err)

	invoice, err := s.Billing.GetInvoice(context.Background(), &billingpb.GetInvoiceRequest{
		InvoiceId: checkOutRes.InvoiceId,
	})
	require.NoError(t, err)
	// 3 hours (23:00→02:00, ceil) × 5000 = 15000 parking fee + 20000 overnight fee
	assert.Equal(t, int64(15000), invoice.ParkingFeeIdr, "3 started hours × 5000 IDR")
	assert.Equal(t, int64(20000), invoice.OvernightFeeIdr, "one midnight crossed = 20000 IDR overnight fee")
	assert.Equal(t, int64(35000), invoice.TotalIdr, "15000 parking + 20000 overnight")
}

// TestE2E_BookingPaymentFailed_ReservationCancelled verifies that a FAILED
// booking-fee payment transitions the reservation to CANCELLED and releases
// the spot back to inventory.
func TestE2E_BookingPaymentFailed_ReservationCancelled(t *testing.T) {
	s := NewSuite(t)

	before, err := s.Search.GetAvailability(context.Background(), &searchpb.GetAvailabilityRequest{
		VehicleType: searchpb.VehicleType_VEHICLE_TYPE_CAR,
	})
	require.NoError(t, err)
	totalBefore := before.TotalAvailable

	createRes, err := s.Reservation.CreateReservation(context.Background(), &reservationpb.CreateReservationRequest{
		IdempotencyKey: uuid.New().String(),
		DriverId:       uuid.New().String(),
		VehicleType:    reservationpb.VehicleType_VEHICLE_TYPE_CAR,
		Mode:           reservationpb.AssignmentMode_ASSIGNMENT_MODE_SYSTEM_ASSIGNED,
	})
	require.NoError(t, err)

	// Booking fee payment fails.
	s.triggerWebhookCallback(t, createRes.ReservationId, "BOOKING_FEE", "FAILED", 5000)

	// Reservation must be CANCELLED.
	waitForReservationStatus(t, s, createRes.ReservationId, reservationpb.ReservationStatus_RESERVATION_STATUS_CANCELLED, 10*time.Second)

	// Spot must be released — availability should return to the original count.
	after, err := s.Search.GetAvailability(context.Background(), &searchpb.GetAvailabilityRequest{
		VehicleType: searchpb.VehicleType_VEHICLE_TYPE_CAR,
	})
	require.NoError(t, err)
	assert.Equal(t, totalBefore, after.TotalAvailable,
		"spot should be released back to inventory after booking payment failure")
}

// TestE2E_ParkingPaymentFailed_InvoicePaymentFailed_RetryPayment verifies the
// retry payment flow: parking fee fails → invoice PAYMENT_FAILED →
// RetryPayment issues a new QR code → second payment succeeds → invoice PAID.
func TestE2E_ParkingPaymentFailed_InvoicePaymentFailed_RetryPayment(t *testing.T) {
	s := NewSuite(t)
	driverID := uuid.New().String()

	// Create and confirm reservation.
	createRes, err := s.Reservation.CreateReservation(context.Background(), &reservationpb.CreateReservationRequest{
		IdempotencyKey: uuid.New().String(),
		DriverId:       driverID,
		VehicleType:    reservationpb.VehicleType_VEHICLE_TYPE_CAR,
		Mode:           reservationpb.AssignmentMode_ASSIGNMENT_MODE_SYSTEM_ASSIGNED,
	})
	require.NoError(t, err)

	s.triggerWebhookCallback(t, createRes.ReservationId, "BOOKING_FEE", "SUCCESS", 5000)
	waitForReservationStatus(t, s, createRes.ReservationId, reservationpb.ReservationStatus_RESERVATION_STATUS_CONFIRMED, 10*time.Second)

	// Check in and check out.
	checkedInAt := time.Now().Add(-1 * time.Hour)
	checkInRes, err := s.Presence.CheckIn(context.Background(), &presencev1.CheckInRequest{
		ReservationId: createRes.ReservationId,
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
	invoiceID := checkOutRes.InvoiceId

	// Parking fee payment fails.
	s.triggerWebhookCallback(t, invoiceID, "PARKING_FEE", "FAILED", checkOutRes.TotalIdr)
	waitForInvoiceStatus(t, s, invoiceID, billingpb.InvoiceStatus_INVOICE_STATUS_PAYMENT_FAILED, 10*time.Second)

	// Driver retries payment — should get a new QR code.
	retryRes, err := s.Billing.RetryPayment(context.Background(), &billingpb.RetryPaymentRequest{
		InvoiceId: invoiceID,
		DriverId:  driverID,
	})
	require.NoError(t, err)
	assert.NotEmpty(t, retryRes.PaymentId)
	assert.NotEmpty(t, retryRes.QrCodeUrl)

	// Invoice should be back to PENDING_PAYMENT after retry.
	invoice, err := s.Billing.GetInvoice(context.Background(), &billingpb.GetInvoiceRequest{
		InvoiceId: invoiceID,
	})
	require.NoError(t, err)
	assert.Equal(t, billingpb.InvoiceStatus_INVOICE_STATUS_PENDING_PAYMENT, invoice.Status)

	// Second payment attempt succeeds.
	s.triggerWebhookCallback(t, invoiceID, "PARKING_FEE", "SUCCESS", checkOutRes.TotalIdr)
	waitForInvoiceStatus(t, s, invoiceID, billingpb.InvoiceStatus_INVOICE_STATUS_PAID, 10*time.Second)
}
