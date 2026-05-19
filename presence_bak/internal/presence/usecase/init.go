package usecase

import (
	"context"

	"parkir-pintar/services/presence/internal/presence/model"
	"parkir-pintar/services/presence/internal/presence/repository"
	"parkir-pintar/services/presence/pkg/apperror"
	"parkir-pintar/services/presence/pkg/billingclient"
)

// Presence defines the business logic contract for the presence domain
type Presence interface {
	// CheckIn validates the reservation and creates a parking session
	CheckIn(ctx context.Context, req model.CheckInRequest) (*model.CheckInResponse, *apperror.AppError)

	// CheckOut completes the session, releases the spot, and triggers billing
	CheckOut(ctx context.Context, req model.CheckOutRequest) (*model.CheckOutResponse, *apperror.AppError)

	// GetSession retrieves a session by its UUID
	GetSession(ctx context.Context, sessionID string) (*model.GetSessionResponse, *apperror.AppError)
}

// PresenceUsecase is the concrete implementation
type PresenceUsecase struct {
	repo          repository.Presence
	billingClient billingclient.BillingService
}

// NewPresence creates a new PresenceUsecase with all required dependencies
func NewPresence(repo repository.Presence, bc billingclient.BillingService) Presence {
	return &PresenceUsecase{
		repo:          repo,
		billingClient: bc,
	}
}
