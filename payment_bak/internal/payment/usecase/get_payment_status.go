package usecase

import (
	"context"

	"parkir-pintar/services/payment/internal/payment/model"
	"parkir-pintar/services/payment/pkg/apperror"
)

func (u *PaymentUsecase) GetPaymentStatus(ctx context.Context, paymentID string) (*model.GetPaymentStatusResponse, *apperror.AppError) {
	p, appErr := u.repo.GetByID(ctx, paymentID)
	if appErr != nil {
		return nil, appErr
	}

	return &model.GetPaymentStatusResponse{
		PaymentID:   p.ID,
		ReferenceID: p.ReferenceID,
		PaymentType: p.PaymentType,
		Status:      p.Status,
		GatewayRef:  p.GatewayRef,
		AmountIDR:   p.AmountIDR,
	}, nil
}
