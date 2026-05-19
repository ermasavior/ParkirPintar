package usecase

import (
	"context"

	"parkir-pintar/services/search/internal/search/model"
	"parkir-pintar/services/search/pkg/apperror"
)

func (u *SearchUsecase) GetAvailability(ctx context.Context, req model.GetAvailabilityRequest) (*model.GetAvailabilityResponse, *apperror.AppError) {
	floors, appErr := u.repo.GetAvailability(ctx, req.VehicleType)
	if appErr != nil {
		return nil, appErr
	}

	total := 0
	for _, f := range floors {
		total += f.AvailableSpots
	}

	return &model.GetAvailabilityResponse{
		TotalAvailable: total,
		Floors:         floors,
	}, nil
}
