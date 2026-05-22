//go:build e2e

package e2e_test

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	natsgo "github.com/nats-io/nats.go"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/modules/compose"
	"github.com/testcontainers/testcontainers-go/wait"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	billingpb "parkir-pintar/services/billing/gen/billing/v1"
	paymentpb "parkir-pintar/services/payment/gen/payment/v1"
	presencev1 "parkir-pintar/services/presence/gen/presence/v1"
	reservationpb "parkir-pintar/services/reservation/gen/reservation/v1"
	searchpb "parkir-pintar/services/search/gen/search/v1"
)

const (
	// webhookSecret must match WEBHOOK_SECRET in payment/.env.docker
	webhookSecret = "your-webhook-secret"
)

// Suite holds gRPC clients and a NATS connection wired to the
// docker-compose stack started by testcontainers-compose.
type Suite struct {
	Reservation reservationpb.ReservationServiceClient
	Billing     billingpb.BillingServiceClient
	Presence    presencev1.PresenceServiceClient
	Payment     paymentpb.PaymentServiceClient
	Search      searchpb.SearchServiceClient
	NC          *natsgo.Conn
	JS          natsgo.JetStreamContext

	// webhookURL is resolved dynamically from the mapped payment-service port.
	webhookURL string
}

// serviceEndpoint resolves the host:port for a container service using the
// mapped (random) host port assigned by Docker.
func serviceEndpoint(ctx context.Context, t *testing.T, dc compose.ComposeStack, service, containerPort string) string {
	t.Helper()
	c, err := dc.ServiceContainer(ctx, service)
	require.NoError(t, err, "get container for service %s", service)
	endpoint, err := c.PortEndpoint(ctx, containerPort, "")
	require.NoError(t, err, "get port endpoint for service %s port %s", service, containerPort)
	return endpoint
}

// NewSuite starts the full docker-compose stack via testcontainers-compose,
// waits for each service to be ready, then returns connected clients.
// Everything is torn down automatically via t.Cleanup.
func NewSuite(t *testing.T) *Suite {
	t.Helper()
	ctx := context.Background()

	// Resolve path to the standalone e2e compose file at the repo root.
	// This file is one directory up from e2e/.
	_, thisFile, _, _ := runtime.Caller(0)
	repoRoot := filepath.Join(filepath.Dir(thisFile), "..")
	composeFile := filepath.Join(repoRoot, "docker-compose.e2e.yml")

	// Use a unique project name so the test stack does not conflict with the
	// developer's already-running docker-compose stack.
	dc, err := compose.NewDockerComposeWith(
		compose.WithStackFiles(composeFile),
		compose.StackIdentifier("parkir-e2e-test"),
	)
	require.NoError(t, err, "failed to create docker-compose environment")

	err = dc.
		WaitForService("postgres", wait.ForListeningPort("5432/tcp")).
		WaitForService("redis", wait.ForListeningPort("6379/tcp")).
		WaitForService("nats", wait.ForListeningPort("4222/tcp")).
		WaitForService("reservation-service", wait.ForListeningPort("8082/tcp")).
		WaitForService("search-service", wait.ForListeningPort("8083/tcp")).
		WaitForService("billing-service", wait.ForListeningPort("8084/tcp")).
		WaitForService("presence-service", wait.ForListeningPort("8085/tcp")).
		WaitForService("payment-service", wait.ForListeningPort("8086/tcp")).
		Up(ctx, compose.Wait(true))
	require.NoError(t, err, "failed to start docker-compose stack")

	t.Cleanup(func() {
		require.NoError(t, dc.Down(context.Background(), compose.RemoveOrphans(true), compose.RemoveVolumes(true)))
	})

	// ── Resolve dynamic host ports ─────────────────────────────────
	resAddr      := serviceEndpoint(ctx, t, dc, "reservation-service", "8082/tcp")
	searchAddr   := serviceEndpoint(ctx, t, dc, "search-service", "8083/tcp")
	billAddr     := serviceEndpoint(ctx, t, dc, "billing-service", "8084/tcp")
	presenceAddr := serviceEndpoint(ctx, t, dc, "presence-service", "8085/tcp")
	paymentAddr  := serviceEndpoint(ctx, t, dc, "payment-service", "8086/tcp")
	webhookAddr  := serviceEndpoint(ctx, t, dc, "payment-service", "8087/tcp")
	natsAddr     := serviceEndpoint(ctx, t, dc, "nats", "4222/tcp")

	// ── gRPC clients ──────────────────────────────────────────────
	resConn, err := grpc.NewClient(resAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	t.Cleanup(func() { _ = resConn.Close() })

	searchConn, err := grpc.NewClient(searchAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	t.Cleanup(func() { _ = searchConn.Close() })

	billConn, err := grpc.NewClient(billAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	t.Cleanup(func() { _ = billConn.Close() })

	presenceConn, err := grpc.NewClient(presenceAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	t.Cleanup(func() { _ = presenceConn.Close() })

	paymentConn, err := grpc.NewClient(paymentAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	t.Cleanup(func() { _ = paymentConn.Close() })

	// ── NATS client ───────────────────────────────────────────────
	nc, err := natsgo.Connect(fmt.Sprintf("nats://%s", natsAddr))
	require.NoError(t, err)
	t.Cleanup(nc.Close)

	js, err := nc.JetStream()
	require.NoError(t, err)

	return &Suite{
		Reservation: reservationpb.NewReservationServiceClient(resConn),
		Billing:     billingpb.NewBillingServiceClient(billConn),
		Presence:    presencev1.NewPresenceServiceClient(presenceConn),
		Payment:     paymentpb.NewPaymentServiceClient(paymentConn),
		Search:      searchpb.NewSearchServiceClient(searchConn),
		NC:          nc,
		JS:          js,
		webhookURL:  fmt.Sprintf("http://%s/webhook/payment/callback", webhookAddr),
	}
}

// ── Webhook stub helpers ──────────────────────────────────────────

// webhookCallbackPayload mirrors the JSON body the payment gateway sends to
// the Payment Service's inbound webhook endpoint.
type webhookCallbackPayload struct {
	GatewayRef  string `json:"gateway_ref"`
	ReferenceID string `json:"reference_id"`
	PaymentType string `json:"payment_type"` // "BOOKING_FEE" | "PARKING_FEE"
	Status      string `json:"status"`       // "SUCCESS" | "FAILED" | "EXPIRED"
	AmountIDR   int64  `json:"amount_idr"`
	PaidAt      string `json:"paid_at"`
}

// triggerWebhookCallback posts a signed payment callback directly to the
// Payment Service webhook endpoint, bypassing the payment-gateway stub UI.
// This lets e2e tests drive payment outcomes without a browser.
func (s *Suite) triggerWebhookCallback(t *testing.T, referenceID, paymentType, status string, amountIDR int64) {
	t.Helper()

	payload := webhookCallbackPayload{
		GatewayRef:  fmt.Sprintf("GW-TEST-%s", referenceID[:8]),
		ReferenceID: referenceID,
		PaymentType: paymentType,
		Status:      status,
		AmountIDR:   amountIDR,
		PaidAt:      time.Now().UTC().Format(time.RFC3339),
	}

	body, err := json.Marshal(payload)
	require.NoError(t, err)

	sig := computeHMACSignature(body, webhookSecret)

	req, err := http.NewRequest(http.MethodPost, s.webhookURL, bytes.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Webhook-Signature", sig)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode, "webhook callback must return 200 OK")
}

// computeHMACSignature computes HMAC-SHA256 of body using the shared secret,
// matching the signature scheme in the payment-gateway stub and Payment Service.
func computeHMACSignature(body []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}
