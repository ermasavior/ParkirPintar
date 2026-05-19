package repository

import (
	"context"
	"errors"
	"log/slog"

	"parkir-pintar/services/billing/internal/billing/model"
	"parkir-pintar/services/billing/pkg/apperror"
	"parkir-pintar/services/billing/pkg/logger"

	"github.com/jackc/pgx/v5"
)

func (r *BillingRepository) GetByIdempotencyKey(ctx context.Context, key string) (*model.Invoice, *apperror.AppError) {
	query := `SELECT id, idempotency_key, session_id, reservation_id, status,
	           booking_fee_idr, parking_fee_idr, overnight_fee_idr, total_idr, qr_code_url, created_at
	           FROM invoices WHERE idempotency_key = $1`

	var inv model.Invoice
	err := r.db.QueryRow(ctx, query, key).Scan(
		&inv.ID,
		&inv.IdempotencyKey,
		&inv.SessionID,
		&inv.ReservationID,
		&inv.Status,
		&inv.BookingFeeIDR,
		&inv.ParkingFeeIDR,
		&inv.OvernightFeeIDR,
		&inv.TotalIDR,
		&inv.QRCodeURL,
		&inv.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		logger.Error(ctx, "GetByIdempotencyKey failed", slog.String("error", err.Error()))
		return nil, apperror.New("db_error", "failed to query invoice by idempotency key")
	}
	return &inv, nil
}

func (r *BillingRepository) CreateInvoice(ctx context.Context, invoice *model.Invoice) (*model.Invoice, *apperror.AppError) {
	query := `INSERT INTO invoices
	  (idempotency_key, session_id, reservation_id, status,
	   booking_fee_idr, parking_fee_idr, overnight_fee_idr, total_idr, qr_code_url)
	  VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	  RETURNING id, created_at`

	err := r.db.QueryRow(ctx, query,
		invoice.IdempotencyKey,
		invoice.SessionID,
		invoice.ReservationID,
		model.InvoiceStatusPendingPayment,
		invoice.BookingFeeIDR,
		invoice.ParkingFeeIDR,
		invoice.OvernightFeeIDR,
		invoice.TotalIDR,
		invoice.QRCodeURL,
	).Scan(&invoice.ID, &invoice.CreatedAt)
	if err != nil {
		logger.Error(ctx, "CreateInvoice failed", slog.String("error", err.Error()))
		return nil, apperror.New("db_error", "failed to create invoice")
	}

	invoice.Status = model.InvoiceStatusPendingPayment
	return invoice, nil
}

func (r *BillingRepository) GetByID(ctx context.Context, invoiceID string) (*model.Invoice, *apperror.AppError) {
	query := `SELECT id, idempotency_key, session_id, reservation_id, status,
	           booking_fee_idr, parking_fee_idr, overnight_fee_idr, total_idr, qr_code_url, created_at
	           FROM invoices WHERE id = $1`

	var inv model.Invoice
	err := r.db.QueryRow(ctx, query, invoiceID).Scan(
		&inv.ID,
		&inv.IdempotencyKey,
		&inv.SessionID,
		&inv.ReservationID,
		&inv.Status,
		&inv.BookingFeeIDR,
		&inv.ParkingFeeIDR,
		&inv.OvernightFeeIDR,
		&inv.TotalIDR,
		&inv.QRCodeURL,
		&inv.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperror.New("not_found", "invoice not found")
		}
		logger.Error(ctx, "GetByID failed", slog.String("error", err.Error()))
		return nil, apperror.New("db_error", "failed to query invoice")
	}
	return &inv, nil
}

func (r *BillingRepository) GetByIDForDriver(ctx context.Context, invoiceID, driverID string) (*model.Invoice, *apperror.AppError) {
	query := `SELECT i.id, i.idempotency_key, i.session_id, i.reservation_id, i.status,
	           i.booking_fee_idr, i.parking_fee_idr, i.overnight_fee_idr, i.total_idr, i.qr_code_url, i.created_at
	           FROM invoices i
	           JOIN sessions s ON s.id = i.session_id
	           WHERE i.id = $1 AND s.driver_id = $2`

	var inv model.Invoice
	err := r.db.QueryRow(ctx, query, invoiceID, driverID).Scan(
		&inv.ID,
		&inv.IdempotencyKey,
		&inv.SessionID,
		&inv.ReservationID,
		&inv.Status,
		&inv.BookingFeeIDR,
		&inv.ParkingFeeIDR,
		&inv.OvernightFeeIDR,
		&inv.TotalIDR,
		&inv.QRCodeURL,
		&inv.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperror.New("not_found", "invoice not found or does not belong to driver")
		}
		logger.Error(ctx, "GetByIDForDriver failed", slog.String("error", err.Error()))
		return nil, apperror.New("db_error", "failed to query invoice")
	}
	return &inv, nil
}

func (r *BillingRepository) UpdateStatusToPendingPayment(ctx context.Context, invoiceID string) *apperror.AppError {
	_, err := r.db.Exec(ctx,
		`UPDATE invoices SET status = $1 WHERE id = $2`,
		model.InvoiceStatusPendingPayment, invoiceID,
	)
	if err != nil {
		logger.Error(ctx, "UpdateStatusToPendingPayment failed", slog.String("error", err.Error()))
		return apperror.New("db_error", "failed to update invoice status")
	}
	return nil
}

func (r *BillingRepository) UpdateInvoiceStatus(ctx context.Context, invoiceID string, status model.InvoiceStatus) *apperror.AppError {
	_, err := r.db.Exec(ctx,
		`UPDATE invoices SET status = $1 WHERE id = $2`,
		status, invoiceID,
	)
	if err != nil {
		logger.Error(ctx, "UpdateInvoiceStatus failed", slog.String("error", err.Error()))
		return apperror.New("db_error", "failed to update invoice status")
	}
	return nil
}
