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

	paymentpb "parkir-pintar/services/payment/gen/payment/v1"
)

// ── Payment tests ─────────────────────────────────────────────────

// TestE2E_CreatePayment_BookingFee verifies that creating a BOOKING_FEE payment
// returns a pending payment with a QR code URL.
func TestE2E_CreatePayment_BookingFee(t *testing.T) {
	s := NewSuite(t)

	res, err := s.Payment.CreatePayment(context.Background(), &paymentpb.CreatePaymentRequest{
		IdempotencyKey: uuid.New().String(),
		ReferenceId:    uuid.New().String(),
		PaymentType:    paymentpb.PaymentType_PAYMENT_TYPE_BOOKING_FEE,
		AmountIdr:      5000,
		DriverId:       uuid.New().String(),
		Method:         paymentpb.PaymentMethod_PAYMENT_METHOD_QRIS,
	})

	require.NoError(t, err)
	assert.NotEmpty(t, res.PaymentId)
	assert.NotEmpty(t, res.QrCodeUrl)
	assert.Equal(t, paymentpb.PaymentStatus_PAYMENT_STATUS_PENDING, res.Status)
}

// TestE2E_CreatePayment_ParkingFee verifies that creating a PARKING_FEE payment
// returns a pending payment with a QR code URL.
func TestE2E_CreatePayment_ParkingFee(t *testing.T) {
	s := NewSuite(t)

	res, err := s.Payment.CreatePayment(context.Background(), &paymentpb.CreatePaymentRequest{
		IdempotencyKey: uuid.New().String(),
		ReferenceId:    uuid.New().String(),
		PaymentType:    paymentpb.PaymentType_PAYMENT_TYPE_PARKING_FEE,
		AmountIdr:      15000,
		DriverId:       uuid.New().String(),
		Method:         paymentpb.PaymentMethod_PAYMENT_METHOD_QRIS,
	})

	require.NoError(t, err)
	assert.NotEmpty(t, res.PaymentId)
	assert.NotEmpty(t, res.QrCodeUrl)
	assert.Equal(t, paymentpb.PaymentStatus_PAYMENT_STATUS_PENDING, res.Status)
}

// TestE2E_CreatePayment_Idempotency verifies that retrying the same idempotency
// key returns the same payment record without creating a duplicate.
func TestE2E_CreatePayment_Idempotency(t *testing.T) {
	s := NewSuite(t)

	req := &paymentpb.CreatePaymentRequest{
		IdempotencyKey: uuid.New().String(),
		ReferenceId:    uuid.New().String(),
		PaymentType:    paymentpb.PaymentType_PAYMENT_TYPE_BOOKING_FEE,
		AmountIdr:      5000,
		DriverId:       uuid.New().String(),
		Method:         paymentpb.PaymentMethod_PAYMENT_METHOD_QRIS,
	}

	first, err := s.Payment.CreatePayment(context.Background(), req)
	require.NoError(t, err)

	second, err := s.Payment.CreatePayment(context.Background(), req)
	require.NoError(t, err)

	assert.Equal(t, first.PaymentId, second.PaymentId)
	assert.Equal(t, first.QrCodeUrl, second.QrCodeUrl)
}

// TestE2E_GetPaymentStatus_Pending verifies that a newly created payment
// can be fetched and shows PENDING status.
func TestE2E_GetPaymentStatus_Pending(t *testing.T) {
	s := NewSuite(t)

	referenceID := uuid.New().String()
	createRes, err := s.Payment.CreatePayment(context.Background(), &paymentpb.CreatePaymentRequest{
		IdempotencyKey: uuid.New().String(),
		ReferenceId:    referenceID,
		PaymentType:    paymentpb.PaymentType_PAYMENT_TYPE_BOOKING_FEE,
		AmountIdr:      5000,
		DriverId:       uuid.New().String(),
		Method:         paymentpb.PaymentMethod_PAYMENT_METHOD_QRIS,
	})
	require.NoError(t, err)

	statusRes, err := s.Payment.GetPaymentStatus(context.Background(), &paymentpb.GetPaymentStatusRequest{
		PaymentId: createRes.PaymentId,
	})

	require.NoError(t, err)
	assert.Equal(t, createRes.PaymentId, statusRes.PaymentId)
	assert.Equal(t, referenceID, statusRes.ReferenceId)
	assert.Equal(t, paymentpb.PaymentType_PAYMENT_TYPE_BOOKING_FEE, statusRes.PaymentType)
	assert.Equal(t, paymentpb.PaymentStatus_PAYMENT_STATUS_PENDING, statusRes.Status)
	assert.Equal(t, int64(5000), statusRes.AmountIdr)
}

// TestE2E_GetPaymentStatus_NotFound verifies that fetching a non-existent
// payment returns a NOT_FOUND error.
func TestE2E_GetPaymentStatus_NotFound(t *testing.T) {
	s := NewSuite(t)

	_, err := s.Payment.GetPaymentStatus(context.Background(), &paymentpb.GetPaymentStatusRequest{
		PaymentId: uuid.New().String(),
	})

	require.Error(t, err)
	assert.Equal(t, codes.NotFound, status.Code(err))
}

// TestE2E_WebhookCallback_BookingFeeSuccess verifies that a SUCCESS webhook
// callback for a BOOKING_FEE payment transitions the payment to SUCCESS.
func TestE2E_WebhookCallback_BookingFeeSuccess(t *testing.T) {
	s := NewSuite(t)

	referenceID := uuid.New().String()
	createRes, err := s.Payment.CreatePayment(context.Background(), &paymentpb.CreatePaymentRequest{
		IdempotencyKey: uuid.New().String(),
		ReferenceId:    referenceID,
		PaymentType:    paymentpb.PaymentType_PAYMENT_TYPE_BOOKING_FEE,
		AmountIdr:      5000,
		DriverId:       uuid.New().String(),
		Method:         paymentpb.PaymentMethod_PAYMENT_METHOD_QRIS,
	})
	require.NoError(t, err)

	s.triggerWebhookCallback(t, referenceID, "BOOKING_FEE", "SUCCESS", 5000)

	waitForPaymentStatus(t, s, createRes.PaymentId, paymentpb.PaymentStatus_PAYMENT_STATUS_SUCCESS, 10*time.Second)
}

// TestE2E_WebhookCallback_BookingFeeFailed verifies that a FAILED webhook
// callback transitions the payment to FAILED.
func TestE2E_WebhookCallback_BookingFeeFailed(t *testing.T) {
	s := NewSuite(t)

	referenceID := uuid.New().String()
	createRes, err := s.Payment.CreatePayment(context.Background(), &paymentpb.CreatePaymentRequest{
		IdempotencyKey: uuid.New().String(),
		ReferenceId:    referenceID,
		PaymentType:    paymentpb.PaymentType_PAYMENT_TYPE_BOOKING_FEE,
		AmountIdr:      5000,
		DriverId:       uuid.New().String(),
		Method:         paymentpb.PaymentMethod_PAYMENT_METHOD_QRIS,
	})
	require.NoError(t, err)

	s.triggerWebhookCallback(t, referenceID, "BOOKING_FEE", "FAILED", 5000)

	waitForPaymentStatus(t, s, createRes.PaymentId, paymentpb.PaymentStatus_PAYMENT_STATUS_FAILED, 10*time.Second)
}

// TestE2E_WebhookCallback_ParkingFeeExpired verifies that an EXPIRED webhook
// callback transitions the payment to EXPIRED.
func TestE2E_WebhookCallback_ParkingFeeExpired(t *testing.T) {
	s := NewSuite(t)

	referenceID := uuid.New().String()
	createRes, err := s.Payment.CreatePayment(context.Background(), &paymentpb.CreatePaymentRequest{
		IdempotencyKey: uuid.New().String(),
		ReferenceId:    referenceID,
		PaymentType:    paymentpb.PaymentType_PAYMENT_TYPE_PARKING_FEE,
		AmountIdr:      10000,
		DriverId:       uuid.New().String(),
		Method:         paymentpb.PaymentMethod_PAYMENT_METHOD_QRIS,
	})
	require.NoError(t, err)

	s.triggerWebhookCallback(t, referenceID, "PARKING_FEE", "EXPIRED", 10000)

	waitForPaymentStatus(t, s, createRes.PaymentId, paymentpb.PaymentStatus_PAYMENT_STATUS_EXPIRED, 10*time.Second)
}

// waitForPaymentStatus polls the payment service until the payment reaches the
// expected status or the timeout expires.
func waitForPaymentStatus(t *testing.T, s *Suite, paymentID string, want paymentpb.PaymentStatus, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		res, err := s.Payment.GetPaymentStatus(context.Background(), &paymentpb.GetPaymentStatusRequest{
			PaymentId: paymentID,
		})
		require.NoError(t, err)
		if res.Status == want {
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatalf("payment %s did not reach status %v within %s", paymentID, want, timeout)
}
