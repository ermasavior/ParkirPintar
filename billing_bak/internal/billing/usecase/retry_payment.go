package usecase

import (
	"context"
	"log/slog"

	paymentpb "parkir-pintar/services/billing/gen/payment/v1"
	"parkir-pintar/services/billing/internal/billing/model"
	"parkir-pintar/services/billing/pkg/apperror"
	"parkir-pintar/services/billing/pkg/logger"

	"github.com/google/uuid"
)

// RetryPayment resets a PAYMENT_FAILED invoice and issues a new QRIS code.
//
// Flow:
// 1. Fetch invoice — validates it exists and belongs to driver
// 2. Validate invoice status == PAYMENT_FAILED
// 3. Update invoice status → PENDING_PAYMENT
// 4. Call Payment Service via gRPC to create a new QRIS payment
// 5. Return new payment_id + qr_code_url
func (u *BillingUsecase) RetryPayment(ctx context.Context, req model.RetryPaymentRequest) (*model.RetryPaymentResponse, *apperror.AppError) {
	inv, appErr := u.repo.GetByIDForDriver(ctx, req.InvoiceID, req.DriverID)
	if appErr != nil {
		return nil, appErr
	}

	if inv.Status != model.InvoiceStatusPaymentFailed {
		return nil, apperror.New("conflict", "invoice is not in PAYMENT_FAILED status — retry is only allowed for failed payments")
	}

	if appErr := u.repo.UpdateStatusToPendingPayment(ctx, req.InvoiceID); appErr != nil {
		return nil, appErr
	}

	// Step 4: Call Payment Service for a fresh QRIS code
	// Generate a new UUID for each retry — each retry creates a new payment record
	// (previous FAILED/EXPIRED record is preserved for audit)
	retryIdemKey := uuid.New().String()

	paymentResult, appErr := u.paymentClient.CreatePayment(ctx,
		retryIdemKey,
		req.InvoiceID, // reference_id = invoice_id for parking fee
		req.DriverID,
		paymentpb.PaymentType_PAYMENT_TYPE_PARKING_FEE,
		inv.TotalIDR,
	)
	if appErr != nil {
		logger.Error(ctx, "RetryPayment: payment service call failed",
			slog.String("invoice_id", req.InvoiceID),
			slog.String("error", appErr.Error()),
		)
		return nil, appErr
	}

	logger.Info(ctx, "RetryPayment: new payment created",
		slog.String("invoice_id", req.InvoiceID),
		slog.String("payment_id", paymentResult.PaymentID),
	)

	return &model.RetryPaymentResponse{
		PaymentID: paymentResult.PaymentID,
		QRCodeURL: paymentResult.QRCodeURL,
	}, nil
}
