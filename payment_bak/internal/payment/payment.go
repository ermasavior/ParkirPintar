package payment

import (
	"net/http"

	pb "parkir-pintar/services/payment/gen/payment/v1"
	"parkir-pintar/services/payment/internal/payment/handler"
	"parkir-pintar/services/payment/internal/payment/repository"
	"parkir-pintar/services/payment/internal/payment/usecase"
	"parkir-pintar/services/payment/pkg/gatewayclient"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

type Service struct {
	uc            usecase.Payment
	webhookSecret string
}

func New(db *pgxpool.Pool, nc *nats.Conn, webhookSecret string, gatewayURL string) *Service {
	repo := repository.NewPayment(db)
	gw := gatewayclient.New(gatewayURL)
	uc := usecase.NewPayment(repo, nc, webhookSecret, gw)
	return &Service{uc: uc, webhookSecret: webhookSecret}
}

func (s *Service) RegisterGRPC(grpcServer *grpc.Server) {
	srv := handler.NewPaymentServer(s.uc)
	pb.RegisterPaymentServiceServer(grpcServer, srv)
	reflection.Register(grpcServer)
}

func (s *Service) RegisterWebhook(mux *http.ServeMux) {
	wh := handler.NewWebhookHandler(s.uc, s.webhookSecret)
	mux.HandleFunc("POST /webhook/payment/callback", wh.HandleCallback)
}
