package repository

import (
	"context"
	"log/slog"

	"parkir-pintar/services/search/internal/search/model"
	"parkir-pintar/services/search/pkg/apperror"
	"parkir-pintar/services/search/pkg/logger"
)

func (r *SearchRepository) GetAvailability(ctx context.Context, vehicleType model.VehicleType) ([]model.FloorAvailability, *apperror.AppError) {
	query := `SELECT floor_number, COUNT(*) AS available_spots
	           FROM spots
	           WHERE status = $1 AND vehicle_type = $2
	           GROUP BY floor_number
	           ORDER BY floor_number`

	rows, err := r.db.Query(ctx, query, model.SpotStatusAvailable, vehicleType)
	if err != nil {
		logger.Error(ctx, "GetAvailability failed", slog.String("error", err.Error()))
		return nil, apperror.New("db_error", "failed to query availability")
	}
	defer rows.Close()

	var floors []model.FloorAvailability
	for rows.Next() {
		var f model.FloorAvailability
		if err := rows.Scan(&f.FloorNumber, &f.AvailableSpots); err != nil {
			logger.Error(ctx, "GetAvailability scan failed", slog.String("error", err.Error()))
			return nil, apperror.New("db_error", "failed to scan availability row")
		}
		f.VehicleType = vehicleType
		floors = append(floors, f)
	}
	if err := rows.Err(); err != nil {
		logger.Error(ctx, "GetAvailability rows error", slog.String("error", err.Error()))
		return nil, apperror.New("db_error", "failed to iterate availability rows")
	}

	return floors, nil
}

func (r *SearchRepository) ListSpots(ctx context.Context, floorNumber int, vehicleType model.VehicleType) ([]model.Spot, *apperror.AppError) {
	query := `SELECT id, floor_number, spot_code, vehicle_type, status
	           FROM spots
	           WHERE floor_number = $1 AND vehicle_type = $2
	           ORDER BY spot_code`

	rows, err := r.db.Query(ctx, query, floorNumber, vehicleType)
	if err != nil {
		logger.Error(ctx, "ListSpots failed", slog.String("error", err.Error()))
		return nil, apperror.New("db_error", "failed to query spots")
	}
	defer rows.Close()

	var spots []model.Spot
	for rows.Next() {
		var s model.Spot
		if err := rows.Scan(&s.ID, &s.FloorNumber, &s.SpotCode, &s.VehicleType, &s.Status); err != nil {
			logger.Error(ctx, "ListSpots scan failed", slog.String("error", err.Error()))
			return nil, apperror.New("db_error", "failed to scan spot row")
		}
		spots = append(spots, s)
	}
	if err := rows.Err(); err != nil {
		logger.Error(ctx, "ListSpots rows error", slog.String("error", err.Error()))
		return nil, apperror.New("db_error", "failed to iterate spot rows")
	}

	return spots, nil
}
