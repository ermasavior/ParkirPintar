package handler

import (
	"context"
	"testing"

	mocksearch "parkir-pintar/services/search/_mock/search"
	pb "parkir-pintar/services/search/gen/search/v1"
	"parkir-pintar/services/search/internal/search/model"
	"parkir-pintar/services/search/pkg/apperror"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func newServer(uc *mocksearch.MockSearchUsecase) *SearchServer {
	return &SearchServer{uc: uc}
}

func grpcCode(err error) codes.Code {
	if s, ok := status.FromError(err); ok {
		return s.Code()
	}
	return codes.Unknown
}

// ── GetAvailability — validation ──────────────────────────────────────────────

func TestGetAvailability_UnspecifiedVehicleType(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	srv := newServer(mocksearch.NewMockSearchUsecase(ctrl))

	_, err := srv.GetAvailability(context.Background(), &pb.GetAvailabilityRequest{
		VehicleType: pb.VehicleType_VEHICLE_TYPE_UNSPECIFIED,
	})

	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, grpcCode(err))
	assert.Contains(t, status.Convert(err).Message(), "vehicle_type")
}

// ── GetAvailability — usecase error mapping ───────────────────────────────────

func TestGetAvailability_InternalError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc := mocksearch.NewMockSearchUsecase(ctrl)
	uc.EXPECT().GetAvailability(gomock.Any(), gomock.Any()).
		Return(nil, apperror.New("db_error", "failed to query availability"))

	_, err := newServer(uc).GetAvailability(context.Background(), &pb.GetAvailabilityRequest{
		VehicleType: pb.VehicleType_VEHICLE_TYPE_CAR,
	})

	require.Error(t, err)
	assert.Equal(t, codes.Internal, grpcCode(err))
}

// ── GetAvailability — success ─────────────────────────────────────────────────

func TestGetAvailability_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc := mocksearch.NewMockSearchUsecase(ctrl)
	uc.EXPECT().GetAvailability(gomock.Any(), gomock.Any()).
		Return(&model.GetAvailabilityResponse{
			TotalAvailable: 8,
			Floors: []model.FloorAvailability{
				{FloorNumber: 1, AvailableSpots: 5, VehicleType: model.VehicleTypeCar},
				{FloorNumber: 2, AvailableSpots: 3, VehicleType: model.VehicleTypeCar},
			},
		}, nil)

	res, err := newServer(uc).GetAvailability(context.Background(), &pb.GetAvailabilityRequest{
		VehicleType: pb.VehicleType_VEHICLE_TYPE_CAR,
	})

	require.NoError(t, err)
	assert.Equal(t, int32(8), res.TotalAvailable)
	assert.Len(t, res.Floors, 2)
	assert.Equal(t, int32(1), res.Floors[0].FloorNumber)
	assert.Equal(t, int32(5), res.Floors[0].AvailableSpots)
	assert.Equal(t, pb.VehicleType_VEHICLE_TYPE_CAR, res.Floors[0].VehicleType)
	assert.Equal(t, int32(2), res.Floors[1].FloorNumber)
	assert.Equal(t, int32(3), res.Floors[1].AvailableSpots)
}

func TestGetAvailability_Success_NoSpots(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc := mocksearch.NewMockSearchUsecase(ctrl)
	uc.EXPECT().GetAvailability(gomock.Any(), gomock.Any()).
		Return(&model.GetAvailabilityResponse{TotalAvailable: 0, Floors: nil}, nil)

	res, err := newServer(uc).GetAvailability(context.Background(), &pb.GetAvailabilityRequest{
		VehicleType: pb.VehicleType_VEHICLE_TYPE_MOTORCYCLE,
	})

	require.NoError(t, err)
	assert.Equal(t, int32(0), res.TotalAvailable)
	assert.Empty(t, res.Floors)
}

// ── ListSpots — validation ────────────────────────────────────────────────────

func TestListSpots_FloorNumberZero(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	srv := newServer(mocksearch.NewMockSearchUsecase(ctrl))

	_, err := srv.ListSpots(context.Background(), &pb.ListSpotsRequest{
		FloorNumber: 0,
		VehicleType: pb.VehicleType_VEHICLE_TYPE_CAR,
	})

	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, grpcCode(err))
	assert.Contains(t, status.Convert(err).Message(), "floor_number")
}

func TestListSpots_FloorNumberAboveMax(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	srv := newServer(mocksearch.NewMockSearchUsecase(ctrl))

	_, err := srv.ListSpots(context.Background(), &pb.ListSpotsRequest{
		FloorNumber: 6,
		VehicleType: pb.VehicleType_VEHICLE_TYPE_CAR,
	})

	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, grpcCode(err))
}

func TestListSpots_UnspecifiedVehicleType(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	srv := newServer(mocksearch.NewMockSearchUsecase(ctrl))

	_, err := srv.ListSpots(context.Background(), &pb.ListSpotsRequest{
		FloorNumber: 1,
		VehicleType: pb.VehicleType_VEHICLE_TYPE_UNSPECIFIED,
	})

	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, grpcCode(err))
	assert.Contains(t, status.Convert(err).Message(), "vehicle_type")
}

// ── ListSpots — usecase error mapping ────────────────────────────────────────

func TestListSpots_InternalError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc := mocksearch.NewMockSearchUsecase(ctrl)
	uc.EXPECT().ListSpots(gomock.Any(), gomock.Any()).
		Return(nil, apperror.New("db_error", "failed to query spots"))

	_, err := newServer(uc).ListSpots(context.Background(), &pb.ListSpotsRequest{
		FloorNumber: 1,
		VehicleType: pb.VehicleType_VEHICLE_TYPE_CAR,
	})

	require.Error(t, err)
	assert.Equal(t, codes.Internal, grpcCode(err))
}

// ── ListSpots — success ───────────────────────────────────────────────────────

func TestListSpots_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc := mocksearch.NewMockSearchUsecase(ctrl)
	uc.EXPECT().ListSpots(gomock.Any(), gomock.Any()).
		Return(&model.ListSpotsResponse{
			Spots: []model.Spot{
				{ID: "spot-1", FloorNumber: 1, SpotCode: "A1", VehicleType: model.VehicleTypeCar, Status: model.SpotStatusAvailable},
				{ID: "spot-2", FloorNumber: 1, SpotCode: "A2", VehicleType: model.VehicleTypeCar, Status: model.SpotStatusLocked},
			},
		}, nil)

	res, err := newServer(uc).ListSpots(context.Background(), &pb.ListSpotsRequest{
		FloorNumber: 1,
		VehicleType: pb.VehicleType_VEHICLE_TYPE_CAR,
	})

	require.NoError(t, err)
	assert.Len(t, res.Spots, 2)
	assert.Equal(t, "spot-1", res.Spots[0].SpotId)
	assert.Equal(t, "A1", res.Spots[0].SpotCode)
	assert.Equal(t, int32(1), res.Spots[0].FloorNumber)
	assert.Equal(t, pb.VehicleType_VEHICLE_TYPE_CAR, res.Spots[0].VehicleType)
	assert.Equal(t, pb.SpotStatus_SPOT_STATUS_AVAILABLE, res.Spots[0].Status)
	assert.Equal(t, "spot-2", res.Spots[1].SpotId)
	assert.Equal(t, pb.SpotStatus_SPOT_STATUS_LOCKED, res.Spots[1].Status)
}

func TestListSpots_Success_EmptyFloor(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc := mocksearch.NewMockSearchUsecase(ctrl)
	uc.EXPECT().ListSpots(gomock.Any(), gomock.Any()).
		Return(&model.ListSpotsResponse{Spots: []model.Spot{}}, nil)

	res, err := newServer(uc).ListSpots(context.Background(), &pb.ListSpotsRequest{
		FloorNumber: 5,
		VehicleType: pb.VehicleType_VEHICLE_TYPE_CAR,
	})

	require.NoError(t, err)
	assert.Empty(t, res.Spots)
}

func TestListSpots_Success_Motorcycle(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uc := mocksearch.NewMockSearchUsecase(ctrl)
	uc.EXPECT().ListSpots(gomock.Any(), gomock.Any()).
		Return(&model.ListSpotsResponse{
			Spots: []model.Spot{
				{ID: "moto-1", FloorNumber: 2, SpotCode: "M1", VehicleType: model.VehicleTypeMotorcycle, Status: model.SpotStatusAvailable},
			},
		}, nil)

	res, err := newServer(uc).ListSpots(context.Background(), &pb.ListSpotsRequest{
		FloorNumber: 2,
		VehicleType: pb.VehicleType_VEHICLE_TYPE_MOTORCYCLE,
	})

	require.NoError(t, err)
	assert.Len(t, res.Spots, 1)
	assert.Equal(t, pb.VehicleType_VEHICLE_TYPE_MOTORCYCLE, res.Spots[0].VehicleType)
	assert.Equal(t, "M1", res.Spots[0].SpotCode)
}
