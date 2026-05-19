// Package billingclient provides a gRPC client for the Billing Service
// with a circuit breaker to prevent cascading failures.
package billingclient

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	billingpb "parkir-pintar/services/presence/gen/billing/v1"
	"parkir-pintar/services/presence/pkg/apperror"
	"parkir-pintar/services/presence/pkg/logger"

	"github.com/google/uuid"
	"github.com/sony/gobreaker/v2"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// CreateInvoiceResult holds the result of a CalculateAndCreateInvoice call
type CreateInvoiceResult struct {
	InvoiceID string
	TotalIDR  int64
	QRCodeURL string
}

type BillingService interface {
	CalculateAndCreateInvoice(ctx context.Context,
		sessionID, reservationID, driverID string,
		checkedInAt, checkedOutAt time.Time,
	) (*CreateInvoiceResult, *apperror.AppError)
}

// caller is the function that actually makes the gRPC call — injectable for tests
type caller func(ctx context.Context,
	sessionID, reservationID, driverID string,
	checkedInAt, checkedOutAt time.Time,
) (*CreateInvoiceResult, error)

type Client struct {
	cb     *gobreaker.CircuitBreaker[*CreateInvoiceResult]
	callFn caller
}

// compile-time check
var _ BillingService = (*Client)(nil)

// New creates a Client connected to the given address.
//
// Circuit breaker settings (from LLD):
//   - 5 consecutive failures → OPEN
//   - 30s timeout before HALF-OPEN probe
//   - 1 probe request allowed in HALF-OPEN
func New(target string) (*Client, error) {
	conn, err := grpc.NewClient(target,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
	)
	if err != nil {
		return nil, fmt.Errorf("billingclient: failed to connect to %s: %w", target, err)
	}

	grpcClient := billingpb.NewBillingServiceClient(conn)

	callFn := func(ctx context.Context,
		sessionID, reservationID, driverID string,
		checkedInAt, checkedOutAt time.Time,
	) (*CreateInvoiceResult, error) {
		// Deterministic idempotency key: same session always produces the same invoice key
		idemKey := uuid.NewSHA1(uuid.NameSpaceURL, []byte("invoice:"+sessionID)).String()
		resp, err := grpcClient.CalculateAndCreateInvoice(ctx, &billingpb.CreateInvoiceRequest{
			IdempotencyKey: idemKey,
			SessionId:      sessionID,
			ReservationId:  reservationID,
			DriverId:       driverID,
			CheckedInAt:    timestamppb.New(checkedInAt),
			CheckedOutAt:   timestamppb.New(checkedOutAt),
		})
		if err != nil {
			return nil, err
		}
		return &CreateInvoiceResult{
			InvoiceID: resp.InvoiceId,
			TotalIDR:  resp.TotalIdr,
			QRCodeURL: resp.QrCodeUrl,
		}, nil
	}

	return newWithCaller(callFn), nil
}

// newWithCaller creates a Client with an injected caller — used in tests
func newWithCaller(fn caller) *Client {
	cb := gobreaker.NewCircuitBreaker[*CreateInvoiceResult](gobreaker.Settings{
		Name:        "billing-service",
		MaxRequests: 1,
		Interval:    60 * time.Second,
		Timeout:     30 * time.Second,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return counts.ConsecutiveFailures >= 5
		},
		OnStateChange: func(name string, from, to gobreaker.State) {
			logger.Info(context.Background(), "billingclient: circuit breaker state changed",
				slog.String("name", name),
				slog.String("from", from.String()),
				slog.String("to", to.String()),
			)
		},
	})
	return &Client{cb: cb, callFn: fn}
}

func (c *Client) CalculateAndCreateInvoice(ctx context.Context,
	sessionID, reservationID, driverID string,
	checkedInAt, checkedOutAt time.Time,
) (*CreateInvoiceResult, *apperror.AppError) {

	result, err := c.cb.Execute(func() (*CreateInvoiceResult, error) {
		return c.callFn(ctx, sessionID, reservationID, driverID, checkedInAt, checkedOutAt)
	})

	if err != nil {
		if err == gobreaker.ErrOpenState {
			logger.Error(ctx, "billingclient: circuit breaker is OPEN — billing service unavailable",
				slog.String("session_id", sessionID),
			)
			return nil, apperror.New("billing_service_unavailable", "billing service is temporarily unavailable, please retry later")
		}
		logger.Error(ctx, "billingclient: CalculateAndCreateInvoice failed",
			slog.String("session_id", sessionID),
			slog.String("error", err.Error()),
		)
		return nil, apperror.New("billing_service_error", "failed to create invoice: "+err.Error())
	}

	return result, nil
}
