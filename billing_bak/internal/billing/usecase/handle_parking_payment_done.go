package usecase

import (
	"context"
	"log/slog"

	"parkir-pintar/services/billing/internal/billing/model"
	"parkir-pintar/services/billing/pkg/apperror"
	"parkir-pintar/services/billing/pkg/logger"
)

func (u *BillingUsecase) HandleParkingPaymentDone(ctx context.Context, invoiceID string, status string) *apperror.AppError {
	// Validate invoice exists
	inv, appErr := u.repo.GetByID(ctx, invoiceID)
	if appErr != nil {
		return appErr
	}

	var newStatus model.InvoiceStatus
	switch status {
	case "SUCCESS":
		newStatus = model.InvoiceStatusPaid
	case "FAILED", "EXPIRED":
		newStatus = model.InvoiceStatusPaymentFailed
	default:
		return apperror.New("validation_error", "unknown payment status: "+status)
	}

	if appErr := u.repo.UpdateInvoiceStatus(ctx, inv.ID, newStatus); appErr != nil {
		return appErr
	}

	logger.Info(ctx, "HandleParkingPaymentDone: invoice status updated",
		slog.String("invoice_id", inv.ID),
		slog.String("status", status),
	)

	return nil
}
