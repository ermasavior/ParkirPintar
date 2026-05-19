package usecase

import (
	"context"

	"parkir-pintar/services/payment/internal/payment/model"
	"parkir-pintar/services/payment/internal/payment/repository"
	"parkir-pintar/services/payment/pkg/apperror"
	"parkir-pintar/services/payment/pkg/gatewayclient"

	"github.com/nats-io/nats.go"
)

// Payment defines the business logic contract for the payment domain
type Payment interface {
	// CreatePayment creates a new payment and returns a QRIS code URL (idempotent)
	CreatePayment(ctx context.Context, req model.CreatePaymentRequest) (*model.CreatePaymentResponse, *apperror.AppError)

	// GetPaymentStatus returns the current status of a payment
	GetPaymentStatus(ctx context.Context, paymentID string) (*model.GetPaymentStatusResponse, *apperror.AppError)

	// HandleCallback processes an inbound webhook from the payment gateway
	HandleCallback(ctx context.Context, req model.WebhookCallbackRequest) *apperror.AppError
}

// PaymentUsecase is the concrete implementation
type PaymentUsecase struct {
	repo          repository.Payment
	natsConn      *nats.Conn
	webhookSecret string                // HMAC-SHA256 shared secret for webhook signature validation
	gateway       gatewayclient.Gateway // injectable — allows mocking in tests
}

// NewPayment creates a new PaymentUsecase
func NewPayment(repo repository.Payment, natsConn *nats.Conn, webhookSecret string, gateway gatewayclient.Gateway) Payment {
	return &PaymentUsecase{
		repo:          repo,
		natsConn:      natsConn,
		webhookSecret: webhookSecret,
		gateway:       gateway,
	}
}
