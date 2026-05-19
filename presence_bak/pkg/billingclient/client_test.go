package billingclient

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testSessionID     = "110e8400-e29b-41d4-a716-446655440001"
	testReservationID = "220e8400-e29b-41d4-a716-446655440002"
	testDriverID      = "330e8400-e29b-41d4-a716-446655440003"
)

func successCaller(_ context.Context, _, _, _ string, _, _ time.Time) (*CreateInvoiceResult, error) {
	return &CreateInvoiceResult{
		InvoiceID: "inv-001",
		TotalIDR:  15000,
		QRCodeURL: "https://qr.example.com/stub",
	}, nil
}

func failCaller(_ context.Context, _, _, _ string, _, _ time.Time) (*CreateInvoiceResult, error) {
	return nil, fmt.Errorf("rpc error: connection refused")
}

// ── Success ───────────────────────────────────────────────────────────────────

func TestCalculateAndCreateInvoice_Success(t *testing.T) {
	c := newWithCaller(successCaller)

	result, appErr := c.CalculateAndCreateInvoice(context.Background(),
		testSessionID, testReservationID, testDriverID,
		time.Now().Add(-2*time.Hour), time.Now(),
	)

	require.Nil(t, appErr)
	assert.Equal(t, "inv-001", result.InvoiceID)
	assert.Equal(t, int64(15000), result.TotalIDR)
	assert.Equal(t, "https://qr.example.com/stub", result.QRCodeURL)
}

// ── Single failure — circuit stays CLOSED ────────────────────────────────────

func TestCalculateAndCreateInvoice_SingleFailure_CircuitStayClosed(t *testing.T) {
	c := newWithCaller(failCaller)

	_, appErr := c.CalculateAndCreateInvoice(context.Background(),
		testSessionID, testReservationID, testDriverID,
		time.Now().Add(-2*time.Hour), time.Now(),
	)

	require.NotNil(t, appErr)
	assert.Equal(t, "billing_service_error", appErr.ErrorCode)
}

// ── 5 consecutive failures → circuit OPENS ───────────────────────────────────

func TestCalculateAndCreateInvoice_FiveConsecutiveFailures_CircuitOpens(t *testing.T) {
	c := newWithCaller(failCaller)

	for i := 0; i < 5; i++ {
		_, appErr := c.CalculateAndCreateInvoice(context.Background(),
			testSessionID, testReservationID, testDriverID,
			time.Now().Add(-2*time.Hour), time.Now(),
		)
		require.NotNil(t, appErr)
		assert.Equal(t, "billing_service_error", appErr.ErrorCode)
	}

	// 6th call — circuit is now OPEN
	_, appErr := c.CalculateAndCreateInvoice(context.Background(),
		testSessionID, testReservationID, testDriverID,
		time.Now().Add(-2*time.Hour), time.Now(),
	)

	require.NotNil(t, appErr)
	assert.Equal(t, "billing_service_unavailable", appErr.ErrorCode)
	assert.Contains(t, appErr.Message, "temporarily unavailable")
}

// ── Circuit OPEN — no downstream calls made ───────────────────────────────────

func TestCalculateAndCreateInvoice_CircuitOpen_DoesNotCallDownstream(t *testing.T) {
	callCount := 0
	countingCaller := func(_ context.Context, _, _, _ string, _, _ time.Time) (*CreateInvoiceResult, error) {
		callCount++
		return nil, fmt.Errorf("downstream error")
	}

	c := newWithCaller(countingCaller)

	for i := 0; i < 5; i++ {
		c.CalculateAndCreateInvoice(context.Background(),
			testSessionID, testReservationID, testDriverID,
			time.Now().Add(-2*time.Hour), time.Now())
	}

	callsBefore := callCount

	_, appErr := c.CalculateAndCreateInvoice(context.Background(),
		testSessionID, testReservationID, testDriverID,
		time.Now().Add(-2*time.Hour), time.Now())

	require.NotNil(t, appErr)
	assert.Equal(t, "billing_service_unavailable", appErr.ErrorCode)
	assert.Equal(t, callsBefore, callCount, "downstream should not be called when circuit is OPEN")
}

// ── Success after failures resets consecutive count ───────────────────────────

func TestCalculateAndCreateInvoice_SuccessAfterFailures_ResetsCircuit(t *testing.T) {
	callCount := 0
	mixedCaller := func(_ context.Context, _, _, _ string, _, _ time.Time) (*CreateInvoiceResult, error) {
		callCount++
		if callCount <= 4 {
			return nil, fmt.Errorf("transient error")
		}
		return &CreateInvoiceResult{InvoiceID: "inv-ok", TotalIDR: 15000, QRCodeURL: "https://qr.example.com"}, nil
	}

	c := newWithCaller(mixedCaller)

	for i := 0; i < 4; i++ {
		_, appErr := c.CalculateAndCreateInvoice(context.Background(),
			testSessionID, testReservationID, testDriverID,
			time.Now().Add(-2*time.Hour), time.Now())
		assert.Equal(t, "billing_service_error", appErr.ErrorCode)
	}

	result, appErr := c.CalculateAndCreateInvoice(context.Background(),
		testSessionID, testReservationID, testDriverID,
		time.Now().Add(-2*time.Hour), time.Now())

	require.Nil(t, appErr)
	assert.Equal(t, "inv-ok", result.InvoiceID)
}

// ── New() with bad target — gRPC dials lazily ─────────────────────────────────

func TestNew_InvalidTarget_ReturnsClient(t *testing.T) {
	c, err := New("localhost:1")
	assert.NoError(t, err) // gRPC dials lazily
	assert.NotNil(t, c)
}
