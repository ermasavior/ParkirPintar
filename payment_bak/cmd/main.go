package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"parkir-pintar/services/payment/internal/payment"
	"parkir-pintar/services/payment/pkg/config"
	"parkir-pintar/services/payment/pkg/dotenv"
	"parkir-pintar/services/payment/pkg/logger"
	pkgOtel "parkir-pintar/services/payment/pkg/otel"

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
			ServiceName: dotenv.GetEnv("APP_NAME", "payment-service"),
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

	webhookSecret := dotenv.GetEnv("WEBHOOK_SECRET", "")
	gatewayURL := dotenv.GetEnv("GATEWAY_STUB_URL", "http://gateway-stub:8088")

	// Payment domain
	svc := payment.New(pool, nc, webhookSecret, gatewayURL)

	// gRPC server
	grpcPort := dotenv.GetEnv("APP_PORT", "8086")
	lis, err := net.Listen("tcp", fmt.Sprintf(":%s", grpcPort))
	if err != nil {
		logger.Error(ctx, "failed to listen", slog.String("port", grpcPort), slog.String("error", err.Error()))
		os.Exit(1)
	}

	grpcServer := grpc.NewServer(
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
	)
	svc.RegisterGRPC(grpcServer)

	go func() {
		logger.Info(ctx, "payment gRPC service starting", slog.String("port", grpcPort))
		if err := grpcServer.Serve(lis); err != nil {
			logger.Error(ctx, "gRPC server error", slog.String("error", err.Error()))
			os.Exit(1)
		}
	}()

	// HTTP server for webhook
	webhookPort := dotenv.GetEnv("WEBHOOK_PORT", "8087")
	mux := http.NewServeMux()
	svc.RegisterWebhook(mux)

	httpServer := &http.Server{
		Addr:    fmt.Sprintf(":%s", webhookPort),
		Handler: mux,
	}

	go func() {
		logger.Info(ctx, "payment webhook server starting", slog.String("port", webhookPort))
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error(ctx, "webhook server error", slog.String("error", err.Error()))
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info(ctx, "shutting down payment service...")
	grpcServer.GracefulStop()
	_ = httpServer.Shutdown(ctx)
	logger.Info(ctx, "payment service stopped")

	if err := otel.EndAPM(ctx); err != nil {
		logger.Error(ctx, err.Error(), nil)
	}
}
