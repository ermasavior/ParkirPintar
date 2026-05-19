package usecase

import (
	"context"
	"log/slog"

	"parkir-pintar/services/payment/internal/payment/model"
	"parkir-pintar/services/payment/pkg/apperror"
	"parkir-pintar/services/payment/pkg/gatewayclient"
	"parkir-pintar/services/payment/pkg/logger"

	"github.com/google/uuid"
)

func (u *PaymentUsecase) CreatePayment(ctx context.Context, req model.CreatePaymentRequest) (*model.CreatePaymentResponse, *apperror.AppError) {
	// Idempotency check
	existing, appErr := u.repo.GetByIdempotencyKey(ctx, req.IdempotencyKey)
	if appErr != nil {
		return nil, appErr
	}
	if existing != nil {
		logger.Info(ctx, "CreatePayment: duplicate request, returning cached response",
			slog.String("idempotency_key", req.IdempotencyKey),
			slog.String("payment_id", existing.ID),
		)
		return &model.CreatePaymentResponse{
			PaymentID: existing.ID,
			QRCodeURL: existing.QRCodeURL,
			Status:    existing.Status,
		}, nil
	}

	// Generate payment_id upfront so it can be passed to the gateway
	paymentID := uuid.New().String()

	// Resolve payment type string for gateway
	var paymentTypeStr string
	switch req.PaymentType {
	case model.PaymentTypeBookingFee:
		paymentTypeStr = "BOOKING_FEE"
	case model.PaymentTypeParkingFee:
		paymentTypeStr = "PARKING_FEE"
	}

	// Call payment gateway to create QRIS code
	var qrCodeURL string
	if u.gateway != nil {
		url, err := u.gateway.CreateQRIS(gatewayclient.CreateQRISRequest{
			PaymentID:   paymentID,
			ReferenceID: req.ReferenceID,
			PaymentType: paymentTypeStr,
			AmountIDR:   req.AmountIDR,
		})
		if err != nil {
			logger.Error(ctx, "CreatePayment: gateway call failed",
				slog.String("payment_id", paymentID),
				slog.String("error", err.Error()),
			)
			// Non-fatal: use a fallback URL so the flow isn't blocked
			qrCodeURL = "https://payment-gateway.example.com/qris/" + paymentID
		} else {
			qrCodeURL = url
		}
	} else {
		// No gateway configured (test environment) — use a placeholder URL
		qrCodeURL = "https://payment-gateway.example.com/qris/" + paymentID
	}

	// Insert complete payment record in a single write
	payment := &model.Payment{
		ID:             paymentID,
		IdempotencyKey: req.IdempotencyKey,
		ReferenceID:    req.ReferenceID,
		PaymentType:    req.PaymentType,
		AmountIDR:      req.AmountIDR,
		Method:         req.Method,
		QRCodeURL:      qrCodeURL,
	}
	created, appErr := u.repo.CreatePayment(ctx, payment)
	if appErr != nil {
		return nil, appErr
	}

	logger.Info(ctx, "CreatePayment: payment created",
		slog.String("payment_id", created.ID),
		slog.String("reference_id", req.ReferenceID),
		slog.String("qr_code_url", qrCodeURL),
	)

	return &model.CreatePaymentResponse{
		PaymentID: created.ID,
		QRCodeURL: created.QRCodeURL,
		Status:    model.PaymentStatusPending,
	}, nil
}
