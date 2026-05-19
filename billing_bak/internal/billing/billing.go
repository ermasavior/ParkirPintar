package billing

import (
	pb "parkir-pintar/services/billing/gen/billing/v1"
	"parkir-pintar/services/billing/internal/billing/handler"
	"parkir-pintar/services/billing/internal/billing/repository"
	"parkir-pintar/services/billing/internal/billing/usecase"
	"parkir-pintar/services/billing/pkg/paymentclient"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

type Service struct {
	consumer *handler.ParkingPaymentConsumer
	uc       usecase.Billing
}

func New(db *pgxpool.Pool, nc *nats.Conn, pc paymentclient.PaymentService) *Service {
	repo := repository.NewBilling(db)
	uc := usecase.NewBilling(repo, pc)
	consumer := handler.NewParkingPaymentConsumer(nc, uc)
	return &Service{consumer: consumer, uc: uc}
}

func (s *Service) Start() error {
	return s.consumer.Start()
}

func (s *Service) Stop() {
	s.consumer.Stop()
}

func (s *Service) RegisterGRPC(grpcServer *grpc.Server) {
	srv := handler.NewBillingServer(s.uc)
	pb.RegisterBillingServiceServer(grpcServer, srv)
	reflection.Register(grpcServer)
}
