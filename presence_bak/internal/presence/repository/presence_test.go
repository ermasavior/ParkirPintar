package repository

import (
	"context"
	"fmt"
	"testing"
	"time"

	"parkir-pintar/services/presence/internal/presence/model"

	pgxmock "github.com/pashagolub/pgxmock/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testSessionID     = "110e8400-e29b-41d4-a716-446655440001"
	testReservationID = "220e8400-e29b-41d4-a716-446655440002"
	testDriverID      = "330e8400-e29b-41d4-a716-446655440003"
	testSpotID        = "440e8400-e29b-41d4-a716-446655440004"
)

func newRepo(t *testing.T) (pgxmock.PgxPoolIface, *PresenceRepository) {
	t.Helper()
	db, err := pgxmock.NewPool()
	require.NoError(t, err)
	return db, &PresenceRepository{db: db}
}

// ── GetReservationForCheckIn ──────────────────────────────────────────────────
// Scan order: id, driver_id, spot_id, status, expires_at

func TestGetReservationForCheckIn_Found(t *testing.T) {
	db, repo := newRepo(t)

	expiresAt := time.Now().Add(1 * time.Hour)
	db.ExpectQuery(`SELECT id`).
		WithArgs(testReservationID, testDriverID).
		WillReturnRows(pgxmock.NewRows([]string{"id", "driver_id", "spot_id", "status", "expires_at"}).
			AddRow(testReservationID, testDriverID, testSpotID, model.ReservationStatusConfirmed, &expiresAt))

	res, appErr := repo.GetReservationForCheckIn(context.Background(), testReservationID, testDriverID)

	require.Nil(t, appErr)
	assert.Equal(t, testReservationID, res.ID)
	assert.Equal(t, testDriverID, res.DriverID)
	assert.Equal(t, testSpotID, res.SpotID)
	assert.Equal(t, model.ReservationStatusConfirmed, res.Status)
	assert.NotNil(t, res.ExpiresAt)
	assert.NoError(t, db.ExpectationsWereMet())
}

func TestGetReservationForCheckIn_NotFound(t *testing.T) {
	db, repo := newRepo(t)

	db.ExpectQuery(`SELECT id`).
		WithArgs(testReservationID, testDriverID).
		WillReturnRows(pgxmock.NewRows([]string{"id", "driver_id", "spot_id", "status", "expires_at"}))

	_, appErr := repo.GetReservationForCheckIn(context.Background(), testReservationID, testDriverID)

	require.NotNil(t, appErr)
	assert.Equal(t, "not_found", appErr.ErrorCode)
	assert.NoError(t, db.ExpectationsWereMet())
}

func TestGetReservationForCheckIn_DBError(t *testing.T) {
	db, repo := newRepo(t)

	db.ExpectQuery(`SELECT id`).
		WithArgs(testReservationID, testDriverID).
		WillReturnError(fmt.Errorf("connection refused"))

	_, appErr := repo.GetReservationForCheckIn(context.Background(), testReservationID, testDriverID)

	require.NotNil(t, appErr)
	assert.Equal(t, "db_error", appErr.ErrorCode)
	assert.NoError(t, db.ExpectationsWereMet())
}

// ── CreateSessionAndMarkCheckedIn ─────────────────────────────────────────────

func TestCreateSessionAndMarkCheckedIn_Success(t *testing.T) {
	db, repo := newRepo(t)

	now := time.Now()
	db.ExpectBegin()
	db.ExpectQuery(`INSERT INTO sessions`).
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{"id", "reservation_id", "driver_id", "spot_id", "status", "checked_in_at"}).
			AddRow(testSessionID, testReservationID, testDriverID, testSpotID, model.SessionStatusActive, now))
	db.ExpectExec(`UPDATE reservations`).
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	db.ExpectCommit()

	session, appErr := repo.CreateSessionAndMarkCheckedIn(context.Background(), testReservationID, testDriverID, testSpotID, now)

	require.Nil(t, appErr)
	assert.Equal(t, testSessionID, session.ID)
	assert.Equal(t, model.SessionStatusActive, session.Status)
	assert.NoError(t, db.ExpectationsWereMet())
}

func TestCreateSessionAndMarkCheckedIn_InsertFails(t *testing.T) {
	db, repo := newRepo(t)

	db.ExpectBegin()
	db.ExpectQuery(`INSERT INTO sessions`).
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnError(fmt.Errorf("insert failed"))
	db.ExpectRollback()

	_, appErr := repo.CreateSessionAndMarkCheckedIn(context.Background(), testReservationID, testDriverID, testSpotID, time.Now())

	require.NotNil(t, appErr)
	assert.Equal(t, "db_error", appErr.ErrorCode)
	assert.NoError(t, db.ExpectationsWereMet())
}

func TestCreateSessionAndMarkCheckedIn_UpdateFails(t *testing.T) {
	db, repo := newRepo(t)

	now := time.Now()
	db.ExpectBegin()
	db.ExpectQuery(`INSERT INTO sessions`).
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{"id", "reservation_id", "driver_id", "spot_id", "status", "checked_in_at"}).
			AddRow(testSessionID, testReservationID, testDriverID, testSpotID, model.SessionStatusActive, now))
	db.ExpectExec(`UPDATE reservations`).
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnError(fmt.Errorf("update failed"))
	db.ExpectRollback()

	_, appErr := repo.CreateSessionAndMarkCheckedIn(context.Background(), testReservationID, testDriverID, testSpotID, now)

	require.NotNil(t, appErr)
	assert.Equal(t, "db_error", appErr.ErrorCode)
	assert.NoError(t, db.ExpectationsWereMet())
}

// ── GetSessionForCheckOut ─────────────────────────────────────────────────────
// Scan order: id, reservation_id, driver_id, spot_id, status, checked_in_at, checked_out_at

func TestGetSessionForCheckOut_Found(t *testing.T) {
	db, repo := newRepo(t)

	now := time.Now()
	db.ExpectQuery(`SELECT id`).
		WithArgs(testSessionID, testDriverID).
		WillReturnRows(pgxmock.NewRows([]string{"id", "reservation_id", "driver_id", "spot_id", "status", "checked_in_at", "checked_out_at"}).
			AddRow(testSessionID, testReservationID, testDriverID, testSpotID, model.SessionStatusActive, now, nil))

	session, appErr := repo.GetSessionForCheckOut(context.Background(), testSessionID, testDriverID)

	require.Nil(t, appErr)
	assert.Equal(t, testSessionID, session.ID)
	assert.Equal(t, model.SessionStatusActive, session.Status)
	assert.Nil(t, session.CheckedOutAt)
	assert.NoError(t, db.ExpectationsWereMet())
}

func TestGetSessionForCheckOut_NotFound(t *testing.T) {
	db, repo := newRepo(t)

	db.ExpectQuery(`SELECT id`).
		WithArgs(testSessionID, testDriverID).
		WillReturnRows(pgxmock.NewRows([]string{"id", "reservation_id", "driver_id", "spot_id", "status", "checked_in_at", "checked_out_at"}))

	_, appErr := repo.GetSessionForCheckOut(context.Background(), testSessionID, testDriverID)

	require.NotNil(t, appErr)
	assert.Equal(t, "not_found", appErr.ErrorCode)
	assert.NoError(t, db.ExpectationsWereMet())
}

// ── CompleteCheckOut ──────────────────────────────────────────────────────────

func TestCompleteCheckOut_Success(t *testing.T) {
	db, repo := newRepo(t)

	now := time.Now()
	db.ExpectBegin()
	db.ExpectExec(`UPDATE sessions`).
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	db.ExpectExec(`UPDATE spots`).
		WithArgs(pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	db.ExpectExec(`UPDATE reservations`).
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	db.ExpectCommit()

	appErr := repo.CompleteCheckOut(context.Background(), testSessionID, testSpotID, testReservationID, now)

	require.Nil(t, appErr)
	assert.NoError(t, db.ExpectationsWereMet())
}

func TestCompleteCheckOut_SessionUpdateFails(t *testing.T) {
	db, repo := newRepo(t)

	db.ExpectBegin()
	db.ExpectExec(`UPDATE sessions`).
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnError(fmt.Errorf("update failed"))
	db.ExpectRollback()

	appErr := repo.CompleteCheckOut(context.Background(), testSessionID, testSpotID, testReservationID, time.Now())

	require.NotNil(t, appErr)
	assert.Equal(t, "db_error", appErr.ErrorCode)
	assert.NoError(t, db.ExpectationsWereMet())
}

// ── GetSessionByID ────────────────────────────────────────────────────────────

func TestGetSessionByID_Found(t *testing.T) {
	db, repo := newRepo(t)

	now := time.Now()
	db.ExpectQuery(`SELECT id`).
		WithArgs(testSessionID).
		WillReturnRows(pgxmock.NewRows([]string{"id", "reservation_id", "driver_id", "spot_id", "status", "checked_in_at", "checked_out_at"}).
			AddRow(testSessionID, testReservationID, testDriverID, testSpotID, model.SessionStatusActive, now, nil))

	session, appErr := repo.GetSessionByID(context.Background(), testSessionID)

	require.Nil(t, appErr)
	assert.Equal(t, testSessionID, session.ID)
	assert.Equal(t, testReservationID, session.ReservationID)
	assert.Equal(t, model.SessionStatusActive, session.Status)
	assert.NoError(t, db.ExpectationsWereMet())
}

func TestGetSessionByID_NotFound(t *testing.T) {
	db, repo := newRepo(t)

	db.ExpectQuery(`SELECT id`).
		WithArgs(testSessionID).
		WillReturnRows(pgxmock.NewRows([]string{"id", "reservation_id", "driver_id", "spot_id", "status", "checked_in_at", "checked_out_at"}))

	_, appErr := repo.GetSessionByID(context.Background(), testSessionID)

	require.NotNil(t, appErr)
	assert.Equal(t, "not_found", appErr.ErrorCode)
	assert.NoError(t, db.ExpectationsWereMet())
}

// ── Additional error paths ────────────────────────────────────────────────────

func TestCreateSessionAndMarkCheckedIn_BeginFails(t *testing.T) {
	db, repo := newRepo(t)

	db.ExpectBegin().WillReturnError(fmt.Errorf("begin failed"))

	_, appErr := repo.CreateSessionAndMarkCheckedIn(context.Background(), testReservationID, testDriverID, testSpotID, time.Now())

	require.NotNil(t, appErr)
	assert.Equal(t, "db_error", appErr.ErrorCode)
	assert.NoError(t, db.ExpectationsWereMet())
}

func TestCreateSessionAndMarkCheckedIn_CommitFails(t *testing.T) {
	db, repo := newRepo(t)

	now := time.Now()
	db.ExpectBegin()
	db.ExpectQuery(`INSERT INTO sessions`).
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{"id", "reservation_id", "driver_id", "spot_id", "status", "checked_in_at"}).
			AddRow(testSessionID, testReservationID, testDriverID, testSpotID, model.SessionStatusActive, now))
	db.ExpectExec(`UPDATE reservations`).
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	db.ExpectCommit().WillReturnError(fmt.Errorf("commit failed"))

	_, appErr := repo.CreateSessionAndMarkCheckedIn(context.Background(), testReservationID, testDriverID, testSpotID, now)

	require.NotNil(t, appErr)
	assert.Equal(t, "db_error", appErr.ErrorCode)
	assert.NoError(t, db.ExpectationsWereMet())
}

func TestGetSessionForCheckOut_DBError(t *testing.T) {
	db, repo := newRepo(t)

	db.ExpectQuery(`SELECT id`).
		WithArgs(testSessionID, testDriverID).
		WillReturnError(fmt.Errorf("connection refused"))

	_, appErr := repo.GetSessionForCheckOut(context.Background(), testSessionID, testDriverID)

	require.NotNil(t, appErr)
	assert.Equal(t, "db_error", appErr.ErrorCode)
	assert.NoError(t, db.ExpectationsWereMet())
}

func TestCompleteCheckOut_BeginFails(t *testing.T) {
	db, repo := newRepo(t)

	db.ExpectBegin().WillReturnError(fmt.Errorf("begin failed"))

	appErr := repo.CompleteCheckOut(context.Background(), testSessionID, testSpotID, testReservationID, time.Now())

	require.NotNil(t, appErr)
	assert.Equal(t, "db_error", appErr.ErrorCode)
	assert.NoError(t, db.ExpectationsWereMet())
}

func TestCompleteCheckOut_SpotUpdateFails(t *testing.T) {
	db, repo := newRepo(t)

	db.ExpectBegin()
	db.ExpectExec(`UPDATE sessions`).
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	db.ExpectExec(`UPDATE spots`).
		WithArgs(pgxmock.AnyArg()).
		WillReturnError(fmt.Errorf("spot update failed"))
	db.ExpectRollback()

	appErr := repo.CompleteCheckOut(context.Background(), testSessionID, testSpotID, testReservationID, time.Now())

	require.NotNil(t, appErr)
	assert.Equal(t, "db_error", appErr.ErrorCode)
	assert.NoError(t, db.ExpectationsWereMet())
}

func TestCompleteCheckOut_ReservationUpdateFails(t *testing.T) {
	db, repo := newRepo(t)

	db.ExpectBegin()
	db.ExpectExec(`UPDATE sessions`).
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	db.ExpectExec(`UPDATE spots`).
		WithArgs(pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	db.ExpectExec(`UPDATE reservations`).
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnError(fmt.Errorf("reservation update failed"))
	db.ExpectRollback()

	appErr := repo.CompleteCheckOut(context.Background(), testSessionID, testSpotID, testReservationID, time.Now())

	require.NotNil(t, appErr)
	assert.Equal(t, "db_error", appErr.ErrorCode)
	assert.NoError(t, db.ExpectationsWereMet())
}

func TestCompleteCheckOut_CommitFails(t *testing.T) {
	db, repo := newRepo(t)

	db.ExpectBegin()
	db.ExpectExec(`UPDATE sessions`).
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	db.ExpectExec(`UPDATE spots`).
		WithArgs(pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	db.ExpectExec(`UPDATE reservations`).
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	db.ExpectCommit().WillReturnError(fmt.Errorf("commit failed"))

	appErr := repo.CompleteCheckOut(context.Background(), testSessionID, testSpotID, testReservationID, time.Now())

	require.NotNil(t, appErr)
	assert.Equal(t, "db_error", appErr.ErrorCode)
	assert.NoError(t, db.ExpectationsWereMet())
}

func TestGetSessionByID_DBError(t *testing.T) {
	db, repo := newRepo(t)

	db.ExpectQuery(`SELECT id`).
		WithArgs(testSessionID).
		WillReturnError(fmt.Errorf("connection refused"))

	_, appErr := repo.GetSessionByID(context.Background(), testSessionID)

	require.NotNil(t, appErr)
	assert.Equal(t, "db_error", appErr.ErrorCode)
	assert.NoError(t, db.ExpectationsWereMet())
}
