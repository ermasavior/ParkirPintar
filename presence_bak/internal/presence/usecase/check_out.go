package usecase

import (
	"context"
	"log/slog"
	"time"

	"parkir-pintar/services/presence/internal/presence/model"
	"parkir-pintar/services/presence/pkg/apperror"
	"parkir-pintar/services/presence/pkg/logger"
)

func (u *PresenceUsecase) CheckOut(ctx context.Context, req model.CheckOutRequest) (*model.CheckOutResponse, *apperror.AppError) {
	// Fetch and validate session ownership
	session, appErr := u.repo.GetSessionForCheckOut(ctx, req.SessionID, req.DriverID)
	if appErr != nil {
		return nil, appErr
	}

	if session.Status != model.SessionStatusActive {
		return nil, apperror.New("conflict", "session is not in ACTIVE status")
	}

	// Complete checkout atomically
	checkedOutAt := req.CheckedOutAt
	if checkedOutAt.IsZero() {
		checkedOutAt = time.Now()
	}

	if appErr := u.repo.CompleteCheckOut(ctx, session.ID, session.SpotID, session.ReservationID, checkedOutAt); appErr != nil {
		return nil, appErr
	}

	// Call Billing Service to calculate invoice and create QRIS payment
	invoiceResult, appErr := u.billingClient.CalculateAndCreateInvoice(ctx,
		session.ID,
		session.ReservationID,
		req.DriverID,
		session.CheckedInAt,
		checkedOutAt,
	)
	if appErr != nil {
		logger.Error(ctx, "CheckOut: billing service call failed",
			slog.String("session_id", session.ID),
			slog.String("error", appErr.Error()),
		)
		return nil, appErr
	}

	logger.Info(ctx, "CheckOut: invoice created",
		slog.String("session_id", session.ID),
		slog.String("invoice_id", invoiceResult.InvoiceID),
	)

	return &model.CheckOutResponse{
		SessionID:    session.ID,
		InvoiceID:    invoiceResult.InvoiceID,
		CheckedOutAt: checkedOutAt,
		Status:       model.SessionStatusCompleted,
		TotalIDR:     invoiceResult.TotalIDR,
		QRCodeURL:    invoiceResult.QRCodeURL,
	}, nil
}
