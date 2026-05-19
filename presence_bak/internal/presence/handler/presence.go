package handler

import (
	"context"
	"log/slog"
	"time"

	pb "parkir-pintar/services/presence/gen/presence/v1"
	"parkir-pintar/services/presence/internal/presence/model"
	"parkir-pintar/services/presence/pkg/logger"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func (s *PresenceServer) CheckIn(ctx context.Context, req *pb.CheckInRequest) (*pb.CheckInResponse, error) {
	if !validateUUID(req.ReservationId) {
		return nil, status.Error(codes.InvalidArgument, "reservation_id must be a valid UUID")
	}
	if !validateUUID(req.DriverId) {
		return nil, status.Error(codes.InvalidArgument, "driver_id must be a valid UUID")
	}

	checkedInAt := time.Now()
	if req.CheckedInAt != nil {
		checkedInAt = req.CheckedInAt.AsTime()
	}

	res, appErr := s.uc.CheckIn(ctx, model.CheckInRequest{
		ReservationID: req.ReservationId,
		DriverID:      req.DriverId,
		CheckedInAt:   checkedInAt,
	})
	if appErr != nil {
		logger.Error(ctx, "CheckIn failed", slog.String("error", appErr.Error()))
		switch appErr.ErrorCode {
		case "not_found":
			return nil, status.Error(codes.NotFound, appErr.Message)
		case "conflict":
			return nil, status.Error(codes.FailedPrecondition, appErr.Message)
		default:
			return nil, status.Error(codes.Internal, appErr.Message)
		}
	}

	return &pb.CheckInResponse{
		SessionId:   res.SessionID,
		CheckedInAt: timestamppb.New(res.CheckedInAt),
		Status:      pb.SessionStatus_SESSION_STATUS_ACTIVE,
	}, nil
}

func (s *PresenceServer) CheckOut(ctx context.Context, req *pb.CheckOutRequest) (*pb.CheckOutResponse, error) {
	if !validateUUID(req.SessionId) {
		return nil, status.Error(codes.InvalidArgument, "session_id must be a valid UUID")
	}
	if !validateUUID(req.DriverId) {
		return nil, status.Error(codes.InvalidArgument, "driver_id must be a valid UUID")
	}

	checkedOutAt := time.Now()
	if req.CheckedOutAt != nil {
		checkedOutAt = req.CheckedOutAt.AsTime()
	}

	res, appErr := s.uc.CheckOut(ctx, model.CheckOutRequest{
		SessionID:    req.SessionId,
		DriverID:     req.DriverId,
		CheckedOutAt: checkedOutAt,
	})
	if appErr != nil {
		logger.Error(ctx, "CheckOut failed", slog.String("error", appErr.Error()))
		switch appErr.ErrorCode {
		case "not_found":
			return nil, status.Error(codes.NotFound, appErr.Message)
		case "conflict":
			return nil, status.Error(codes.FailedPrecondition, appErr.Message)
		default:
			return nil, status.Error(codes.Internal, appErr.Message)
		}
	}

	return &pb.CheckOutResponse{
		SessionId:    res.SessionID,
		InvoiceId:    res.InvoiceID,
		CheckedOutAt: timestamppb.New(res.CheckedOutAt),
		Status:       pb.SessionStatus_SESSION_STATUS_COMPLETED,
		TotalIdr:     res.TotalIDR,
		QrCodeUrl:    res.QRCodeURL,
	}, nil
}

func (s *PresenceServer) GetSession(ctx context.Context, req *pb.GetSessionRequest) (*pb.GetSessionResponse, error) {
	if !validateUUID(req.SessionId) {
		return nil, status.Error(codes.InvalidArgument, "session_id must be a valid UUID")
	}

	res, appErr := s.uc.GetSession(ctx, req.SessionId)
	if appErr != nil {
		logger.Error(ctx, "GetSession failed", slog.String("error", appErr.Error()))
		switch appErr.ErrorCode {
		case "not_found":
			return nil, status.Error(codes.NotFound, appErr.Message)
		default:
			return nil, status.Error(codes.Internal, appErr.Message)
		}
	}

	pbRes := &pb.GetSessionResponse{
		SessionId:     res.SessionID,
		ReservationId: res.ReservationID,
		DriverId:      res.DriverID,
		SpotId:        res.SpotID,
		Status:        pb.SessionStatus(res.Status),
		CheckedInAt:   timestamppb.New(res.CheckedInAt),
	}
	if res.CheckedOutAt != nil {
		pbRes.CheckedOutAt = timestamppb.New(*res.CheckedOutAt)
	}

	return pbRes, nil
}
