package handler

import (
	pb "parkir-pintar/services/billing/gen/billing/v1"
	"parkir-pintar/services/billing/internal/billing/usecase"

	"github.com/go-playground/validator/v10"
)

var validate = validator.New()

type BillingServer struct {
	pb.UnimplementedBillingServiceServer
	uc usecase.Billing
}

func NewBillingServer(uc usecase.Billing) *BillingServer {
	return &BillingServer{uc: uc}
}

func validateUUID(s string) bool {
	return validate.Var(s, "required,uuid") == nil
}
