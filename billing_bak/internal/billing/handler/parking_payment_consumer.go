package handler

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"parkir-pintar/services/billing/internal/billing/model"
	"parkir-pintar/services/billing/internal/billing/usecase"
	"parkir-pintar/services/billing/pkg/logger"

	"github.com/nats-io/nats.go"
)

const (
	subjectParkingDone = "payment.parking.done"
	durableName        = "billing-service"
)

type ParkingPaymentConsumer struct {
	uc   usecase.Billing
	nc   *nats.Conn
	sub  *nats.Subscription
}

func NewParkingPaymentConsumer(nc *nats.Conn, uc usecase.Billing) *ParkingPaymentConsumer {
	return &ParkingPaymentConsumer{nc: nc, uc: uc}
}

func (c *ParkingPaymentConsumer) Start() error {
	js, err := c.nc.JetStream()
	if err != nil {
		return err
	}

	// Ensure stream exists (idempotent)
	_, err = js.AddStream(&nats.StreamConfig{
		Name:     "PAYMENTS",
		Subjects: []string{"payment.booking.done", "payment.parking.done"},
	})
	if err != nil && err != nats.ErrStreamNameAlreadyInUse {
		// Only fail on unexpected errors
		logger.Warn(context.Background(), "ParkingPaymentConsumer: AddStream warning",
			slog.String("error", err.Error()),
		)
	}

	sub, err := js.Subscribe(subjectParkingDone, c.handle,
		nats.Durable(durableName),
		nats.ManualAck(),
		nats.AckExplicit(),
	)
	if err != nil {
		return err
	}

	c.sub = sub
	logger.Info(context.Background(), "ParkingPaymentConsumer: subscribed",
		slog.String("subject", subjectParkingDone),
		slog.String("durable", durableName),
	)
	return nil
}

func (c *ParkingPaymentConsumer) Stop() {
	if c.sub != nil {
		_ = c.sub.Unsubscribe()
	}
}

func (c *ParkingPaymentConsumer) handle(msg *nats.Msg) {
	ctx := context.Background()

	var event model.NATSPaymentDoneEvent
	if err := json.Unmarshal(msg.Data, &event); err != nil {
		logger.Error(ctx, "ParkingPaymentConsumer: failed to unmarshal event",
			slog.String("error", err.Error()),
		)
		_ = msg.Term()
		return
	}

	logger.Info(ctx, "ParkingPaymentConsumer: received event",
		slog.String("reference_id", event.ReferenceID),
		slog.String("status", event.Status),
	)

	appErr := c.uc.HandleParkingPaymentDone(ctx, event.ReferenceID, event.Status)
	if appErr != nil {
		logger.Error(ctx, "ParkingPaymentConsumer: HandleParkingPaymentDone failed",
			slog.String("error", appErr.Error()),
			slog.String("reference_id", event.ReferenceID),
		)
		switch appErr.ErrorCode {
		case "not_found":
			_ = msg.Term()
		default:
			// Transient error (db_error) — redeliver after 5 seconds
			_ = msg.NakWithDelay(5 * time.Second)
		}
		return
	}

	_ = msg.Ack()
}
