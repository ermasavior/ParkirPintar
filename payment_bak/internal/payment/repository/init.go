package repository

import (
	"context"

	"parkir-pintar/services/payment/internal/payment/model"
	"parkir-pintar/services/payment/pkg/apperror"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type DB interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Begin(ctx context.Context) (pgx.Tx, error)
}

var _ DB = (*pgxpool.Pool)(nil)

type Payment interface {
	// GetByIdempotencyKey returns an existing payment by idempotency key
	GetByIdempotencyKey(ctx context.Context, key string) (*model.Payment, *apperror.AppError)

	// CreatePayment inserts a new payment record
	CreatePayment(ctx context.Context, payment *model.Payment) (*model.Payment, *apperror.AppError)

	// GetByID returns a payment by its UUID
	GetByID(ctx context.Context, paymentID string) (*model.Payment, *apperror.AppError)

	// GetByReferenceAndType returns a payment by reference_id + payment_type
	GetByReferenceAndType(ctx context.Context, referenceID string, paymentType model.PaymentType) (*model.Payment, *apperror.AppError)

	// UpdateStatus updates payment status and optionally sets gateway_ref
	UpdateStatus(ctx context.Context, paymentID string, status model.PaymentStatus, gatewayRef string) *apperror.AppError
}

type PaymentRepository struct {
	db DB
}

func NewPayment(db DB) Payment {
	return &PaymentRepository{db: db}
}
