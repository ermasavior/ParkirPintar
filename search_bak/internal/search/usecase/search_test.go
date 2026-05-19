package usecase

import (
	"context"
	"testing"

	mocksearch "parkir-pintar/services/search/_mock/search"
	"parkir-pintar/services/search/internal/search/model"
	"parkir-pintar/services/search/pkg/apperror"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func newUsecase(repo *mocksearch.MockSearchRepository) *SearchUsecase {
	return &SearchUsecase{repo: repo}
}

// ── GetAvailability ───────────────────────────────────────────────────────────

func TestGetAvailability_Success_MultipleFloors(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mocksearch.NewMockSearchRepository(ctrl)
	repo.EXPECT().GetAvailability(gomock.Any(), model.VehicleTypeCar).
		Return([]model.FloorAvailability{
			{FloorNumber: 1, AvailableSpots: 5, VehicleType: model.VehicleTypeCar},
			{FloorNumber: 2, AvailableSpots: 3, VehicleType: model.VehicleTypeCar},
		}, nil)

	res, appErr := newUsecase(repo).GetAvailability(context.Background(), model.GetAvailabilityRequest{
		VehicleType: model.VehicleTypeCar,
	})

	require.Nil(t, appErr)
	assert.Equal(t, 8, res.TotalAvailable) // 5 + 3
	assert.Len(t, res.Floors, 2)
	assert.Equal(t, 1, res.Floors[0].FloorNumber)
	assert.Equal(t, 5, res.Floors[0].AvailableSpots)
	assert.Equal(t, 2, res.Floors[1].FloorNumber)
	assert.Equal(t, 3, res.Floors[1].AvailableSpots)
}

func TestGetAvailability_Success_NoSpots(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mocksearch.NewMockSearchRepository(ctrl)
	repo.EXPECT().GetAvailability(gomock.Any(), model.VehicleTypeCar).
		Return([]model.FloorAvailability{}, nil)

	res, appErr := newUsecase(repo).GetAvailability(context.Background(), model.GetAvailabilityRequest{
		VehicleType: model.VehicleTypeCar,
	})

	require.Nil(t, appErr)
	assert.Equal(t, 0, res.TotalAvailable)
	assert.Empty(t, res.Floors)
}

func TestGetAvailability_Success_TotalCalculation(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mocksearch.NewMockSearchRepository(ctrl)
	repo.EXPECT().GetAvailability(gomock.Any(), model.VehicleTypeMotorcycle).
		Return([]model.FloorAvailability{
			{FloorNumber: 1, AvailableSpots: 10, VehicleType: model.VehicleTypeMotorcycle},
			{FloorNumber: 2, AvailableSpots: 7, VehicleType: model.VehicleTypeMotorcycle},
			{FloorNumber: 3, AvailableSpots: 2, VehicleType: model.VehicleTypeMotorcycle},
		}, nil)

	res, appErr := newUsecase(repo).GetAvailability(context.Background(), model.GetAvailabilityRequest{
		VehicleType: model.VehicleTypeMotorcycle,
	})

	require.Nil(t, appErr)
	assert.Equal(t, 19, res.TotalAvailable) // 10 + 7 + 2
	assert.Len(t, res.Floors, 3)
}

func TestGetAvailability_DBError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mocksearch.NewMockSearchRepository(ctrl)
	repo.EXPECT().GetAvailability(gomock.Any(), model.VehicleTypeCar).
		Return(nil, apperror.New("db_error", "failed to query availability"))

	_, appErr := newUsecase(repo).GetAvailability(context.Background(), model.GetAvailabilityRequest{
		VehicleType: model.VehicleTypeCar,
	})

	require.NotNil(t, appErr)
	assert.Equal(t, "db_error", appErr.ErrorCode)
}

// ── ListSpots ─────────────────────────────────────────────────────────────────

func TestListSpots_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	spots := []model.Spot{
		{ID: "spot-1", FloorNumber: 1, SpotCode: "A1", VehicleType: model.VehicleTypeCar, Status: model.SpotStatusAvailable},
		{ID: "spot-2", FloorNumber: 1, SpotCode: "A2", VehicleType: model.VehicleTypeCar, Status: model.SpotStatusLocked},
		{ID: "spot-3", FloorNumber: 1, SpotCode: "A3", VehicleType: model.VehicleTypeCar, Status: model.SpotStatusLocked},
	}

	repo := mocksearch.NewMockSearchRepository(ctrl)
	repo.EXPECT().ListSpots(gomock.Any(), 1, model.VehicleTypeCar).Return(spots, nil)

	res, appErr := newUsecase(repo).ListSpots(context.Background(), model.ListSpotsRequest{
		FloorNumber: 1,
		VehicleType: model.VehicleTypeCar,
	})

	require.Nil(t, appErr)
	assert.Len(t, res.Spots, 3)
	assert.Equal(t, "A1", res.Spots[0].SpotCode)
	assert.Equal(t, model.SpotStatusAvailable, res.Spots[0].Status)
	assert.Equal(t, "A2", res.Spots[1].SpotCode)
	assert.Equal(t, model.SpotStatusLocked, res.Spots[1].Status)
}

func TestListSpots_EmptyFloor(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mocksearch.NewMockSearchRepository(ctrl)
	repo.EXPECT().ListSpots(gomock.Any(), 5, model.VehicleTypeCar).Return([]model.Spot{}, nil)

	res, appErr := newUsecase(repo).ListSpots(context.Background(), model.ListSpotsRequest{
		FloorNumber: 5,
		VehicleType: model.VehicleTypeCar,
	})

	require.Nil(t, appErr)
	assert.Empty(t, res.Spots)
}

func TestListSpots_Motorcycle(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mocksearch.NewMockSearchRepository(ctrl)
	repo.EXPECT().ListSpots(gomock.Any(), 2, model.VehicleTypeMotorcycle).
		Return([]model.Spot{
			{ID: "moto-1", FloorNumber: 2, SpotCode: "M1", VehicleType: model.VehicleTypeMotorcycle, Status: model.SpotStatusAvailable},
		}, nil)

	res, appErr := newUsecase(repo).ListSpots(context.Background(), model.ListSpotsRequest{
		FloorNumber: 2,
		VehicleType: model.VehicleTypeMotorcycle,
	})

	require.Nil(t, appErr)
	assert.Len(t, res.Spots, 1)
	assert.Equal(t, model.VehicleTypeMotorcycle, res.Spots[0].VehicleType)
}

func TestListSpots_DBError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mocksearch.NewMockSearchRepository(ctrl)
	repo.EXPECT().ListSpots(gomock.Any(), 1, model.VehicleTypeCar).
		Return(nil, apperror.New("db_error", "failed to query spots"))

	_, appErr := newUsecase(repo).ListSpots(context.Background(), model.ListSpotsRequest{
		FloorNumber: 1,
		VehicleType: model.VehicleTypeCar,
	})

	require.NotNil(t, appErr)
	assert.Equal(t, "db_error", appErr.ErrorCode)
}
