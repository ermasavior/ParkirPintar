package repository

import (
	"context"
	"time"

	"parkir-pintar/services/presence/internal/presence/model"
	"parkir-pintar/services/presence/pkg/apperror"

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

type Presence interface {
	// GetReservationForCheckIn fetches a reservation and validates it belongs to the driver
	GetReservationForCheckIn(ctx context.Context, reservationID, driverID string) (*model.Reservation, *apperror.AppError)

	// CreateSessionAndMarkCheckedIn atomically inserts a session and updates reservation status to CHECKED_IN
	CreateSessionAndMarkCheckedIn(ctx context.Context, reservationID, driverID, spotID string, checkedInAt time.Time) (*model.Session, *apperror.AppError)

	// GetSessionForCheckOut fetches an active session and validates it belongs to the driver
	GetSessionForCheckOut(ctx context.Context, sessionID, driverID string) (*model.Session, *apperror.AppError)

	// CompleteCheckOut atomically marks session COMPLETED, spot AVAILABLE, reservation COMPLETED
	CompleteCheckOut(ctx context.Context, sessionID, spotID, reservationID string, checkedOutAt time.Time) *apperror.AppError

	// GetSessionByID returns a session by its UUID
	GetSessionByID(ctx context.Context, sessionID string) (*model.Session, *apperror.AppError)
}

type PresenceRepository struct {
	db DB
}

func NewPresence(db DB) Presence {
	return &PresenceRepository{db: db}
}
