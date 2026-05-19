package handler

import (
	"context"
	"log/slog"

	pb "parkir-pintar/services/search/gen/search/v1"
	"parkir-pintar/services/search/internal/search/model"
	"parkir-pintar/services/search/pkg/logger"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (s *SearchServer) GetAvailability(ctx context.Context, req *pb.GetAvailabilityRequest) (*pb.GetAvailabilityResponse, error) {
	if req.VehicleType == pb.VehicleType_VEHICLE_TYPE_UNSPECIFIED {
		return nil, status.Error(codes.InvalidArgument, "vehicle_type is required")
	}

	res, appErr := s.uc.GetAvailability(ctx, model.GetAvailabilityRequest{
		VehicleType: model.VehicleType(req.VehicleType),
	})
	if appErr != nil {
		logger.Error(ctx, "GetAvailability failed", slog.String("error", appErr.Error()))
		return nil, status.Error(codes.Internal, appErr.Message)
	}

	pbFloors := make([]*pb.FloorAvailability, 0, len(res.Floors))
	for _, f := range res.Floors {
		pbFloors = append(pbFloors, &pb.FloorAvailability{
			FloorNumber:    int32(f.FloorNumber),
			AvailableSpots: int32(f.AvailableSpots),
			VehicleType:    pb.VehicleType(f.VehicleType),
		})
	}

	return &pb.GetAvailabilityResponse{
		TotalAvailable: int32(res.TotalAvailable),
		Floors:         pbFloors,
	}, nil
}

func (s *SearchServer) ListSpots(ctx context.Context, req *pb.ListSpotsRequest) (*pb.ListSpotsResponse, error) {
	if req.FloorNumber < 1 || req.FloorNumber > 5 {
		return nil, status.Error(codes.InvalidArgument, "floor_number must be between 1 and 5")
	}
	if req.VehicleType == pb.VehicleType_VEHICLE_TYPE_UNSPECIFIED {
		return nil, status.Error(codes.InvalidArgument, "vehicle_type is required")
	}

	res, appErr := s.uc.ListSpots(ctx, model.ListSpotsRequest{
		FloorNumber: int(req.FloorNumber),
		VehicleType: model.VehicleType(req.VehicleType),
	})
	if appErr != nil {
		logger.Error(ctx, "ListSpots failed", slog.String("error", appErr.Error()))
		return nil, status.Error(codes.Internal, appErr.Message)
	}

	pbSpots := make([]*pb.Spot, 0, len(res.Spots))
	for _, s := range res.Spots {
		pbSpots = append(pbSpots, &pb.Spot{
			SpotId:      s.ID,
			FloorNumber: int32(s.FloorNumber),
			SpotCode:    s.SpotCode,
			VehicleType: pb.VehicleType(s.VehicleType),
			Status:      pb.SpotStatus(s.Status),
		})
	}

	return &pb.ListSpotsResponse{Spots: pbSpots}, nil
}
