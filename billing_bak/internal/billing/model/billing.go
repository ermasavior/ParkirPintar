package model

import "time"

type InvoiceStatus int

const (
	InvoiceStatusPendingPayment InvoiceStatus = 1
	InvoiceStatusPaid           InvoiceStatus = 2
	InvoiceStatusPaymentFailed  InvoiceStatus = 3
)

type Invoice struct {
	ID              string        `db:"id"`
	IdempotencyKey  string        `db:"idempotency_key"`
	SessionID       string        `db:"session_id"`
	ReservationID   string        `db:"reservation_id"`
	DriverID        string        `db:"driver_id"`
	Status          InvoiceStatus `db:"status"`
	BookingFeeIDR   int64         `db:"booking_fee_idr"`
	ParkingFeeIDR   int64         `db:"parking_fee_idr"`
	OvernightFeeIDR int64         `db:"overnight_fee_idr"`
	TotalIDR        int64         `db:"total_idr"`
	QRCodeURL       string        `db:"qr_code_url"`
	CreatedAt       time.Time     `db:"created_at"`
}

type CreateInvoiceRequest struct {
	IdempotencyKey string    `validate:"required,uuid"`
	SessionID      string    `validate:"required,uuid"`
	ReservationID  string    `validate:"required,uuid"`
	DriverID       string    `validate:"required,uuid"`
	CheckedInAt    time.Time `validate:"required"`
	CheckedOutAt   time.Time `validate:"required"`
}

type CreateInvoiceResponse struct {
	InvoiceID       string
	BookingFeeIDR   int64
	ParkingFeeIDR   int64
	OvernightFeeIDR int64
	TotalIDR        int64
	QRCodeURL       string
}

type GetInvoiceResponse struct {
	InvoiceID       string
	SessionID       string
	ReservationID   string
	Status          InvoiceStatus
	BookingFeeIDR   int64
	ParkingFeeIDR   int64
	OvernightFeeIDR int64
	TotalIDR        int64
	CreatedAt       time.Time
}

type RetryPaymentRequest struct {
	InvoiceID string `validate:"required,uuid"`
	DriverID  string `validate:"required,uuid"`
}

type RetryPaymentResponse struct {
	PaymentID string
	QRCodeURL string
}

type NATSPaymentDoneEvent struct {
	ReferenceID string `json:"reference_id"` // invoice_id
	Status      string `json:"status"`       // "SUCCESS" | "FAILED" | "EXPIRED"
}
