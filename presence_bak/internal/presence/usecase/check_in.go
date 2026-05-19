package usecase

import (
	"context"
	"log/slog"
	"time"

	"parkir-pintar/services/presence/internal/presence/model"
	"parkir-pintar/services/presence/pkg/apperror"
	"parkir-pintar/services/presence/pkg/logger"
)

func (u *PresenceUsecase) CheckIn(ctx context.Context, req model.CheckInRequest) (*model.CheckInResponse, *apperror.AppError) {
	// Fetch and validate reservation ownership
	reservation, appErr := u.repo.GetReservationForCheckIn(ctx, req.ReservationID, req.DriverID)
	if appErr != nil {
		return nil, appErr
	}

	// Validate reservation status
	if reservation.Status != model.ReservationStatusConfirmed {
		return nil, apperror.New("conflict", "reservation booking fee has not been paid yet — please complete payment before checking in")
	}

	// Validate reservation does not expired
	if reservation.ExpiresAt != nil && time.Now().After(*reservation.ExpiresAt) {
		return nil, apperror.New("conflict", "reservation has expired")
	}

	// Create session and mark reservation as CHECKED_IN
	checkedInAt := req.CheckedInAt
	if checkedInAt.IsZero() {
		checkedInAt = time.Now()
	}

	session, appErr := u.repo.CreateSessionAndMarkCheckedIn(ctx, req.ReservationID, req.DriverID, reservation.SpotID, checkedInAt)
	if appErr != nil {
		return nil, appErr
	}

	logger.Info(ctx, "CheckIn: session created",
		slog.String("session_id", session.ID),
		slog.String("reservation_id", req.ReservationID),
	)

	return &model.CheckInResponse{
		SessionID:   session.ID,
		CheckedInAt: session.CheckedInAt,
		Status:      model.SessionStatusActive,
	}, nil
}
