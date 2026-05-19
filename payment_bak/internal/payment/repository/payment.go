package repository

import (
	"context"
	"errors"
	"log/slog"

	"parkir-pintar/services/payment/internal/payment/model"
	"parkir-pintar/services/payment/pkg/apperror"
	"parkir-pintar/services/payment/pkg/logger"

	"github.com/jackc/pgx/v5"
)

// GetByIdempotencyKey returns an existing payment by idempotency key
func (r *PaymentRepository) GetByIdempotencyKey(ctx context.Context, key string) (*model.Payment, *apperror.AppError) {
	query := `SELECT id, idempotency_key, reference_id, payment_type, amount_idr,
	           method, status, COALESCE(gateway_ref, ''), qr_code_url, created_at, updated_at
	           FROM payments WHERE idempotency_key = $1`

	var p model.Payment
	err := r.db.QueryRow(ctx, query, key).Scan(
		&p.ID, &p.IdempotencyKey, &p.ReferenceID, &p.PaymentType, &p.AmountIDR,
		&p.Method, &p.Status, &p.GatewayRef, &p.QRCodeURL, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		logger.Error(ctx, "GetByIdempotencyKey failed", slog.String("error", err.Error()))
		return nil, apperror.New("db_error", "failed to query payment by idempotency key")
	}
	return &p, nil
}

// CreatePayment inserts a new payment record with a pre-generated ID
func (r *PaymentRepository) CreatePayment(ctx context.Context, payment *model.Payment) (*model.Payment, *apperror.AppError) {
	query := `INSERT INTO payments
	  (id, idempotency_key, reference_id, payment_type, amount_idr, method, status, qr_code_url)
	  VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	  RETURNING created_at, updated_at`

	err := r.db.QueryRow(ctx, query,
		payment.ID,
		payment.IdempotencyKey,
		payment.ReferenceID,
		payment.PaymentType,
		payment.AmountIDR,
		payment.Method,
		model.PaymentStatusPending,
		payment.QRCodeURL,
	).Scan(&payment.CreatedAt, &payment.UpdatedAt)
	if err != nil {
		logger.Error(ctx, "CreatePayment failed", slog.String("error", err.Error()))
		return nil, apperror.New("db_error", "failed to create payment")
	}

	payment.Status = model.PaymentStatusPending
	return payment, nil
}

// GetByID returns a payment by its UUID
func (r *PaymentRepository) GetByID(ctx context.Context, paymentID string) (*model.Payment, *apperror.AppError) {
	query := `SELECT id, idempotency_key, reference_id, payment_type, amount_idr,
	           method, status, COALESCE(gateway_ref, ''), qr_code_url, created_at, updated_at
	           FROM payments WHERE id = $1`

	var p model.Payment
	err := r.db.QueryRow(ctx, query, paymentID).Scan(
		&p.ID, &p.IdempotencyKey, &p.ReferenceID, &p.PaymentType, &p.AmountIDR,
		&p.Method, &p.Status, &p.GatewayRef, &p.QRCodeURL, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperror.New("not_found", "payment not found")
		}
		logger.Error(ctx, "GetByID failed", slog.String("error", err.Error()))
		return nil, apperror.New("db_error", "failed to query payment")
	}
	return &p, nil
}

// GetByReferenceAndType returns a payment by reference_id + payment_type
func (r *PaymentRepository) GetByReferenceAndType(ctx context.Context, referenceID string, paymentType model.PaymentType) (*model.Payment, *apperror.AppError) {
	query := `SELECT id, idempotency_key, reference_id, payment_type, amount_idr,
	           method, status, COALESCE(gateway_ref, ''), qr_code_url, created_at, updated_at
	           FROM payments WHERE reference_id = $1 AND payment_type = $2
	           ORDER BY created_at DESC LIMIT 1`

	var p model.Payment
	err := r.db.QueryRow(ctx, query, referenceID, paymentType).Scan(
		&p.ID, &p.IdempotencyKey, &p.ReferenceID, &p.PaymentType, &p.AmountIDR,
		&p.Method, &p.Status, &p.GatewayRef, &p.QRCodeURL, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperror.New("not_found", "payment not found for reference")
		}
		logger.Error(ctx, "GetByReferenceAndType failed", slog.String("error", err.Error()))
		return nil, apperror.New("db_error", "failed to query payment by reference")
	}
	return &p, nil
}

// UpdateStatus updates payment status and optionally sets gateway_ref
func (r *PaymentRepository) UpdateStatus(ctx context.Context, paymentID string, status model.PaymentStatus, gatewayRef string) *apperror.AppError {
	_, err := r.db.Exec(ctx,
		`UPDATE payments SET status = $1, gateway_ref = NULLIF($2, ''), updated_at = NOW() WHERE id = $3`,
		status, gatewayRef, paymentID,
	)
	if err != nil {
		logger.Error(ctx, "UpdateStatus failed", slog.String("error", err.Error()))
		return apperror.New("db_error", "failed to update payment status")
	}
	return nil
}
