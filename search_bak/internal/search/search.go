package search

import (
	pb "parkir-pintar/services/search/gen/search/v1"
	"parkir-pintar/services/search/internal/search/handler"
	"parkir-pintar/services/search/internal/search/repository"
	"parkir-pintar/services/search/internal/search/usecase"

	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

type Service struct {
	uc usecase.Search
}

func New(db *pgxpool.Pool) *Service {
	repo := repository.NewSearch(db)
	uc := usecase.NewSearch(repo)
	return &Service{uc: uc}
}

func (s *Service) RegisterGRPC(grpcServer *grpc.Server) {
	srv := handler.NewSearchServer(s.uc)
	pb.RegisterSearchServiceServer(grpcServer, srv)
	reflection.Register(grpcServer)
}
