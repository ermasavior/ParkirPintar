package repository

import (
	"context"
	"fmt"
	"testing"

	"parkir-pintar/services/search/internal/search/model"

	pgxmock "github.com/pashagolub/pgxmock/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testSpotID1 = "770e8400-e29b-41d4-a716-446655440001"
	testSpotID2 = "770e8400-e29b-41d4-a716-446655440002"
)

func newRepo(t *testing.T) (pgxmock.PgxPoolIface, *SearchRepository) {
	t.Helper()
	db, err := pgxmock.NewPool()
	require.NoError(t, err)
	return db, &SearchRepository{db: db}
}

// ── GetAvailability ───────────────────────────────────────────────────────────

func TestGetAvailability_MultipleFloors(t *testing.T) {
	db, repo := newRepo(t)

	db.ExpectQuery(`SELECT floor_number`).
		WithArgs(model.SpotStatusAvailable, model.VehicleTypeCar).
		WillReturnRows(pgxmock.NewRows([]string{"floor_number", "available_spots"}).
			AddRow(1, 5).
			AddRow(2, 3).
			AddRow(3, 8))

	floors, appErr := repo.GetAvailability(context.Background(), model.VehicleTypeCar)

	require.Nil(t, appErr)
	require.Len(t, floors, 3)
	assert.Equal(t, 1, floors[0].FloorNumber)
	assert.Equal(t, 5, floors[0].AvailableSpots)
	assert.Equal(t, model.VehicleTypeCar, floors[0].VehicleType)
	assert.Equal(t, 2, floors[1].FloorNumber)
	assert.Equal(t, 3, floors[1].AvailableSpots)
	assert.Equal(t, 3, floors[2].FloorNumber)
	assert.Equal(t, 8, floors[2].AvailableSpots)
	assert.NoError(t, db.ExpectationsWereMet())
}

func TestGetAvailability_NoAvailableSpots(t *testing.T) {
	db, repo := newRepo(t)

	db.ExpectQuery(`SELECT floor_number`).
		WithArgs(model.SpotStatusAvailable, model.VehicleTypeCar).
		WillReturnRows(pgxmock.NewRows([]string{"floor_number", "available_spots"}))

	floors, appErr := repo.GetAvailability(context.Background(), model.VehicleTypeCar)

	require.Nil(t, appErr)
	assert.Empty(t, floors)
	assert.NoError(t, db.ExpectationsWereMet())
}

func TestGetAvailability_Motorcycle(t *testing.T) {
	db, repo := newRepo(t)

	db.ExpectQuery(`SELECT floor_number`).
		WithArgs(model.SpotStatusAvailable, model.VehicleTypeMotorcycle).
		WillReturnRows(pgxmock.NewRows([]string{"floor_number", "available_spots"}).
			AddRow(1, 10))

	floors, appErr := repo.GetAvailability(context.Background(), model.VehicleTypeMotorcycle)

	require.Nil(t, appErr)
	require.Len(t, floors, 1)
	assert.Equal(t, model.VehicleTypeMotorcycle, floors[0].VehicleType)
	assert.Equal(t, 10, floors[0].AvailableSpots)
	assert.NoError(t, db.ExpectationsWereMet())
}

func TestGetAvailability_DBError(t *testing.T) {
	db, repo := newRepo(t)

	db.ExpectQuery(`SELECT floor_number`).
		WithArgs(model.SpotStatusAvailable, model.VehicleTypeCar).
		WillReturnError(fmt.Errorf("connection refused"))

	_, appErr := repo.GetAvailability(context.Background(), model.VehicleTypeCar)

	require.NotNil(t, appErr)
	assert.Equal(t, "db_error", appErr.ErrorCode)
	assert.NoError(t, db.ExpectationsWereMet())
}

// ── ListSpots ─────────────────────────────────────────────────────────────────
// Scan order: id, floor_number, spot_code, vehicle_type, status

func TestListSpots_MultipleSpots(t *testing.T) {
	db, repo := newRepo(t)

	db.ExpectQuery(`SELECT id`).
		WithArgs(1, model.VehicleTypeCar).
		WillReturnRows(pgxmock.NewRows([]string{"id", "floor_number", "spot_code", "vehicle_type", "status"}).
			AddRow(testSpotID1, 1, "A1", model.VehicleTypeCar, model.SpotStatusAvailable).
			AddRow(testSpotID2, 1, "A2", model.VehicleTypeCar, model.SpotStatusLocked))

	spots, appErr := repo.ListSpots(context.Background(), 1, model.VehicleTypeCar)

	require.Nil(t, appErr)
	require.Len(t, spots, 2)
	assert.Equal(t, testSpotID1, spots[0].ID)
	assert.Equal(t, "A1", spots[0].SpotCode)
	assert.Equal(t, model.SpotStatusAvailable, spots[0].Status)
	assert.Equal(t, testSpotID2, spots[1].ID)
	assert.Equal(t, "A2", spots[1].SpotCode)
	assert.Equal(t, model.SpotStatusLocked, spots[1].Status)
	assert.NoError(t, db.ExpectationsWereMet())
}

func TestListSpots_EmptyFloor(t *testing.T) {
	db, repo := newRepo(t)

	db.ExpectQuery(`SELECT id`).
		WithArgs(5, model.VehicleTypeCar).
		WillReturnRows(pgxmock.NewRows([]string{"id", "floor_number", "spot_code", "vehicle_type", "status"}))

	spots, appErr := repo.ListSpots(context.Background(), 5, model.VehicleTypeCar)

	require.Nil(t, appErr)
	assert.Empty(t, spots)
	assert.NoError(t, db.ExpectationsWereMet())
}

func TestListSpots_Motorcycle(t *testing.T) {
	db, repo := newRepo(t)

	db.ExpectQuery(`SELECT id`).
		WithArgs(2, model.VehicleTypeMotorcycle).
		WillReturnRows(pgxmock.NewRows([]string{"id", "floor_number", "spot_code", "vehicle_type", "status"}).
			AddRow(testSpotID1, 2, "M1", model.VehicleTypeMotorcycle, model.SpotStatusAvailable))

	spots, appErr := repo.ListSpots(context.Background(), 2, model.VehicleTypeMotorcycle)

	require.Nil(t, appErr)
	require.Len(t, spots, 1)
	assert.Equal(t, model.VehicleTypeMotorcycle, spots[0].VehicleType)
	assert.Equal(t, "M1", spots[0].SpotCode)
	assert.NoError(t, db.ExpectationsWereMet())
}

func TestListSpots_DBError(t *testing.T) {
	db, repo := newRepo(t)

	db.ExpectQuery(`SELECT id`).
		WithArgs(1, model.VehicleTypeCar).
		WillReturnError(fmt.Errorf("connection refused"))

	_, appErr := repo.ListSpots(context.Background(), 1, model.VehicleTypeCar)

	require.NotNil(t, appErr)
	assert.Equal(t, "db_error", appErr.ErrorCode)
	assert.NoError(t, db.ExpectationsWereMet())
}

// ── rows.Err() and scan error paths ──────────────────────────────────────────

func TestGetAvailability_RowsError(t *testing.T) {
	db, repo := newRepo(t)

	db.ExpectQuery(`SELECT floor_number`).
		WithArgs(model.SpotStatusAvailable, model.VehicleTypeCar).
		WillReturnRows(pgxmock.NewRows([]string{"floor_number", "available_spots"}).
			AddRow(1, 5).
			RowError(0, fmt.Errorf("rows iteration error")))

	_, appErr := repo.GetAvailability(context.Background(), model.VehicleTypeCar)

	require.NotNil(t, appErr)
	assert.Equal(t, "db_error", appErr.ErrorCode)
	assert.NoError(t, db.ExpectationsWereMet())
}

func TestGetAvailability_ScanError(t *testing.T) {
	db, repo := newRepo(t)

	db.ExpectQuery(`SELECT floor_number`).
		WithArgs(model.SpotStatusAvailable, model.VehicleTypeCar).
		WillReturnRows(pgxmock.NewRows([]string{"floor_number", "available_spots"}).
			AddRow("not-an-int", "also-not-an-int"))

	_, appErr := repo.GetAvailability(context.Background(), model.VehicleTypeCar)

	require.NotNil(t, appErr)
	assert.Equal(t, "db_error", appErr.ErrorCode)
	assert.NoError(t, db.ExpectationsWereMet())
}

func TestListSpots_RowsError(t *testing.T) {
	db, repo := newRepo(t)

	db.ExpectQuery(`SELECT id`).
		WithArgs(1, model.VehicleTypeCar).
		WillReturnRows(pgxmock.NewRows([]string{"id", "floor_number", "spot_code", "vehicle_type", "status"}).
			AddRow(testSpotID1, 1, "A1", model.VehicleTypeCar, model.SpotStatusAvailable).
			RowError(0, fmt.Errorf("rows iteration error")))

	_, appErr := repo.ListSpots(context.Background(), 1, model.VehicleTypeCar)

	require.NotNil(t, appErr)
	assert.Equal(t, "db_error", appErr.ErrorCode)
	assert.NoError(t, db.ExpectationsWereMet())
}

func TestListSpots_ScanError(t *testing.T) {
	db, repo := newRepo(t)

	db.ExpectQuery(`SELECT id`).
		WithArgs(1, model.VehicleTypeCar).
		WillReturnRows(pgxmock.NewRows([]string{"id", "floor_number", "spot_code", "vehicle_type", "status"}).
			AddRow(12345, "not-an-int", nil, nil, nil))

	_, appErr := repo.ListSpots(context.Background(), 1, model.VehicleTypeCar)

	require.NotNil(t, appErr)
	assert.Equal(t, "db_error", appErr.ErrorCode)
	assert.NoError(t, db.ExpectationsWereMet())
}
