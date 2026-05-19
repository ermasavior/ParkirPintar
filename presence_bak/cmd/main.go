package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"

	"parkir-pintar/services/presence/internal/presence"
	"parkir-pintar/services/presence/pkg/billingclient"
	"parkir-pintar/services/presence/pkg/config"
	"parkir-pintar/services/presence/pkg/dotenv"
	"parkir-pintar/services/presence/pkg/logger"
	pkgOtel "parkir-pintar/services/presence/pkg/otel"

	"github.com/jackc/pgx/v5/pgxpool"
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
			ServiceName: dotenv.GetEnv("APP_NAME", "presence-service"),
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

	// Billing Service gRPC client with circuit breaker
	billingServiceURL := dotenv.GetEnv("BILLING_SERVICE_URL", "localhost:8084")
	bc, err := billingclient.New(billingServiceURL)
	if err != nil {
		logger.Error(ctx, "failed to create billing service client", slog.String("error", err.Error()))
		os.Exit(1)
	}
	logger.Info(ctx, "billing service client created", slog.String("target", billingServiceURL))

	// gRPC server
	port := dotenv.GetEnv("APP_PORT", "8085")
	lis, err := net.Listen("tcp", fmt.Sprintf(":%s", port))
	if err != nil {
		logger.Error(ctx, "failed to listen", slog.String("port", port), slog.String("error", err.Error()))
		os.Exit(1)
	}

	grpcServer := grpc.NewServer(
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
	)
	presence.RegisterGRPC(grpcServer, pool, bc)

	go func() {
		logger.Info(ctx, "presence service starting", slog.String("port", port))
		if err := grpcServer.Serve(lis); err != nil {
			logger.Error(ctx, "gRPC server error", slog.String("error", err.Error()))
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info(ctx, "shutting down presence service...")
	grpcServer.GracefulStop()
	logger.Info(ctx, "presence service stopped")

	if err := otel.EndAPM(ctx); err != nil {
		logger.Error(ctx, err.Error(), nil)
	}
}
