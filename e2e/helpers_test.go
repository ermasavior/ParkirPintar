//go:build e2e

package e2e_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	billingpb "parkir-pintar/services/billing/gen/billing/v1"
	reservationpb "parkir-pintar/services/reservation/gen/reservation/v1"
)

// mustNewReservationReq returns a fresh CreateReservationRequest with a unique
// idempotency key and driver ID. Used by tests that need a reservation but do
// not care about the specific driver identity.
func mustNewReservationReq() *reservationpb.CreateReservationRequest {
	return &reservationpb.CreateReservationRequest{
		IdempotencyKey: uuid.New().String(),
		DriverId:       uuid.New().String(),
		VehicleType:    reservationpb.VehicleType_VEHICLE_TYPE_CAR,
		Mode:           reservationpb.AssignmentMode_ASSIGNMENT_MODE_SYSTEM_ASSIGNED,
	}
}

// waitForReservationStatus polls the reservation service until the reservation
// reaches the expected status or the timeout expires.
func waitForReservationStatus(t *testing.T, s *Suite, reservationID string, want reservationpb.ReservationStatus, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		res, err := s.Reservation.GetReservation(context.Background(), &reservationpb.GetReservationRequest{
			ReservationId: reservationID,
		})
		require.NoError(t, err)
		if res.Status == want {
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatalf("reservation %s did not reach status %v within %s", reservationID, want, timeout)
}

// waitForInvoiceStatus polls the billing service until the invoice reaches the
// expected status or the timeout expires.
func waitForInvoiceStatus(t *testing.T, s *Suite, invoiceID string, want billingpb.InvoiceStatus, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		res, err := s.Billing.GetInvoice(context.Background(), &billingpb.GetInvoiceRequest{
			InvoiceId: invoiceID,
		})
		require.NoError(t, err)
		if res.Status == want {
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatalf("invoice %s did not reach status %v within %s", invoiceID, want, timeout)
}
