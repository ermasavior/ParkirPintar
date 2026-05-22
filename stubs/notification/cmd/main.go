package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"parkir-pintar/services/notification/pkg/dotenv"
	"parkir-pintar/services/notification/pkg/logger"

	"github.com/nats-io/nats.go"
)

// Notification subjects consumed by this stub service.
// In production each would trigger a push/SMS/email to the driver.
//
// payment.booking.done and payment.parking.done are JetStream subjects (durable).
// reservation.expired is a core NATS subject (plain subscribe).
var jetStreamSubjects = []string{
	"payment.booking.done",
	"payment.parking.done",
}

const coreSubjectReservationExpired = "reservation.expired"

func main() {
	dotenv.LoadEnv()

	logger.SetupLogger(logger.LogConfig{
		Level:  dotenv.GetEnv("LOG_LEVEL", "info"),
		Format: dotenv.GetEnv("LOG_FORMAT", "json"),
	})

	ctx := context.Background()

	natsURL := dotenv.GetEnv("NATS_URL", nats.DefaultURL)
	nc, err := nats.Connect(natsURL)
	if err != nil {
		logger.Error(ctx, "failed to connect to NATS", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer nc.Close()
	logger.Info(ctx, "connected to NATS", slog.String("url", natsURL))

	js, err := nc.JetStream()
	if err != nil {
		logger.Error(ctx, "failed to create JetStream context", slog.String("error", err.Error()))
		os.Exit(1)
	}

	// Ensure streams exist — idempotent, safe to call even if already created
	// by billing or reservation service consumers.
	streams := map[string][]string{
		"PAYMENTS": {"payment.booking.done", "payment.parking.done"},
	}
	for streamName, streamSubjects := range streams {
		_, err := js.AddStream(&nats.StreamConfig{
			Name:     streamName,
			Subjects: streamSubjects,
		})
		if err != nil && err != nats.ErrStreamNameAlreadyInUse {
			logger.Error(ctx, "failed to ensure stream",
				slog.String("stream", streamName),
				slog.String("error", err.Error()),
			)
		}
	}

	// Subscribe to JetStream subjects (durable — persists consumer state across restarts)
	for _, subj := range jetStreamSubjects {
		subj := subj
		durableName := "notification-stub-" + strings.ReplaceAll(subj, ".", "-")

		_, err := js.Subscribe(subj, func(msg *nats.Msg) {
			logger.Info(ctx, "[NOTIFICATION STUB] event received",
				slog.String("subject", msg.Subject),
				slog.String("payload", string(msg.Data)),
			)
			msg.Ack()
		}, nats.Durable(durableName), nats.DeliverNew())
		if err != nil {
			logger.Error(ctx, "failed to subscribe to JetStream subject",
				slog.String("subject", subj),
				slog.String("error", err.Error()),
			)
		} else {
			logger.Info(ctx, "subscribed to JetStream subject",
				slog.String("subject", subj),
				slog.String("durable", durableName),
			)
		}
	}

	// Subscribe to core NATS subject (reservation.expired — published via nc.Publish)
	_, err = nc.Subscribe(coreSubjectReservationExpired, func(msg *nats.Msg) {
		logger.Info(ctx, "[NOTIFICATION STUB] event received",
			slog.String("subject", msg.Subject),
			slog.String("payload", string(msg.Data)),
		)
	})
	if err != nil {
		logger.Error(ctx, "failed to subscribe to core NATS subject",
			slog.String("subject", coreSubjectReservationExpired),
			slog.String("error", err.Error()),
		)
	} else {
		logger.Info(ctx, "subscribed to core NATS subject",
			slog.String("subject", coreSubjectReservationExpired),
		)
	}

	logger.Info(ctx, "notification stub running — waiting for events...")

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info(ctx, "shutting down notification stub...")
}
