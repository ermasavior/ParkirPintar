package usecase

import (
	"context"

	"parkir-pintar/services/presence/internal/presence/model"
	"parkir-pintar/services/presence/pkg/apperror"
)

func (u *PresenceUsecase) GetSession(ctx context.Context, sessionID string) (*model.GetSessionResponse, *apperror.AppError) {
	session, appErr := u.repo.GetSessionByID(ctx, sessionID)
	if appErr != nil {
		return nil, appErr
	}

	return &model.GetSessionResponse{
		SessionID:     session.ID,
		ReservationID: session.ReservationID,
		DriverID:      session.DriverID,
		SpotID:        session.SpotID,
		Status:        session.Status,
		CheckedInAt:   session.CheckedInAt,
		CheckedOutAt:  session.CheckedOutAt,
	}, nil
}
