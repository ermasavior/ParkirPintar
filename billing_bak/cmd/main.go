package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"

	"parkir-pintar/services/billing/internal/billing"
	"parkir-pintar/services/billing/pkg/config"
	"parkir-pintar/services/billing/pkg/dotenv"
	"parkir-pintar/services/billing/pkg/logger"
	pkgOtel "parkir-pintar/services/billing/pkg/otel"
	"parkir-pintar/services/billing/pkg/paymentclient"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
)

func main() {
	dotenv.LoadEnv()

	cfg := config.Config{
		Log: config.LogConfig{
			Level:  dotenv.GetEnv("LOG_LEVEL", "info"),
			Format: dotenv.GetEnv("LOG_FORMAT", "json"),
		},
		OTEL: config.OTELConfig{
			ServiceName: dotenv.GetEnv("APP_NAME", "billing-service"),
			Endpoint:    dotenv.GetEnv("OTLP_ENDPOINT", ""),
			Insecure:    true,
		},
	}
	logger.SetupLogger(cfg.Log)

	otel := pkgOtel.NewOpenTelemetry(cfg.OTEL.Endpoint, cfg.OTEL.ServiceName, dotenv.GetEnv("APP_ENV", "local"))

	ctx := context.Background()

	// PostgreSQL
	pool, err := pgxpool.New(ctx, dotenv.GetEnv("POSTGRES_DSN", ""))
	if err != nil {
		logger.Error(ctx, "failed to create postgres pool", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		logger.Error(ctx, "failed to connect to postgres", slog.String("error", err.Error()))
		os.Exit(1)
	}
	logger.Info(ctx, "connected to postgres")

	// NATS
	natsURL := dotenv.GetEnv("NATS_URL", nats.DefaultURL)
	nc, err := nats.Connect(natsURL)
	if err != nil {
		logger.Error(ctx, "failed to connect to NATS", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer nc.Close()
	logger.Info(ctx, "connected to NATS")

	// Payment Service gRPC client
	paymentServiceURL := dotenv.GetEnv("PAYMENT_SERVICE_URL", "localhost:8086")
	pc, err := paymentclient.New(paymentServiceURL)
	if err != nil {
		logger.Error(ctx, "failed to create payment service client", slog.String("error", err.Error()))
		os.Exit(1)
	}
	logger.Info(ctx, "payment service client created", slog.String("target", paymentServiceURL))

	// Billing domain
	svc := billing.New(pool, nc, pc)

	if err := svc.Start(); err != nil {
		logger.Error(ctx, "failed to start billing service", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer svc.Stop()

	// gRPC server
	port := dotenv.GetEnv("APP_PORT", "8084")
	lis, err := net.Listen("tcp", fmt.Sprintf(":%s", port))
	if err != nil {
		logger.Error(ctx, "failed to listen", slog.String("port", port), slog.String("error", err.Error()))
		os.Exit(1)
	}

	grpcServer := grpc.NewServer(
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
	)
	svc.RegisterGRPC(grpcServer)

	go func() {
		logger.Info(ctx, "billing gRPC server listening", slog.String("port", port))
		if err := grpcServer.Serve(lis); err != nil {
			logger.Error(ctx, "gRPC server error", slog.String("error", err.Error()))
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info(ctx, "shutting down billing service...")
	grpcServer.GracefulStop()
	logger.Info(ctx, "billing service stopped")

	if err := otel.EndAPM(ctx); err != nil {
		logger.Error(ctx, err.Error(), nil)
	}
}
