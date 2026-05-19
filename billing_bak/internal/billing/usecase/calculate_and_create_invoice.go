package usecase

import (
	"context"
	"log/slog"

	paymentpb "parkir-pintar/services/billing/gen/payment/v1"
	"parkir-pintar/services/billing/internal/billing/model"
	"parkir-pintar/services/billing/pkg/apperror"
	"parkir-pintar/services/billing/pkg/logger"
	"parkir-pintar/services/billing/pkg/pricing"

	"github.com/google/uuid"
)

func (u *BillingUsecase) CalculateAndCreateInvoice(ctx context.Context, req model.CreateInvoiceRequest) (*model.CreateInvoiceResponse, *apperror.AppError) {
	// Idempotency check
	existing, appErr := u.repo.GetByIdempotencyKey(ctx, req.IdempotencyKey)
	if appErr != nil {
		return nil, appErr
	}
	if existing != nil {
		logger.Info(ctx, "CalculateAndCreateInvoice: duplicate request, returning cached response",
			slog.String("idempotency_key", req.IdempotencyKey),
			slog.String("invoice_id", existing.ID),
		)
		return &model.CreateInvoiceResponse{
			InvoiceID:       existing.ID,
			BookingFeeIDR:   existing.BookingFeeIDR,
			ParkingFeeIDR:   existing.ParkingFeeIDR,
			OvernightFeeIDR: existing.OvernightFeeIDR,
			TotalIDR:        existing.TotalIDR,
			QRCodeURL:       existing.QRCodeURL,
		}, nil
	}

	// Calculate fees
	result := pricing.Calculate(req.CheckedInAt, req.CheckedOutAt)

	// Call Payment Service to create QRIS parking fee payment
	// Generate a UUID for the payment idempotency key — tied to this invoice's
	// idempotency key so replaying the same invoice request reuses the same payment
	paymentIdemKey := uuid.NewSHA1(uuid.NameSpaceURL, []byte("parking:"+req.IdempotencyKey)).String()
	paymentResult, appErr := u.paymentClient.CreatePayment(ctx,
		paymentIdemKey,
		req.SessionID, // reference_id = session_id for parking fee
		req.DriverID,
		paymentpb.PaymentType_PAYMENT_TYPE_PARKING_FEE,
		result.TotalIDR,
	)
	if appErr != nil {
		logger.Error(ctx, "CalculateAndCreateInvoice: payment service call failed",
			slog.String("session_id", req.SessionID),
			slog.String("error", appErr.Error()),
		)
		return nil, appErr
	}

	logger.Info(ctx, "CalculateAndCreateInvoice: payment created",
		slog.String("session_id", req.SessionID),
		slog.String("payment_id", paymentResult.PaymentID),
	)

	invoice := &model.Invoice{
		IdempotencyKey:  req.IdempotencyKey,
		SessionID:       req.SessionID,
		ReservationID:   req.ReservationID,
		BookingFeeIDR:   pricing.BookingFeeIDR,
		ParkingFeeIDR:   result.ParkingFeeIDR,
		OvernightFeeIDR: result.OvernightFeeIDR,
		TotalIDR:        result.TotalIDR,
		QRCodeURL:       paymentResult.QRCodeURL,
	}
	created, appErr := u.repo.CreateInvoice(ctx, invoice)
	if appErr != nil {
		return nil, appErr
	}

	return &model.CreateInvoiceResponse{
		InvoiceID:       created.ID,
		BookingFeeIDR:   created.BookingFeeIDR,
		ParkingFeeIDR:   created.ParkingFeeIDR,
		OvernightFeeIDR: created.OvernightFeeIDR,
		TotalIDR:        created.TotalIDR,
		QRCodeURL:       created.QRCodeURL,
	}, nil
}
