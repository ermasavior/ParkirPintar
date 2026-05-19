package handler

import (
	pb "parkir-pintar/services/payment/gen/payment/v1"
	"parkir-pintar/services/payment/internal/payment/usecase"

	"github.com/go-playground/validator/v10"
)

var validate = validator.New()

// PaymentServer implements the gRPC PaymentServiceServer interface
type PaymentServer struct {
	pb.UnimplementedPaymentServiceServer
	uc usecase.Payment
}

// NewPaymentServer creates a new PaymentServer
func NewPaymentServer(uc usecase.Payment) *PaymentServer {
	return &PaymentServer{uc: uc}
}

// validateUUID returns true if s is a valid UUID
func validateUUID(s string) bool {
	return validate.Var(s, "required,uuid") == nil
}
