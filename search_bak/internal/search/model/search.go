package model

// VehicleType represents the type of vehicle
type VehicleType int

const (
	VehicleTypeCar        VehicleType = 1
	VehicleTypeMotorcycle VehicleType = 2
)

// SpotStatus represents the current state of a parking spot
type SpotStatus int

const (
	SpotStatusAvailable SpotStatus = 1
	SpotStatusLocked    SpotStatus = 2
)

// Spot represents a parking spot record
type Spot struct {
	ID          string      `db:"id"`
	FloorNumber int         `db:"floor_number"`
	SpotCode    string      `db:"spot_code"`
	VehicleType VehicleType `db:"vehicle_type"`
	Status      SpotStatus  `db:"status"`
}

// FloorAvailability holds availability summary for a single floor
type FloorAvailability struct {
	FloorNumber    int
	AvailableSpots int
	VehicleType    VehicleType
}

// GetAvailabilityRequest is the input for GetAvailability
type GetAvailabilityRequest struct {
	VehicleType VehicleType `validate:"required,oneof=1 2"`
}

// GetAvailabilityResponse is the output for GetAvailability
type GetAvailabilityResponse struct {
	TotalAvailable int
	Floors         []FloorAvailability
}

// ListSpotsRequest is the input for ListSpots
type ListSpotsRequest struct {
	FloorNumber int         `validate:"required,min=1,max=5"`
	VehicleType VehicleType `validate:"required,oneof=1 2"`
}

// ListSpotsResponse is the output for ListSpots
type ListSpotsResponse struct {
	Spots []Spot
}
