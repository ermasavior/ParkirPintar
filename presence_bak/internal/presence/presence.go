package presence

import (
	pb "parkir-pintar/services/presence/gen/presence/v1"
	"parkir-pintar/services/presence/internal/presence/handler"
	"parkir-pintar/services/presence/internal/presence/repository"
	"parkir-pintar/services/presence/internal/presence/usecase"
	"parkir-pintar/services/presence/pkg/billingclient"

	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"
)

// RegisterGRPC wires up the presence domain and registers it on the gRPC server
func RegisterGRPC(grpcServer *grpc.Server, db *pgxpool.Pool, bc billingclient.BillingService) {
	repo := repository.NewPresence(db)
	uc := usecase.NewPresence(repo, bc)
	srv := handler.NewPresenceServer(uc)

	pb.RegisterPresenceServiceServer(grpcServer, srv)
}
