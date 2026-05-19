package model

import "time"

// PaymentType represents the type of payment
type PaymentType int

const (
	PaymentTypeBookingFee PaymentType = 1
	PaymentTypeParkingFee PaymentType = 2
)

// PaymentMethod represents the payment method
type PaymentMethod int

const (
	PaymentMethodQRIS PaymentMethod = 1
)

// PaymentStatus represents the current state of a payment
type PaymentStatus int

const (
	PaymentStatusPending PaymentStatus = 1
	PaymentStatusSuccess PaymentStatus = 2
	PaymentStatusFailed  PaymentStatus = 3
	PaymentStatusExpired PaymentStatus = 4
)

// Payment represents a payment record
type Payment struct {
	ID             string        `db:"id"`
	IdempotencyKey string        `db:"idempotency_key"`
	ReferenceID    string        `db:"reference_id"` // reservation_id or invoice_id
	PaymentType    PaymentType   `db:"payment_type"`
	AmountIDR      int64         `db:"amount_idr"`
	Method         PaymentMethod `db:"method"`
	Status         PaymentStatus `db:"status"`
	GatewayRef     string        `db:"gateway_ref"`
	QRCodeURL      string        `db:"qr_code_url"`
	CreatedAt      time.Time     `db:"created_at"`
	UpdatedAt      time.Time     `db:"updated_at"`
}

// CreatePaymentRequest is the input for CreatePayment
type CreatePaymentRequest struct {
	IdempotencyKey string        `validate:"required,uuid"`
	ReferenceID    string        `validate:"required,uuid"`
	PaymentType    PaymentType   `validate:"required,oneof=1 2"`
	AmountIDR      int64         `validate:"required,gt=0"`
	DriverID       string        `validate:"required,uuid"`
	Method         PaymentMethod `validate:"required,oneof=1"`
}

// CreatePaymentResponse is the output for CreatePayment
type CreatePaymentResponse struct {
	PaymentID  string
	QRCodeURL  string
	Status     PaymentStatus
}

// GetPaymentStatusResponse is the output for GetPaymentStatus
type GetPaymentStatusResponse struct {
	PaymentID   string
	ReferenceID string
	PaymentType PaymentType
	Status      PaymentStatus
	GatewayRef  string
	AmountIDR   int64
}

// WebhookCallbackRequest is the inbound HTTP webhook payload from the payment gateway
type WebhookCallbackRequest struct {
	GatewayRef  string `json:"gateway_ref"`
	ReferenceID string `json:"reference_id"`
	PaymentType string `json:"payment_type"` // "BOOKING_FEE" | "PARKING_FEE"
	Status      string `json:"status"`       // "SUCCESS" | "FAILED" | "EXPIRED"
	AmountIDR   int64  `json:"amount_idr"`
	PaidAt      string `json:"paid_at"`
}

// NATSPaymentDoneEvent is published to NATS after a callback is processed
type NATSPaymentDoneEvent struct {
	ReferenceID string `json:"reference_id"` // reservation_id or invoice_id
	Status      string `json:"status"`       // "SUCCESS" | "FAILED" | "EXPIRED"
}
