package repository

import (
	"context"

	"parkir-pintar/services/search/internal/search/model"
	"parkir-pintar/services/search/pkg/apperror"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type DB interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Begin(ctx context.Context) (pgx.Tx, error)
}

var _ DB = (*pgxpool.Pool)(nil)

type Search interface {
	// GetAvailability returns available spot counts grouped by floor for a vehicle type
	GetAvailability(ctx context.Context, vehicleType model.VehicleType) ([]model.FloorAvailability, *apperror.AppError)

	// ListSpots returns all spots on a given floor for a vehicle type
	ListSpots(ctx context.Context, floorNumber int, vehicleType model.VehicleType) ([]model.Spot, *apperror.AppError)
}

type SearchRepository struct {
	db DB
}

func NewSearch(db DB) Search {
	return &SearchRepository{db: db}
}
