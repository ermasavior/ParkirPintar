package repository

import (
	"context"
	"fmt"
	"testing"
	"time"

	"parkir-pintar/services/billing/internal/billing/model"

	pgxmock "github.com/pashagolub/pgxmock/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testInvoiceID     = "110e8400-e29b-41d4-a716-446655440001"
	testIdemKey       = "220e8400-e29b-41d4-a716-446655440002"
	testSessionID     = "330e8400-e29b-41d4-a716-446655440003"
	testReservationID = "440e8400-e29b-41d4-a716-446655440004"
	testDriverID      = "550e8400-e29b-41d4-a716-446655440005"
)

func newRepo(t *testing.T) (pgxmock.PgxPoolIface, *BillingRepository) {
	t.Helper()
	db, err := pgxmock.NewPool()
	require.NoError(t, err)
	return db, &BillingRepository{db: db}
}

func invoiceRow() *pgxmock.Rows {
	return pgxmock.NewRows([]string{
		"id", "idempotency_key", "session_id", "reservation_id", "status",
		"booking_fee_idr", "parking_fee_idr", "overnight_fee_idr", "total_idr", "qr_code_url", "created_at",
	}).AddRow(
		testInvoiceID, testIdemKey, testSessionID, testReservationID,
		model.InvoiceStatusPendingPayment,
		int64(5000), int64(10000), int64(0), int64(15000),
		"https://qr.example.com", time.Now(),
	)
}

// ── GetByIdempotencyKey ───────────────────────────────────────────────────────

func TestGetByIdempotencyKey_Found(t *testing.T) {
	db, repo := newRepo(t)

	db.ExpectQuery(`SELECT id`).
		WithArgs(testIdemKey).
		WillReturnRows(invoiceRow())

	inv, appErr := repo.GetByIdempotencyKey(context.Background(), testIdemKey)

	require.Nil(t, appErr)
	require.NotNil(t, inv)
	assert.Equal(t, testInvoiceID, inv.ID)
	assert.Equal(t, int64(15000), inv.TotalIDR)
	assert.NoError(t, db.ExpectationsWereMet())
}

func TestGetByIdempotencyKey_NotFound(t *testing.T) {
	db, repo := newRepo(t)

	db.ExpectQuery(`SELECT id`).
		WithArgs(testIdemKey).
		WillReturnRows(pgxmock.NewRows([]string{"id"}))

	inv, appErr := repo.GetByIdempotencyKey(context.Background(), testIdemKey)

	require.Nil(t, appErr)
	assert.Nil(t, inv)
	assert.NoError(t, db.ExpectationsWereMet())
}

func TestGetByIdempotencyKey_DBError(t *testing.T) {
	db, repo := newRepo(t)

	db.ExpectQuery(`SELECT id`).
		WithArgs(testIdemKey).
		WillReturnError(fmt.Errorf("connection refused"))

	_, appErr := repo.GetByIdempotencyKey(context.Background(), testIdemKey)

	require.NotNil(t, appErr)
	assert.Equal(t, "db_error", appErr.ErrorCode)
	assert.NoError(t, db.ExpectationsWereMet())
}

// ── CreateInvoice ─────────────────────────────────────────────────────────────

func TestCreateInvoice_Success(t *testing.T) {
	db, repo := newRepo(t)

	now := time.Now()
	db.ExpectQuery(`INSERT INTO invoices`).
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(),
			pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{"id", "created_at"}).AddRow(testInvoiceID, now))

	inv, appErr := repo.CreateInvoice(context.Background(), &model.Invoice{
		IdempotencyKey:  testIdemKey,
		SessionID:       testSessionID,
		ReservationID:   testReservationID,
		BookingFeeIDR:   5000,
		ParkingFeeIDR:   10000,
		OvernightFeeIDR: 0,
		TotalIDR:        15000,
		QRCodeURL:       "https://qr.example.com",
	})

	require.Nil(t, appErr)
	assert.Equal(t, testInvoiceID, inv.ID)
	assert.Equal(t, model.InvoiceStatusPendingPayment, inv.Status)
	assert.NoError(t, db.ExpectationsWereMet())
}

func TestCreateInvoice_DBError(t *testing.T) {
	db, repo := newRepo(t)

	db.ExpectQuery(`INSERT INTO invoices`).
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(),
			pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnError(fmt.Errorf("insert failed"))

	_, appErr := repo.CreateInvoice(context.Background(), &model.Invoice{
		IdempotencyKey: testIdemKey,
		SessionID:      testSessionID,
		ReservationID:  testReservationID,
	})

	require.NotNil(t, appErr)
	assert.Equal(t, "db_error", appErr.ErrorCode)
	assert.NoError(t, db.ExpectationsWereMet())
}

// ── GetByID ───────────────────────────────────────────────────────────────────

func TestGetByID_Found(t *testing.T) {
	db, repo := newRepo(t)

	db.ExpectQuery(`SELECT id`).
		WithArgs(testInvoiceID).
		WillReturnRows(invoiceRow())

	inv, appErr := repo.GetByID(context.Background(), testInvoiceID)

	require.Nil(t, appErr)
	assert.Equal(t, testInvoiceID, inv.ID)
	assert.NoError(t, db.ExpectationsWereMet())
}

func TestGetByID_NotFound(t *testing.T) {
	db, repo := newRepo(t)

	db.ExpectQuery(`SELECT id`).
		WithArgs(testInvoiceID).
		WillReturnRows(pgxmock.NewRows([]string{"id"}))

	_, appErr := repo.GetByID(context.Background(), testInvoiceID)

	require.NotNil(t, appErr)
	assert.Equal(t, "not_found", appErr.ErrorCode)
	assert.NoError(t, db.ExpectationsWereMet())
}

func TestGetByID_DBError(t *testing.T) {
	db, repo := newRepo(t)

	db.ExpectQuery(`SELECT id`).
		WithArgs(testInvoiceID).
		WillReturnError(fmt.Errorf("connection refused"))

	_, appErr := repo.GetByID(context.Background(), testInvoiceID)

	require.NotNil(t, appErr)
	assert.Equal(t, "db_error", appErr.ErrorCode)
	assert.NoError(t, db.ExpectationsWereMet())
}

// ── GetByIDForDriver ──────────────────────────────────────────────────────────

func TestGetByIDForDriver_Found(t *testing.T) {
	db, repo := newRepo(t)

	db.ExpectQuery(`SELECT i\.id`).
		WithArgs(testInvoiceID, testDriverID).
		WillReturnRows(invoiceRow())

	inv, appErr := repo.GetByIDForDriver(context.Background(), testInvoiceID, testDriverID)

	require.Nil(t, appErr)
	assert.Equal(t, testInvoiceID, inv.ID)
	assert.NoError(t, db.ExpectationsWereMet())
}

func TestGetByIDForDriver_NotFound(t *testing.T) {
	db, repo := newRepo(t)

	db.ExpectQuery(`SELECT i\.id`).
		WithArgs(testInvoiceID, testDriverID).
		WillReturnRows(pgxmock.NewRows([]string{"id"}))

	_, appErr := repo.GetByIDForDriver(context.Background(), testInvoiceID, testDriverID)

	require.NotNil(t, appErr)
	assert.Equal(t, "not_found", appErr.ErrorCode)
	assert.NoError(t, db.ExpectationsWereMet())
}

func TestGetByIDForDriver_DBError(t *testing.T) {
	db, repo := newRepo(t)

	db.ExpectQuery(`SELECT i\.id`).
		WithArgs(testInvoiceID, testDriverID).
		WillReturnError(fmt.Errorf("connection refused"))

	_, appErr := repo.GetByIDForDriver(context.Background(), testInvoiceID, testDriverID)

	require.NotNil(t, appErr)
	assert.Equal(t, "db_error", appErr.ErrorCode)
	assert.NoError(t, db.ExpectationsWereMet())
}

// ── UpdateStatusToPendingPayment ──────────────────────────────────────────────

func TestUpdateStatusToPendingPayment_Success(t *testing.T) {
	db, repo := newRepo(t)

	db.ExpectExec(`UPDATE invoices`).
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	appErr := repo.UpdateStatusToPendingPayment(context.Background(), testInvoiceID)

	require.Nil(t, appErr)
	assert.NoError(t, db.ExpectationsWereMet())
}

func TestUpdateStatusToPendingPayment_DBError(t *testing.T) {
	db, repo := newRepo(t)

	db.ExpectExec(`UPDATE invoices`).
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnError(fmt.Errorf("update failed"))

	appErr := repo.UpdateStatusToPendingPayment(context.Background(), testInvoiceID)

	require.NotNil(t, appErr)
	assert.Equal(t, "db_error", appErr.ErrorCode)
	assert.NoError(t, db.ExpectationsWereMet())
}
