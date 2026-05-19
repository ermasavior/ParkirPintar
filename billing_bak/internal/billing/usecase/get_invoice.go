package usecase

import (
	"context"

	"parkir-pintar/services/billing/internal/billing/model"
	"parkir-pintar/services/billing/pkg/apperror"
)

func (u *BillingUsecase) GetInvoice(ctx context.Context, invoiceID string) (*model.GetInvoiceResponse, *apperror.AppError) {
	inv, appErr := u.repo.GetByID(ctx, invoiceID)
	if appErr != nil {
		return nil, appErr
	}

	return &model.GetInvoiceResponse{
		InvoiceID:       inv.ID,
		SessionID:       inv.SessionID,
		ReservationID:   inv.ReservationID,
		Status:          inv.Status,
		BookingFeeIDR:   inv.BookingFeeIDR,
		ParkingFeeIDR:   inv.ParkingFeeIDR,
		OvernightFeeIDR: inv.OvernightFeeIDR,
		TotalIDR:        inv.TotalIDR,
		CreatedAt:       inv.CreatedAt,
	}, nil
}
