package usecase

import (
	"context"

	"parkir-pintar/services/billing/internal/billing/model"
	"parkir-pintar/services/billing/internal/billing/repository"
	"parkir-pintar/services/billing/pkg/apperror"
	"parkir-pintar/services/billing/pkg/paymentclient"
)

type Billing interface {
	// CalculateAndCreateInvoice calculates fees and creates an invoice (idempotent)
	CalculateAndCreateInvoice(ctx context.Context, req model.CreateInvoiceRequest) (*model.CreateInvoiceResponse, *apperror.AppError)

	// GetInvoice retrieves an invoice by its UUID
	GetInvoice(ctx context.Context, invoiceID string) (*model.GetInvoiceResponse, *apperror.AppError)

	// RetryPayment resets a PAYMENT_FAILED invoice and issues a new QRIS code
	RetryPayment(ctx context.Context, req model.RetryPaymentRequest) (*model.RetryPaymentResponse, *apperror.AppError)

	// HandleParkingPaymentDone processes a payment.parking.done NATS event
	HandleParkingPaymentDone(ctx context.Context, invoiceID string, status string) *apperror.AppError
}

type BillingUsecase struct {
	repo          repository.Billing
	paymentClient paymentclient.PaymentService // interface — allows mocking in tests
}

func NewBilling(repo repository.Billing, pc paymentclient.PaymentService) Billing {
	return &BillingUsecase{
		repo:          repo,
		paymentClient: pc,
	}
}
