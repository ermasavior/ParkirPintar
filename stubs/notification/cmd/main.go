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
var subjects = []string{
	"reservation.expired",
	"payment.booking.done",
	"payment.parking.done",
}

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

	for _, subj := range subjects {
		subj := subj // capture loop variable

		// Durable name is derived from the subject so NATS persists consumer
		// state across restarts. On resume it delivers only new messages.
		durableName := "notification-stub-" + strings.ReplaceAll(subj, ".", "-")

		_, err := js.Subscribe(subj, func(msg *nats.Msg) {
			logger.Info(ctx, "[NOTIFICATION STUB] event received",
				slog.String("subject", msg.Subject),
				slog.String("payload", string(msg.Data)),
			)
			// In production: dispatch push notification / SMS / email here.
			msg.Ack()
		}, nats.Durable(durableName), nats.DeliverNew())
		if err != nil {
			// Non-fatal: log and continue — stream may not exist yet on first boot
			logger.Error(ctx, "failed to subscribe",
				slog.String("subject", subj),
				slog.String("error", err.Error()),
			)
		} else {
			logger.Info(ctx, "subscribed to subject",
				slog.String("subject", subj),
				slog.String("durable", durableName),
			)
		}
	}

	logger.Info(ctx, "notification stub running — waiting for events...")

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info(ctx, "shutting down notification stub...")
}
