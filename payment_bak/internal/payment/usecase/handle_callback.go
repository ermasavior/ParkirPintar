package usecase

import (
	"context"
	"encoding/json"
	"log/slog"

	"parkir-pintar/services/payment/internal/payment/model"
	"parkir-pintar/services/payment/pkg/apperror"
	"parkir-pintar/services/payment/pkg/logger"
)

// HandleCallback processes an inbound webhook from the payment gateway.
func (u *PaymentUsecase) HandleCallback(ctx context.Context, req model.WebhookCallbackRequest) *apperror.AppError {
	// Resolve payment type
	var paymentType model.PaymentType
	switch req.PaymentType {
	case "BOOKING_FEE":
		paymentType = model.PaymentTypeBookingFee
	case "PARKING_FEE":
		paymentType = model.PaymentTypeParkingFee
	default:
		return apperror.New("validation_error", "unknown payment_type: "+req.PaymentType)
	}

	// Look up payment record
	payment, appErr := u.repo.GetByReferenceAndType(ctx, req.ReferenceID, paymentType)
	if appErr != nil {
		return appErr
	}

	// Map gateway status to internal status
	var newStatus model.PaymentStatus
	switch req.Status {
	case "SUCCESS":
		newStatus = model.PaymentStatusSuccess
	case "FAILED":
		newStatus = model.PaymentStatusFailed
	case "EXPIRED":
		newStatus = model.PaymentStatusExpired
	default:
		return apperror.New("validation_error", "unknown status: "+req.Status)
	}

	// Update payment record
	if appErr := u.repo.UpdateStatus(ctx, payment.ID, newStatus, req.GatewayRef); appErr != nil {
		return appErr
	}

	// Publish to NATS
	event := model.NATSPaymentDoneEvent{
		ReferenceID: req.ReferenceID,
		Status:      req.Status,
	}
	payload, err := json.Marshal(event)
	if err != nil {
		logger.Error(ctx, "HandleCallback: failed to marshal NATS event", slog.String("error", err.Error()))
		return apperror.New("internal_error", "failed to marshal event")
	}

	var subject string
	switch paymentType {
	case model.PaymentTypeBookingFee:
		subject = "payment.booking.done"
	case model.PaymentTypeParkingFee:
		subject = "payment.parking.done"
	}

	if err := u.natsConn.Publish(subject, payload); err != nil {
		logger.Error(ctx, "HandleCallback: failed to publish NATS event",
			slog.String("subject", subject),
			slog.String("error", err.Error()),
		)
		return apperror.New("nats_error", "failed to publish payment event")
	}

	logger.Info(ctx, "HandleCallback: event published",
		slog.String("subject", subject),
		slog.String("reference_id", req.ReferenceID),
		slog.String("status", req.Status),
	)

	return nil
}
