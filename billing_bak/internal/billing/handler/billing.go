package handler

import (
	"context"
	"log/slog"

	pb "parkir-pintar/services/billing/gen/billing/v1"
	"parkir-pintar/services/billing/internal/billing/model"
	"parkir-pintar/services/billing/pkg/logger"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func (s *BillingServer) CalculateAndCreateInvoice(ctx context.Context, req *pb.CreateInvoiceRequest) (*pb.CreateInvoiceResponse, error) {
	if !validateUUID(req.IdempotencyKey) {
		return nil, status.Error(codes.InvalidArgument, "idempotency_key must be a valid UUID")
	}
	if !validateUUID(req.SessionId) {
		return nil, status.Error(codes.InvalidArgument, "session_id must be a valid UUID")
	}
	if !validateUUID(req.ReservationId) {
		return nil, status.Error(codes.InvalidArgument, "reservation_id must be a valid UUID")
	}
	if !validateUUID(req.DriverId) {
		return nil, status.Error(codes.InvalidArgument, "driver_id must be a valid UUID")
	}
	if req.CheckedInAt == nil {
		return nil, status.Error(codes.InvalidArgument, "checked_in_at is required")
	}
	if req.CheckedOutAt == nil {
		return nil, status.Error(codes.InvalidArgument, "checked_out_at is required")
	}

	res, appErr := s.uc.CalculateAndCreateInvoice(ctx, model.CreateInvoiceRequest{
		IdempotencyKey: req.IdempotencyKey,
		SessionID:      req.SessionId,
		ReservationID:  req.ReservationId,
		DriverID:       req.DriverId,
		CheckedInAt:    req.CheckedInAt.AsTime(),
		CheckedOutAt:   req.CheckedOutAt.AsTime(),
	})
	if appErr != nil {
		logger.Error(ctx, "CalculateAndCreateInvoice failed", slog.String("error", appErr.Error()))
		switch appErr.ErrorCode {
		case "db_error":
			return nil, status.Error(codes.Internal, appErr.Message)
		default:
			return nil, status.Error(codes.Internal, appErr.Message)
		}
	}

	return &pb.CreateInvoiceResponse{
		InvoiceId:       res.InvoiceID,
		BookingFeeIdr:   res.BookingFeeIDR,
		ParkingFeeIdr:   res.ParkingFeeIDR,
		OvernightFeeIdr: res.OvernightFeeIDR,
		TotalIdr:        res.TotalIDR,
		QrCodeUrl:       res.QRCodeURL,
	}, nil
}

func (s *BillingServer) GetInvoice(ctx context.Context, req *pb.GetInvoiceRequest) (*pb.GetInvoiceResponse, error) {
	if !validateUUID(req.InvoiceId) {
		return nil, status.Error(codes.InvalidArgument, "invoice_id must be a valid UUID")
	}

	res, appErr := s.uc.GetInvoice(ctx, req.InvoiceId)
	if appErr != nil {
		logger.Error(ctx, "GetInvoice failed", slog.String("error", appErr.Error()))
		switch appErr.ErrorCode {
		case "not_found":
			return nil, status.Error(codes.NotFound, appErr.Message)
		default:
			return nil, status.Error(codes.Internal, appErr.Message)
		}
	}

	return &pb.GetInvoiceResponse{
		InvoiceId:       res.InvoiceID,
		SessionId:       res.SessionID,
		ReservationId:   res.ReservationID,
		Status:          pb.InvoiceStatus(res.Status),
		BookingFeeIdr:   res.BookingFeeIDR,
		ParkingFeeIdr:   res.ParkingFeeIDR,
		OvernightFeeIdr: res.OvernightFeeIDR,
		TotalIdr:        res.TotalIDR,
		CreatedAt:       timestamppb.New(res.CreatedAt),
	}, nil
}

func (s *BillingServer) RetryPayment(ctx context.Context, req *pb.RetryPaymentRequest) (*pb.RetryPaymentResponse, error) {
	if !validateUUID(req.InvoiceId) {
		return nil, status.Error(codes.InvalidArgument, "invoice_id must be a valid UUID")
	}
	if !validateUUID(req.DriverId) {
		return nil, status.Error(codes.InvalidArgument, "driver_id must be a valid UUID")
	}

	res, appErr := s.uc.RetryPayment(ctx, model.RetryPaymentRequest{
		InvoiceID: req.InvoiceId,
		DriverID:  req.DriverId,
	})
	if appErr != nil {
		logger.Error(ctx, "RetryPayment failed", slog.String("error", appErr.Error()))
		switch appErr.ErrorCode {
		case "not_found":
			return nil, status.Error(codes.NotFound, appErr.Message)
		case "conflict":
			return nil, status.Error(codes.FailedPrecondition, appErr.Message)
		default:
			return nil, status.Error(codes.Internal, appErr.Message)
		}
	}

	return &pb.RetryPaymentResponse{
		PaymentId: res.PaymentID,
		QrCodeUrl: res.QRCodeURL,
	}, nil
}
