package repository

import (
	"context"
	"fmt"
	"testing"
	"time"

	"parkir-pintar/services/payment/internal/payment/model"

	pgxmock "github.com/pashagolub/pgxmock/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testPaymentID  = "110e8400-e29b-41d4-a716-446655440001"
	testIdemKey    = "220e8400-e29b-41d4-a716-446655440002"
	testRefID      = "330e8400-e29b-41d4-a716-446655440003"
)

func newRepo(t *testing.T) (pgxmock.PgxPoolIface, *PaymentRepository) {
	t.Helper()
	db, err := pgxmock.NewPool()
	require.NoError(t, err)
	return db, &PaymentRepository{db: db}
}

func paymentRow() *pgxmock.Rows {
	return pgxmock.NewRows([]string{
		"id", "idempotency_key", "reference_id", "payment_type", "amount_idr",
		"method", "status", "gateway_ref", "qr_code_url", "created_at", "updated_at",
	}).AddRow(
		testPaymentID, testIdemKey, testRefID,
		model.PaymentTypeBookingFee, int64(5000),
		model.PaymentMethodQRIS, model.PaymentStatusPending,
		"", "https://qr.example.com", time.Now(), time.Now(),
	)
}

// ── GetByIdempotencyKey ───────────────────────────────────────────────────────

func TestGetByIdempotencyKey_Found(t *testing.T) {
	db, repo := newRepo(t)

	db.ExpectQuery(`SELECT id`).WithArgs(testIdemKey).WillReturnRows(paymentRow())

	p, appErr := repo.GetByIdempotencyKey(context.Background(), testIdemKey)

	require.Nil(t, appErr)
	require.NotNil(t, p)
	assert.Equal(t, testPaymentID, p.ID)
	assert.Equal(t, "https://qr.example.com", p.QRCodeURL)
	assert.NoError(t, db.ExpectationsWereMet())
}

func TestGetByIdempotencyKey_NotFound(t *testing.T) {
	db, repo := newRepo(t)

	db.ExpectQuery(`SELECT id`).WithArgs(testIdemKey).
		WillReturnRows(pgxmock.NewRows([]string{"id"}))

	p, appErr := repo.GetByIdempotencyKey(context.Background(), testIdemKey)

	require.Nil(t, appErr)
	assert.Nil(t, p)
	assert.NoError(t, db.ExpectationsWereMet())
}

func TestGetByIdempotencyKey_DBError(t *testing.T) {
	db, repo := newRepo(t)

	db.ExpectQuery(`SELECT id`).WithArgs(testIdemKey).
		WillReturnError(fmt.Errorf("connection refused"))

	_, appErr := repo.GetByIdempotencyKey(context.Background(), testIdemKey)

	require.NotNil(t, appErr)
	assert.Equal(t, "db_error", appErr.ErrorCode)
	assert.NoError(t, db.ExpectationsWereMet())
}

// ── CreatePayment ─────────────────────────────────────────────────────────────

func TestCreatePayment_Success(t *testing.T) {
	db, repo := newRepo(t)

	now := time.Now()
	db.ExpectQuery(`INSERT INTO payments`).
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(),
			pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(),
			pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{"created_at", "updated_at"}).
			AddRow(now, now))

	p, appErr := repo.CreatePayment(context.Background(), &model.Payment{
		ID:             testPaymentID,
		IdempotencyKey: testIdemKey,
		ReferenceID:    testRefID,
		PaymentType:    model.PaymentTypeBookingFee,
		AmountIDR:      5000,
		Method:         model.PaymentMethodQRIS,
		QRCodeURL:      "https://qr.example.com",
	})

	require.Nil(t, appErr)
	assert.Equal(t, testPaymentID, p.ID)
	assert.Equal(t, model.PaymentStatusPending, p.Status)
	assert.NoError(t, db.ExpectationsWereMet())
}

func TestCreatePayment_DBError(t *testing.T) {
	db, repo := newRepo(t)

	db.ExpectQuery(`INSERT INTO payments`).
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(),
			pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(),
			pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnError(fmt.Errorf("insert failed"))

	_, appErr := repo.CreatePayment(context.Background(), &model.Payment{
		ID:             testPaymentID,
		IdempotencyKey: testIdemKey,
		ReferenceID:    testRefID,
	})

	require.NotNil(t, appErr)
	assert.Equal(t, "db_error", appErr.ErrorCode)
	assert.NoError(t, db.ExpectationsWereMet())
}

// ── GetByID ───────────────────────────────────────────────────────────────────

func TestGetByID_Found(t *testing.T) {
	db, repo := newRepo(t)

	db.ExpectQuery(`SELECT id`).WithArgs(testPaymentID).WillReturnRows(paymentRow())

	p, appErr := repo.GetByID(context.Background(), testPaymentID)

	require.Nil(t, appErr)
	assert.Equal(t, testPaymentID, p.ID)
	assert.NoError(t, db.ExpectationsWereMet())
}

func TestGetByID_NotFound(t *testing.T) {
	db, repo := newRepo(t)

	db.ExpectQuery(`SELECT id`).WithArgs(testPaymentID).
		WillReturnRows(pgxmock.NewRows([]string{"id"}))

	_, appErr := repo.GetByID(context.Background(), testPaymentID)

	require.NotNil(t, appErr)
	assert.Equal(t, "not_found", appErr.ErrorCode)
	assert.NoError(t, db.ExpectationsWereMet())
}

func TestGetByID_DBError(t *testing.T) {
	db, repo := newRepo(t)

	db.ExpectQuery(`SELECT id`).WithArgs(testPaymentID).
		WillReturnError(fmt.Errorf("connection refused"))

	_, appErr := repo.GetByID(context.Background(), testPaymentID)

	require.NotNil(t, appErr)
	assert.Equal(t, "db_error", appErr.ErrorCode)
	assert.NoError(t, db.ExpectationsWereMet())
}

// ── GetByReferenceAndType ─────────────────────────────────────────────────────

func TestGetByReferenceAndType_Found(t *testing.T) {
	db, repo := newRepo(t)

	db.ExpectQuery(`SELECT id`).
		WithArgs(testRefID, model.PaymentTypeBookingFee).
		WillReturnRows(paymentRow())

	p, appErr := repo.GetByReferenceAndType(context.Background(), testRefID, model.PaymentTypeBookingFee)

	require.Nil(t, appErr)
	assert.Equal(t, testPaymentID, p.ID)
	assert.NoError(t, db.ExpectationsWereMet())
}

func TestGetByReferenceAndType_NotFound(t *testing.T) {
	db, repo := newRepo(t)

	db.ExpectQuery(`SELECT id`).
		WithArgs(testRefID, model.PaymentTypeBookingFee).
		WillReturnRows(pgxmock.NewRows([]string{"id"}))

	_, appErr := repo.GetByReferenceAndType(context.Background(), testRefID, model.PaymentTypeBookingFee)

	require.NotNil(t, appErr)
	assert.Equal(t, "not_found", appErr.ErrorCode)
	assert.NoError(t, db.ExpectationsWereMet())
}

// ── UpdateStatus ──────────────────────────────────────────────────────────────

func TestUpdateStatus_Success(t *testing.T) {
	db, repo := newRepo(t)

	db.ExpectExec(`UPDATE payments`).
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	appErr := repo.UpdateStatus(context.Background(), testPaymentID, model.PaymentStatusSuccess, "GW-REF-123")

	require.Nil(t, appErr)
	assert.NoError(t, db.ExpectationsWereMet())
}

func TestUpdateStatus_DBError(t *testing.T) {
	db, repo := newRepo(t)

	db.ExpectExec(`UPDATE payments`).
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnError(fmt.Errorf("update failed"))

	appErr := repo.UpdateStatus(context.Background(), testPaymentID, model.PaymentStatusFailed, "")

	require.NotNil(t, appErr)
	assert.Equal(t, "db_error", appErr.ErrorCode)
	assert.NoError(t, db.ExpectationsWereMet())
}
