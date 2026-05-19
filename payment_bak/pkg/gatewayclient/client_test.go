package gatewayclient

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testPaymentID   = "110e8400-e29b-41d4-a716-446655440001"
	testReferenceID = "220e8400-e29b-41d4-a716-446655440002"
	testQRCodeURL   = "http://gateway/qris/110e8400-e29b-41d4-a716-446655440001"
)

func validRequest() CreateQRISRequest {
	return CreateQRISRequest{
		PaymentID:   testPaymentID,
		ReferenceID: testReferenceID,
		PaymentType: "BOOKING_FEE",
		AmountIDR:   5000,
	}
}

// ── Success ───────────────────────────────────────────────────────────────────

func TestCreateQRIS_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/qris/create", r.URL.Path)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		var req CreateQRISRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		assert.Equal(t, testPaymentID, req.PaymentID)
		assert.Equal(t, testReferenceID, req.ReferenceID)
		assert.Equal(t, "BOOKING_FEE", req.PaymentType)
		assert.Equal(t, int64(5000), req.AmountIDR)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"qr_code_url": testQRCodeURL})
	}))
	defer srv.Close()

	c := New(srv.URL)
	url, err := c.CreateQRIS(validRequest())

	require.NoError(t, err)
	assert.Equal(t, testQRCodeURL, url)
}

func TestCreateQRIS_ParkingFee(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req CreateQRISRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		assert.Equal(t, "PARKING_FEE", req.PaymentType)
		assert.Equal(t, int64(25000), req.AmountIDR)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"qr_code_url": testQRCodeURL})
	}))
	defer srv.Close()

	c := New(srv.URL)
	url, err := c.CreateQRIS(CreateQRISRequest{
		PaymentID:   testPaymentID,
		ReferenceID: testReferenceID,
		PaymentType: "PARKING_FEE",
		AmountIDR:   25000,
	})

	require.NoError(t, err)
	assert.Equal(t, testQRCodeURL, url)
}

// ── Gateway errors ────────────────────────────────────────────────────────────

func TestCreateQRIS_GatewayReturns500(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := New(srv.URL)
	_, err := c.CreateQRIS(validRequest())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected status 500")
}

func TestCreateQRIS_GatewayReturns404(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	c := New(srv.URL)
	_, err := c.CreateQRIS(validRequest())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected status 404")
}

func TestCreateQRIS_GatewayReturnsInvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("not-json"))
	}))
	defer srv.Close()

	c := New(srv.URL)
	_, err := c.CreateQRIS(validRequest())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "decode response")
}

func TestCreateQRIS_GatewayUnreachable(t *testing.T) {
	// Point at a port nothing is listening on
	c := New("http://127.0.0.1:19999")
	_, err := c.CreateQRIS(validRequest())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "POST /qris/create")
}

// ── Request payload ───────────────────────────────────────────────────────────

func TestCreateQRIS_SendsCorrectPayload(t *testing.T) {
	var received CreateQRISRequest

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&received)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"qr_code_url": testQRCodeURL})
	}))
	defer srv.Close()

	c := New(srv.URL)
	_, err := c.CreateQRIS(CreateQRISRequest{
		PaymentID:   "pay-001",
		ReferenceID: "ref-001",
		PaymentType: "BOOKING_FEE",
		AmountIDR:   5000,
	})

	require.NoError(t, err)
	assert.Equal(t, "pay-001", received.PaymentID)
	assert.Equal(t, "ref-001", received.ReferenceID)
	assert.Equal(t, "BOOKING_FEE", received.PaymentType)
	assert.Equal(t, int64(5000), received.AmountIDR)
}
