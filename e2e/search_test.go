//go:build e2e

package e2e_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	searchpb "parkir-pintar/services/search/gen/search/v1"
)

// ── Search tests ──────────────────────────────────────────────────

// TestE2E_GetAvailability_Car verifies that the search service returns
// availability data for CAR vehicle type with a positive total count.
func TestE2E_GetAvailability_Car(t *testing.T) {
	s := NewSuite(t)

	res, err := s.Search.GetAvailability(context.Background(), &searchpb.GetAvailabilityRequest{
		VehicleType: searchpb.VehicleType_VEHICLE_TYPE_CAR,
	})

	require.NoError(t, err)
	assert.Greater(t, res.TotalAvailable, int32(0), "should have available car spots in a fresh environment")
	assert.NotEmpty(t, res.Floors, "should return per-floor breakdown")
	for _, floor := range res.Floors {
		assert.Greater(t, floor.FloorNumber, int32(0))
		assert.GreaterOrEqual(t, floor.AvailableSpots, int32(0))
		assert.Equal(t, searchpb.VehicleType_VEHICLE_TYPE_CAR, floor.VehicleType)
	}
}

// TestE2E_GetAvailability_Motorcycle verifies that the search service returns
// availability data for MOTORCYCLE vehicle type.
func TestE2E_GetAvailability_Motorcycle(t *testing.T) {
	s := NewSuite(t)

	res, err := s.Search.GetAvailability(context.Background(), &searchpb.GetAvailabilityRequest{
		VehicleType: searchpb.VehicleType_VEHICLE_TYPE_MOTORCYCLE,
	})

	require.NoError(t, err)
	assert.Greater(t, res.TotalAvailable, int32(0), "should have available motorcycle spots in a fresh environment")
	assert.NotEmpty(t, res.Floors)
	for _, floor := range res.Floors {
		assert.Equal(t, searchpb.VehicleType_VEHICLE_TYPE_MOTORCYCLE, floor.VehicleType)
	}
}

// TestE2E_ListSpots_Car verifies that listing spots for a valid floor and CAR
// vehicle type returns a non-empty list with correct fields.
func TestE2E_ListSpots_Car(t *testing.T) {
	s := NewSuite(t)

	// Discover which floors have car spots.
	avail, err := s.Search.GetAvailability(context.Background(), &searchpb.GetAvailabilityRequest{
		VehicleType: searchpb.VehicleType_VEHICLE_TYPE_CAR,
	})
	require.NoError(t, err)
	require.NotEmpty(t, avail.Floors, "need at least one floor to list spots")

	floor := avail.Floors[0].FloorNumber

	res, err := s.Search.ListSpots(context.Background(), &searchpb.ListSpotsRequest{
		FloorNumber: floor,
		VehicleType: searchpb.VehicleType_VEHICLE_TYPE_CAR,
	})

	require.NoError(t, err)
	assert.NotEmpty(t, res.Spots, "floor %d should have car spots", floor)
	for _, spot := range res.Spots {
		assert.NotEmpty(t, spot.SpotId)
		assert.NotEmpty(t, spot.SpotCode)
		assert.Equal(t, floor, spot.FloorNumber)
		assert.Equal(t, searchpb.VehicleType_VEHICLE_TYPE_CAR, spot.VehicleType)
		assert.NotEqual(t, searchpb.SpotStatus_SPOT_STATUS_UNSPECIFIED, spot.Status)
	}
}

// TestE2E_ListSpots_Motorcycle verifies listing motorcycle spots on a floor.
func TestE2E_ListSpots_Motorcycle(t *testing.T) {
	s := NewSuite(t)

	avail, err := s.Search.GetAvailability(context.Background(), &searchpb.GetAvailabilityRequest{
		VehicleType: searchpb.VehicleType_VEHICLE_TYPE_MOTORCYCLE,
	})
	require.NoError(t, err)
	require.NotEmpty(t, avail.Floors)

	floor := avail.Floors[0].FloorNumber

	res, err := s.Search.ListSpots(context.Background(), &searchpb.ListSpotsRequest{
		FloorNumber: floor,
		VehicleType: searchpb.VehicleType_VEHICLE_TYPE_MOTORCYCLE,
	})

	require.NoError(t, err)
	assert.NotEmpty(t, res.Spots)
	for _, spot := range res.Spots {
		assert.Equal(t, searchpb.VehicleType_VEHICLE_TYPE_MOTORCYCLE, spot.VehicleType)
	}
}

// TestE2E_GetAvailability_DecreasesAfterReservation verifies that creating a
// reservation reduces the available spot count returned by the search service.
func TestE2E_GetAvailability_DecreasesAfterReservation(t *testing.T) {
	s := NewSuite(t)

	before, err := s.Search.GetAvailability(context.Background(), &searchpb.GetAvailabilityRequest{
		VehicleType: searchpb.VehicleType_VEHICLE_TYPE_CAR,
	})
	require.NoError(t, err)
	totalBefore := before.TotalAvailable

	// Create a reservation to lock one spot.
	_, err = s.Reservation.CreateReservation(context.Background(), mustNewReservationReq())
	require.NoError(t, err)

	after, err := s.Search.GetAvailability(context.Background(), &searchpb.GetAvailabilityRequest{
		VehicleType: searchpb.VehicleType_VEHICLE_TYPE_CAR,
	})
	require.NoError(t, err)

	assert.Less(t, after.TotalAvailable, totalBefore,
		"available count should decrease after a reservation locks a spot")
}
