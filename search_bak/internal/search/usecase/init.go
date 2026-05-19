package usecase

import (
	"context"

	"parkir-pintar/services/search/internal/search/model"
	"parkir-pintar/services/search/internal/search/repository"
	"parkir-pintar/services/search/pkg/apperror"
)

type Search interface {
	// GetAvailability returns total available spots and per-floor breakdown for a vehicle type
	GetAvailability(ctx context.Context, req model.GetAvailabilityRequest) (*model.GetAvailabilityResponse, *apperror.AppError)

	// ListSpots returns all spots on a given floor for a vehicle type
	ListSpots(ctx context.Context, req model.ListSpotsRequest) (*model.ListSpotsResponse, *apperror.AppError)
}

type SearchUsecase struct {
	repo repository.Search
}

func NewSearch(repo repository.Search) Search {
	return &SearchUsecase{repo: repo}
}
