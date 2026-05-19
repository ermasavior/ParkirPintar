package repository

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"parkir-pintar/services/presence/internal/presence/model"
	"parkir-pintar/services/presence/pkg/apperror"
	"parkir-pintar/services/presence/pkg/logger"

	"github.com/jackc/pgx/v5"
)

// GetReservationForCheckIn fetches a reservation and validates it belongs to the driver
func (r *PresenceRepository) GetReservationForCheckIn(ctx context.Context, reservationID, driverID string) (*model.Reservation, *apperror.AppError) {
	query := `SELECT id, driver_id, spot_id, status, expires_at
	           FROM reservations
	           WHERE id = $1 AND driver_id = $2`

	var res model.Reservation
	err := r.db.QueryRow(ctx, query, reservationID, driverID).Scan(
		&res.ID,
		&res.DriverID,
		&res.SpotID,
		&res.Status,
		&res.ExpiresAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperror.New("not_found", "reservation not found or does not belong to driver")
		}
		logger.Error(ctx, "GetReservationForCheckIn failed", slog.String("error", err.Error()))
		return nil, apperror.New("db_error", "failed to query reservation")
	}
	return &res, nil
}

// CreateSessionAndMarkCheckedIn atomically inserts a session and updates reservation status to CHECKED_IN
func (r *PresenceRepository) CreateSessionAndMarkCheckedIn(ctx context.Context, reservationID, driverID, spotID string, checkedInAt time.Time) (*model.Session, *apperror.AppError) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		logger.Error(ctx, "CreateSessionAndMarkCheckedIn: begin tx failed", slog.String("error", err.Error()))
		return nil, apperror.New("db_error", "failed to begin transaction")
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback(ctx)
		}
	}()

	var session model.Session
	err = tx.QueryRow(ctx,
		`INSERT INTO sessions (reservation_id, driver_id, spot_id, status, checked_in_at)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id, reservation_id, driver_id, spot_id, status, checked_in_at`,
		reservationID, driverID, spotID, model.SessionStatusActive, checkedInAt,
	).Scan(
		&session.ID,
		&session.ReservationID,
		&session.DriverID,
		&session.SpotID,
		&session.Status,
		&session.CheckedInAt,
	)
	if err != nil {
		logger.Error(ctx, "CreateSessionAndMarkCheckedIn: insert session failed", slog.String("error", err.Error()))
		return nil, apperror.New("db_error", "failed to create session")
	}

	_, err = tx.Exec(ctx,
		`UPDATE reservations SET status = $1 WHERE id = $2`,
		model.ReservationStatusCheckedIn, reservationID,
	)
	if err != nil {
		logger.Error(ctx, "CreateSessionAndMarkCheckedIn: update reservation failed", slog.String("error", err.Error()))
		return nil, apperror.New("db_error", "failed to update reservation status")
	}

	if err = tx.Commit(ctx); err != nil {
		logger.Error(ctx, "CreateSessionAndMarkCheckedIn: commit failed", slog.String("error", err.Error()))
		return nil, apperror.New("db_error", "failed to commit transaction")
	}

	return &session, nil
}

// GetSessionForCheckOut fetches an active session and validates it belongs to the driver
func (r *PresenceRepository) GetSessionForCheckOut(ctx context.Context, sessionID, driverID string) (*model.Session, *apperror.AppError) {
	query := `SELECT id, reservation_id, driver_id, spot_id, status, checked_in_at, checked_out_at
	           FROM sessions
	           WHERE id = $1 AND driver_id = $2`

	var s model.Session
	err := r.db.QueryRow(ctx, query, sessionID, driverID).Scan(
		&s.ID,
		&s.ReservationID,
		&s.DriverID,
		&s.SpotID,
		&s.Status,
		&s.CheckedInAt,
		&s.CheckedOutAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperror.New("not_found", "session not found or does not belong to driver")
		}
		logger.Error(ctx, "GetSessionForCheckOut failed", slog.String("error", err.Error()))
		return nil, apperror.New("db_error", "failed to query session")
	}
	return &s, nil
}

// CompleteCheckOut atomically marks session COMPLETED, spot AVAILABLE, reservation COMPLETED
func (r *PresenceRepository) CompleteCheckOut(ctx context.Context, sessionID, spotID, reservationID string, checkedOutAt time.Time) *apperror.AppError {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		logger.Error(ctx, "CompleteCheckOut: begin tx failed", slog.String("error", err.Error()))
		return apperror.New("db_error", "failed to begin transaction")
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback(ctx)
		}
	}()

	_, err = tx.Exec(ctx,
		`UPDATE sessions SET status = $1, checked_out_at = $2 WHERE id = $3`,
		model.SessionStatusCompleted, checkedOutAt, sessionID,
	)
	if err != nil {
		logger.Error(ctx, "CompleteCheckOut: update session failed", slog.String("error", err.Error()))
		return apperror.New("db_error", "failed to update session")
	}

	_, err = tx.Exec(ctx,
		`UPDATE spots SET status = 1 WHERE id = $1`, // 1 = AVAILABLE
		spotID,
	)
	if err != nil {
		logger.Error(ctx, "CompleteCheckOut: update spot failed", slog.String("error", err.Error()))
		return apperror.New("db_error", "failed to release spot")
	}

	_, err = tx.Exec(ctx,
		`UPDATE reservations SET status = $1 WHERE id = $2`,
		model.ReservationStatusCompleted, reservationID,
	)
	if err != nil {
		logger.Error(ctx, "CompleteCheckOut: update reservation failed", slog.String("error", err.Error()))
		return apperror.New("db_error", "failed to update reservation status")
	}

	if err = tx.Commit(ctx); err != nil {
		logger.Error(ctx, "CompleteCheckOut: commit failed", slog.String("error", err.Error()))
		return apperror.New("db_error", "failed to commit transaction")
	}

	return nil
}

// GetSessionByID returns a session by its UUID
func (r *PresenceRepository) GetSessionByID(ctx context.Context, sessionID string) (*model.Session, *apperror.AppError) {
	query := `SELECT id, reservation_id, driver_id, spot_id, status, checked_in_at, checked_out_at
	           FROM sessions
	           WHERE id = $1`

	var s model.Session
	err := r.db.QueryRow(ctx, query, sessionID).Scan(
		&s.ID,
		&s.ReservationID,
		&s.DriverID,
		&s.SpotID,
		&s.Status,
		&s.CheckedInAt,
		&s.CheckedOutAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperror.New("not_found", "session not found")
		}
		logger.Error(ctx, "GetSessionByID failed", slog.String("error", err.Error()))
		return nil, apperror.New("db_error", "failed to query session")
	}
	return &s, nil
}
