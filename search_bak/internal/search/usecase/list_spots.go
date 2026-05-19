package usecase

import (
	"context"

	"parkir-pintar/services/search/internal/search/model"
	"parkir-pintar/services/search/pkg/apperror"
)

func (u *SearchUsecase) ListSpots(ctx context.Context, req model.ListSpotsRequest) (*model.ListSpotsResponse, *apperror.AppError) {
	spots, appErr := u.repo.ListSpots(ctx, req.FloorNumber, req.VehicleType)
	if appErr != nil {
		return nil, appErr
	}

	return &model.ListSpotsResponse{Spots: spots}, nil
}
