package model

import "time"

// SessionStatus represents the current state of a parking session
type SessionStatus int

const (
	SessionStatusActive    SessionStatus = 1
	SessionStatusCompleted SessionStatus = 2
)

// ReservationStatus mirrors the reservation service statuses needed for validation
type ReservationStatus int

const (
	ReservationStatusConfirmed  ReservationStatus = 2
	ReservationStatusCheckedIn  ReservationStatus = 4
	ReservationStatusCompleted  ReservationStatus = 5
)

// Session represents a parking session record
type Session struct {
	ID            string        `db:"id"`
	ReservationID string        `db:"reservation_id"`
	DriverID      string        `db:"driver_id"`
	SpotID        string        `db:"spot_id"`
	Status        SessionStatus `db:"status"`
	CheckedInAt   time.Time     `db:"checked_in_at"`
	CheckedOutAt  *time.Time    `db:"checked_out_at"`
}

// Reservation holds the fields needed from the reservations table for validation
type Reservation struct {
	ID        string
	DriverID  string
	SpotID    string
	Status    ReservationStatus
	ExpiresAt *time.Time
}

// CheckInRequest is the input for CheckIn
type CheckInRequest struct {
	ReservationID string    `validate:"required,uuid"`
	DriverID      string    `validate:"required,uuid"`
	CheckedInAt   time.Time `validate:"required"`
}

// CheckInResponse is the output for CheckIn
type CheckInResponse struct {
	SessionID   string
	CheckedInAt time.Time
	Status      SessionStatus
}

// CheckOutRequest is the input for CheckOut
type CheckOutRequest struct {
	SessionID    string    `validate:"required,uuid"`
	DriverID     string    `validate:"required,uuid"`
	CheckedOutAt time.Time `validate:"required"`
}

// CheckOutResponse is the output for CheckOut
type CheckOutResponse struct {
	SessionID    string
	InvoiceID    string
	CheckedOutAt time.Time
	Status       SessionStatus
	TotalIDR     int64
	QRCodeURL    string
}

// GetSessionResponse is the output for GetSession
type GetSessionResponse struct {
	SessionID     string
	ReservationID string
	DriverID      string
	SpotID        string
	Status        SessionStatus
	CheckedInAt   time.Time
	CheckedOutAt  *time.Time
}
