package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"

	"parkir-pintar/services/search/internal/search"
	"parkir-pintar/services/search/pkg/config"
	"parkir-pintar/services/search/pkg/dotenv"
	"parkir-pintar/services/search/pkg/logger"
	pkgOtel "parkir-pintar/services/search/pkg/otel"

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
			ServiceName: dotenv.GetEnv("APP_NAME", "search-service"),
			Endpoint:    dotenv.GetEnv("OTLP_ENDPOINT", ""),
			Insecure:    true,
		},
	}
	logger.SetupLogger(cfg.Log)

	otel := pkgOtel.NewOpenTelemetry(cfg.OTEL.Endpoint, cfg.OTEL.ServiceName, dotenv.GetEnv("APP_ENV", "local"))

	ctx := context.Background()

	// PostgreSQL — pgxpool
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

	// gRPC server
	port := dotenv.GetEnv("APP_PORT", "8082")
	lis, err := net.Listen("tcp", fmt.Sprintf(":%s", port))
	if err != nil {
		logger.Error(ctx, "failed to listen", slog.String("port", port), slog.String("error", err.Error()))
		os.Exit(1)
	}

	grpcServer := grpc.NewServer(
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
	)
	search.New(pool).RegisterGRPC(grpcServer)

	go func() {
		logger.Info(ctx, "search service starting", slog.String("port", port))
		if err := grpcServer.Serve(lis); err != nil {
			logger.Error(ctx, "gRPC server error", slog.String("error", err.Error()))
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info(ctx, "shutting down search service...")
	grpcServer.GracefulStop()
	logger.Info(ctx, "search service stopped")

	if err := otel.EndAPM(ctx); err != nil {
		logger.Error(ctx, err.Error(), nil)
	}
}
