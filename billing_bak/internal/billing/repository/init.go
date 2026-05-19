package repository

import (
	"context"

	"parkir-pintar/services/billing/internal/billing/model"
	"parkir-pintar/services/billing/pkg/apperror"

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

type Billing interface {
	// GetByIdempotencyKey returns an existing invoice by idempotency key (for duplicate detection)
	GetByIdempotencyKey(ctx context.Context, key string) (*model.Invoice, *apperror.AppError)

	// CreateInvoice inserts a new invoice record
	CreateInvoice(ctx context.Context, invoice *model.Invoice) (*model.Invoice, *apperror.AppError)

	// GetByID returns an invoice by its UUID
	GetByID(ctx context.Context, invoiceID string) (*model.Invoice, *apperror.AppError)

	// GetByIDForDriver returns an invoice by UUID, validating it belongs to the driver
	GetByIDForDriver(ctx context.Context, invoiceID, driverID string) (*model.Invoice, *apperror.AppError)

	// UpdateStatusToPendingPayment resets invoice status to PENDING_PAYMENT for retry
	UpdateStatusToPendingPayment(ctx context.Context, invoiceID string) *apperror.AppError

	// UpdateInvoiceStatus updates invoice status to PAID or PAYMENT_FAILED
	UpdateInvoiceStatus(ctx context.Context, invoiceID string, status model.InvoiceStatus) *apperror.AppError
}

type BillingRepository struct {
	db DB
}

func NewBilling(db DB) Billing {
	return &BillingRepository{db: db}
}
