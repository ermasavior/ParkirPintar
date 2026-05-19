package handler

import (
	"context"
	"log/slog"

	pb "parkir-pintar/services/payment/gen/payment/v1"
	"parkir-pintar/services/payment/internal/payment/model"
	"parkir-pintar/services/payment/pkg/logger"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (s *PaymentServer) CreatePayment(ctx context.Context, req *pb.CreatePaymentRequest) (*pb.CreatePaymentResponse, error) {
	if !validateUUID(req.IdempotencyKey) {
		return nil, status.Error(codes.InvalidArgument, "idempotency_key must be a valid UUID")
	}
	if !validateUUID(req.ReferenceId) {
		return nil, status.Error(codes.InvalidArgument, "reference_id must be a valid UUID")
	}
	if !validateUUID(req.DriverId) {
		return nil, status.Error(codes.InvalidArgument, "driver_id must be a valid UUID")
	}
	if req.AmountIdr <= 0 {
		return nil, status.Error(codes.InvalidArgument, "amount_idr must be greater than 0")
	}
	if req.PaymentType == pb.PaymentType_PAYMENT_TYPE_UNSPECIFIED {
		return nil, status.Error(codes.InvalidArgument, "payment_type is required")
	}
	if req.Method == pb.PaymentMethod_PAYMENT_METHOD_UNSPECIFIED {
		return nil, status.Error(codes.InvalidArgument, "method is required")
	}

	res, appErr := s.uc.CreatePayment(ctx, model.CreatePaymentRequest{
		IdempotencyKey: req.IdempotencyKey,
		ReferenceID:    req.ReferenceId,
		PaymentType:    model.PaymentType(req.PaymentType),
		AmountIDR:      req.AmountIdr,
		DriverID:       req.DriverId,
		Method:         model.PaymentMethod(req.Method),
	})
	if appErr != nil {
		logger.Error(ctx, "CreatePayment failed", slog.String("error", appErr.Error()))
		return nil, status.Error(codes.Internal, appErr.Message)
	}

	return &pb.CreatePaymentResponse{
		PaymentId:  res.PaymentID,
		QrCodeUrl:  res.QRCodeURL,
		Status:     pb.PaymentStatus(res.Status),
	}, nil
}

func (s *PaymentServer) GetPaymentStatus(ctx context.Context, req *pb.GetPaymentStatusRequest) (*pb.GetPaymentStatusResponse, error) {
	if !validateUUID(req.PaymentId) {
		return nil, status.Error(codes.InvalidArgument, "payment_id must be a valid UUID")
	}

	res, appErr := s.uc.GetPaymentStatus(ctx, req.PaymentId)
	if appErr != nil {
		logger.Error(ctx, "GetPaymentStatus failed", slog.String("error", appErr.Error()))
		switch appErr.ErrorCode {
		case "not_found":
			return nil, status.Error(codes.NotFound, appErr.Message)
		default:
			return nil, status.Error(codes.Internal, appErr.Message)
		}
	}

	return &pb.GetPaymentStatusResponse{
		PaymentId:   res.PaymentID,
		ReferenceId: res.ReferenceID,
		PaymentType: pb.PaymentType(res.PaymentType),
		Status:      pb.PaymentStatus(res.Status),
		GatewayRef:  res.GatewayRef,
		AmountIdr:   res.AmountIDR,
	}, nil
}
