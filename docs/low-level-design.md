# ParkirPintar — Low Level Design

## Directory Structure

```
parkir-pintar/
├── proto/                          # Shared protobuf definitions
│   ├── search/v1/search.proto
│   ├── reservation/v1/reservation.proto
│   ├── presence/v1/presence.proto
│   ├── billing/v1/billing.proto
│   ├── payment/v1/payment.proto
│   └── notification/v1/notification.proto
├── services/
│   ├── gateway/                    # API Gateway
│   ├── search/                     # Search Service
│   ├── reservation/                # Reservation Service
│   ├── presence/                   # Presence Service
│   ├── billing/                    # Billing Service
│   ├── payment/                    # Payment Service
│   └── notification/               # Notification Service
├── pkg/                            # Shared reusable components
│   ├── pricing/                    # Pricing engine
│   ├── lock/                       # Distributed locking
│   ├── config/                     # Config loader
│   ├── logger/                     # Structured logging
│   └── tracer/                     # Distributed tracing
├── infra/                          # Infrastructure configs
│   ├── postgres/
│   ├── redis/
│   ├── nats/
│   └── k8s/
└── tests/
    ├── unit/
    ├── integration/
    └── e2e/
```

---

## gRPC API Contracts

### Search Service
```protobuf
service SearchService {
  rpc GetAvailability (GetAvailabilityRequest) returns (GetAvailabilityResponse);
  rpc ListSpots (ListSpotsRequest) returns (ListSpotsResponse);
}

message GetAvailabilityRequest {
  VehicleType vehicle_type = 1;  // CAR | MOTORCYCLE
}

message GetAvailabilityResponse {
  int32 total_available = 1;
  repeated FloorAvailability floors = 2;
}

message FloorAvailability {
  int32 floor_number = 1;
  int32 available_spots = 2;
  VehicleType vehicle_type = 3;
}

message ListSpotsRequest {
  int32 floor_number = 1;
  VehicleType vehicle_type = 2;
}

message ListSpotsResponse {
  repeated Spot spots = 1;
}

message Spot {
  string spot_id = 1;            // UUID
  int32 floor_number = 2;
  string spot_code = 3;          // e.g. "A1", "B12"
  VehicleType vehicle_type = 4;
  SpotStatus status = 5;         // AVAILABLE | LOCKED
}
```

### Reservation Service
```protobuf
service ReservationService {
  rpc CreateReservation (CreateReservationRequest) returns (CreateReservationResponse);
  rpc GetReservation (GetReservationRequest) returns (GetReservationResponse);
}

message CreateReservationRequest {
  string idempotency_key = 1;    // Client-generated UUID
  string driver_id = 2;          // UUID
  VehicleType vehicle_type = 3;
  AssignmentMode mode = 4;       // SYSTEM_ASSIGNED | USER_SELECTED
  string spot_id = 5;            // UUID — required only for USER_SELECTED mode
}

message CreateReservationResponse {
  string reservation_id = 1;     // UUID
  string spot_id = 2;            // UUID
  string spot_code = 3;
  int32 floor_number = 4;
  ReservationStatus status = 5;    // PENDING_PAYMENT
  string qr_code_url = 6;          // QRIS code URL for booking fee payment
}

message GetReservationRequest {
  string reservation_id = 1;     // UUID
}

enum ReservationStatus {
  PENDING_PAYMENT = 0;
  CONFIRMED = 1;
  EXPIRED = 2;
  CHECKED_IN = 3;
  COMPLETED = 4;
  CANCELLED = 5;
}
```

### Presence Service
```protobuf
service PresenceService {
  rpc CheckIn (CheckInRequest) returns (CheckInResponse);
  rpc CheckOut (CheckOutRequest) returns (CheckOutResponse);
  rpc GetSession (GetSessionRequest) returns (GetSessionResponse);
}

message CheckInRequest {
  string reservation_id = 1;     // UUID
  string driver_id = 2;          // UUID
  google.protobuf.Timestamp checked_in_at = 3;
}

message CheckInResponse {
  string session_id = 1;         // UUID
  google.protobuf.Timestamp checked_in_at = 2;
  SessionStatus status = 3;      // ACTIVE
}

message CheckOutRequest {
  string session_id = 1;         // UUID
  string driver_id = 2;          // UUID
  google.protobuf.Timestamp checked_out_at = 3;
}

message CheckOutResponse {
  string session_id = 1;         // UUID
  string invoice_id = 2;         // UUID
  google.protobuf.Timestamp checked_out_at = 3;
  SessionStatus status = 4;      // COMPLETED
  int64 total_idr = 5;           // Total parking fee due
  string qr_code_url = 6;        // QRIS code URL for parking fee payment
}

enum SessionStatus {
  ACTIVE = 0;
  COMPLETED = 1;
}
```

### Billing Service
```protobuf
service BillingService {
  rpc CalculateAndCreateInvoice (CreateInvoiceRequest) returns (CreateInvoiceResponse);
  rpc GetInvoice (GetInvoiceRequest) returns (GetInvoiceResponse);
  rpc RetryPayment (RetryPaymentRequest) returns (RetryPaymentResponse);
}

message CreateInvoiceRequest {
  string idempotency_key = 1;    // UUID
  string session_id = 2;         // UUID
  string reservation_id = 3;     // UUID
  google.protobuf.Timestamp checked_in_at = 4;
  google.protobuf.Timestamp checked_out_at = 5;
}

message CreateInvoiceResponse {
  string invoice_id = 1;         // UUID
  int64 booking_fee_idr = 2;     // Always 5000
  int64 parking_fee_idr = 3;     // Calculated from session duration
  int64 overnight_fee_idr = 4;   // 20000 if crosses midnight, else 0
  int64 total_idr = 5;
  string qr_code_url = 6;        // QRIS code URL for parking fee payment
}

message GetInvoiceRequest {
  string invoice_id = 1;         // UUID
}

message RetryPaymentRequest {
  string invoice_id = 1;         // UUID
  string driver_id  = 2;         // UUID — validated against invoice owner
}

message RetryPaymentResponse {
  string payment_id  = 1;        // UUID
  string qr_code_url = 2;        // Fresh QRIS code for retry attempt
}
```

**RetryPayment flow:**
```
1. Validate invoice_id belongs to driver_id
2. Validate invoice status == PAYMENT_FAILED (guard against retrying a PAID invoice)
3. Update invoice status → PENDING_PAYMENT
4. Call PaymentService.CreatePayment with a new idempotency_key
   → new payment record created (previous FAILED/EXPIRED record preserved for audit)
5. Return new qr_code_url to Driver
```

### Payment Service
```protobuf
// Internal gRPC API — called by Reservation Service and Billing Service
service PaymentService {
  rpc CreatePayment (CreatePaymentRequest) returns (CreatePaymentResponse);
  rpc GetPaymentStatus (GetPaymentStatusRequest) returns (GetPaymentStatusResponse);
}

message CreatePaymentRequest {
  string idempotency_key = 1;    // UUID
  string reference_id = 2;       // UUID — reservation_id or invoice_id
  PaymentType payment_type = 3;  // BOOKING_FEE | PARKING_FEE
  int64 amount_idr = 4;
  string driver_id = 5;          // UUID
  PaymentMethod method = 6;      // QRIS
}

message CreatePaymentResponse {
  string payment_id = 1;         // UUID
  string qr_code_url = 2;        // QRIS code URL
  PaymentStatus status = 3;      // PENDING | SUCCESS | FAILED
}

enum PaymentType {
  BOOKING_FEE = 0;
  PARKING_FEE = 1;
}

enum PaymentStatus {
  PENDING = 0;
  SUCCESS = 1;
  FAILED = 2;
  EXPIRED = 3;
}
```

**Inbound HTTP Webhook — Payment Gateway Callback**

`HandleCallback` is NOT a gRPC endpoint. It is an HTTP POST endpoint exposed by the Payment Service to receive asynchronous payment results from the external payment gateway. The gateway vendor dictates the HTTP protocol — we have no control over it.

```
POST /webhook/payment/callback
Content-Type: application/json
X-Webhook-Signature: <HMAC-SHA256 of payload using shared secret>

{
  "gateway_ref":    "GW-REF-123456",   // External gateway transaction reference
  "reference_id":   "<UUID>",          // reservation_id or invoice_id
  "payment_type":   "BOOKING_FEE",     // BOOKING_FEE | PARKING_FEE
  "status":         "SUCCESS",         // SUCCESS | FAILED | EXPIRED
  "amount_idr":     5000,
  "paid_at":        "2026-05-14T08:05:00Z"
}
```

**Callback handling flow:**
```
1. Validate X-Webhook-Signature (HMAC-SHA256 with shared secret)
   → reject with 401 if invalid
2. Parse payload and look up payment record by reference_id + payment_type
3. Update payment record:
   - status=SUCCESS  → payments.status = SUCCESS, gateway_ref = gateway_ref
   - status=FAILED   → payments.status = FAILED
   - status=EXPIRED  → payments.status = EXPIRED
4. Publish to NATS JetStream:
   - payment_type=BOOKING_FEE → publish payment.booking.done { reservation_id, status }
   - payment_type=PARKING_FEE → publish payment.parking.done { invoice_id, status }
5. Return 200 OK immediately
   → gateway retries on non-2xx response (idempotency key guards against duplicate processing)
```

**Note:** This endpoint is exposed directly on the Payment Service, bypassing the API Gateway. It must be reachable by the payment gateway's IP range and should be protected by signature verification only — not by the internal auth mechanism used for client-facing APIs.

---

## Service Internals

### Reservation Service — CreateReservation Flow

```
Client → Gateway → ReservationService.CreateReservation

1. Check idempotency key in DB (JOIN spots) → if exists, return cached response (includes spot_code, floor_number, qr_code_url)
2. Validate driver_id has no active reservation (status: PENDING_PAYMENT or CONFIRMED)
3. If USER_SELECTED:
     a. Acquire Redis lock on spot_id (TTL: 30s)
     b. Verify spot is AVAILABLE in DB
   If SYSTEM_ASSIGNED:
     a. Query DB for any AVAILABLE spot matching vehicle_type
     b. Acquire Redis lock on selected spot_id (TTL: 30s)
4. Write reservation record to DB (status: PENDING_PAYMENT)
5. Update spot status to LOCKED in DB
6. Release Redis lock (spot now protected by DB status)
7. Call PaymentService.CreatePayment (booking fee = 5,000 IDR)
   → Returns QRIS code URL immediately (payment is async)
   → Store qr_code_url on reservation record for idempotency replay
8. Return CreateReservationResponse with QRIS code URL to Driver

--- Driver scans QR code and pays on banking app ---

9. Payment gateway calls Payment Service webhook
10. Payment Service publishes payment.booking.done → NATS JetStream
    { reservation_id, status: SUCCESS|FAILED }

11. Reservation Service consumes payment.booking.done:
    If SUCCESS:
      a. Update reservation status → CONFIRMED
      b. Set expires_at = now() + 1 hour
    If FAILED:
      a. Update reservation status → CANCELLED
      b. Update spot status → AVAILABLE
```

### Presence Service — CheckIn Flow

```
Driver App → Gateway → PresenceService.CheckIn

1. Validate reservation_id belongs to driver_id
2. Validate reservation status == CONFIRMED
3. Validate expires_at > now() (reservation not expired)
4. INSERT session record (status=ACTIVE, checked_in_at=now())
5. UPDATE reservation status → CHECKED_IN
6. Return CheckInResponse { session_id, checked_in_at, status=ACTIVE }
```

```
DB Polling Scheduler (Reservation Service — runs every 30s)

1. Query DB:
   SELECT id, spot_id FROM reservations
   WHERE status = 'CONFIRMED' AND expires_at < now()

2. For each expired reservation (in a single DB transaction):
   a. UPDATE reservations SET status = 'EXPIRED' WHERE id = $1
   b. UPDATE spots SET status = 'AVAILABLE' WHERE id = $2

3. Publish reservation.expired event → NATS JetStream (for notification)

Note: idempotent — if scheduler runs twice, second UPDATE affects 0 rows
```

### Presence Service — CheckOut Flow

```
Driver App → Gateway → PresenceService.CheckOut

1. Validate session_id belongs to driver_id
2. Verify session status == ACTIVE
3. Record checked_out_at timestamp
4. Update session status → COMPLETED
5. Update spot status → AVAILABLE (spot released immediately on checkout)
6. Update reservation status → COMPLETED
7. Call BillingService.CalculateAndCreateInvoice
     - Pass: session_id, checked_in_at, checked_out_at
     - Returns: invoice_id + QRIS code URL
8. Return CheckOutResponse with invoice_id + QRIS code URL to Driver

--- Driver scans QR code and pays on banking app ---

9. Payment gateway calls Payment Service webhook
10. Payment Service publishes payment.parking.done → NATS JetStream
    { invoice_id, status: SUCCESS|FAILED|EXPIRED }

11. Billing Service consumes payment.parking.done:
    If SUCCESS:
      a. Update invoice status → PAID
    If FAILED or EXPIRED:
      a. Update invoice status → PAYMENT_FAILED
      b. Driver must call BillingService.RetryPayment to get a new QRIS code
```

### Billing Service — Pricing Engine

```
pkg/pricing/engine.go

Input:  checked_in_at, checked_out_at
Output: parking_fee_idr, overnight_fee_idr

Rules:
1. duration = checked_out_at - checked_in_at
2. hours = ceil(duration.Minutes() / 60)   // each started hour counts
3. parking_fee = hours * 5000
4. overnight_fee = 20000 * number of midnights crossed
   Assumption: the spec states "flat 20,000 IDR" in a single-night example.
   We interpret this as 20,000 IDR per midnight crossed — consistent with the
   "no overstay penalty, same hourly rate" principle. A 3-day stay crossing
   3 midnights is charged 3 × 20,000 = 60,000 IDR overnight fee.
5. total = booking_fee(5000) + parking_fee + overnight_fee
```

---

## Shared Infrastructure

### PostgreSQL — Schema Overview

```sql
-- Parking area inventory
spots (
  id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  floor_number   INT NOT NULL,          -- 1..5
  spot_code      VARCHAR(10) NOT NULL,  -- e.g. "A1"
  vehicle_type   SMALLINT NOT NULL,     -- 1=CAR, 2=MOTORCYCLE
  status         SMALLINT NOT NULL      -- 1=AVAILABLE, 2=LOCKED
)

-- Reservations
reservations (
  id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  idempotency_key  UUID UNIQUE NOT NULL,
  driver_id        UUID NOT NULL,
  spot_id          UUID NOT NULL REFERENCES spots(id),
  vehicle_type     SMALLINT NOT NULL,
  assignment_mode  SMALLINT NOT NULL,   -- 1=SYSTEM_ASSIGNED, 2=USER_SELECTED
  status           SMALLINT NOT NULL,   -- 1=PENDING_PAYMENT, 2=CONFIRMED, 3=EXPIRED,
                                        -- 4=CHECKED_IN, 5=COMPLETED, 6=CANCELLED
  qr_code_url      TEXT NOT NULL DEFAULT '',  -- stored at creation for idempotency replay
  confirmed_at     TIMESTAMPTZ,
  expires_at       TIMESTAMPTZ,         -- confirmed_at + 1 hour (set on CONFIRMED)
  created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
)

-- Parking sessions
sessions (
  id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  reservation_id   UUID NOT NULL REFERENCES reservations(id),
  driver_id        UUID NOT NULL,
  spot_id          UUID NOT NULL REFERENCES spots(id),
  status           SMALLINT NOT NULL,   -- 1=ACTIVE, 2=COMPLETED
  checked_in_at    TIMESTAMPTZ NOT NULL,
  checked_out_at   TIMESTAMPTZ
)

-- Invoices
invoices (
  id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  idempotency_key   UUID UNIQUE NOT NULL,
  session_id        UUID NOT NULL REFERENCES sessions(id),
  reservation_id    UUID NOT NULL REFERENCES reservations(id),
  status            SMALLINT NOT NULL,  -- 1=PENDING_PAYMENT, 2=PAID, 3=PAYMENT_FAILED
  booking_fee_idr   BIGINT NOT NULL DEFAULT 5000,
  parking_fee_idr   BIGINT NOT NULL,
  overnight_fee_idr BIGINT NOT NULL DEFAULT 0,
  total_idr         BIGINT NOT NULL,
  qr_code_url       TEXT NOT NULL DEFAULT '',  -- stored at creation for idempotency replay
  created_at        TIMESTAMPTZ NOT NULL DEFAULT now()
)

-- Payments
payments (
  id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  idempotency_key  UUID UNIQUE NOT NULL,
  reference_id     UUID NOT NULL,       -- reservations.id or invoices.id
  payment_type     SMALLINT NOT NULL,   -- 1=BOOKING_FEE, 2=PARKING_FEE
  amount_idr       BIGINT NOT NULL,
  method           SMALLINT NOT NULL,   -- 1=QRIS
  status           SMALLINT NOT NULL,   -- 1=PENDING, 2=SUCCESS, 3=FAILED, 4=EXPIRED
  gateway_ref      VARCHAR(255),        -- External gateway reference
  created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
)
```

### Redis — Key Patterns

```
# Spot lock during reservation (TTL: 30s)
lock:spot:{spot_id}  →  driver_id   -- spot_id is UUID
```

### Message Queue — NATS JetStream Subjects

```
reservation.expired       →  Notification Service (send expiry alert to Driver)
payment.booking.done      →  Notification Service (send booking payment receipt/failure to Driver)
                          →  Reservation Service (confirm reservation on payment success/failure)
payment.parking.done      →  Notification Service (send parking payment receipt/failure to Driver)
                          →  Billing Service (mark invoice paid on payment success/failure)
```

---

## Shared Reusable Components (`pkg/`)

### `pkg/pricing` — Pricing Engine
```go
type PricingEngine interface {
    Calculate(checkedInAt, checkedOutAt time.Time) PricingResult
}

type PricingResult struct {
    ParkingFeeIDR  int64
    OvernightFeeIDR int64
    TotalFeeIDR    int64
    DurationHours  int
    IsOvernight    bool
}
```

### `pkg/lock` — Distributed Lock
```go
type DistributedLock interface {
    Acquire(ctx context.Context, key string, ttl time.Duration) (bool, error)
    Release(ctx context.Context, key string) error
}
// Backed by Redis SET NX PX
```

### `pkg/config` — Config Loader
```go
type Config struct {
    PostgresDSN     string
    RedisAddr       string
    NatsURL         string
    GatewayPort     int
    // per-service fields...
}
// Loads from env vars + optional config file
```

### `pkg/logger` — Structured Logging
```go
// Wraps zerolog or zap
// Emits JSON with: service, trace_id, span_id, level, message, timestamp
```

### `pkg/tracer` — Distributed Tracing
```go
// OpenTelemetry SDK
// Propagates trace context via gRPC metadata headers
```

---

## Resilience Patterns

### Circuit Breaker (non-core services)
- Applied on Gateway calls to Search and Notification
- States: CLOSED → OPEN → HALF-OPEN
- Threshold: 5 consecutive failures → OPEN
- Recovery: 30s timeout before HALF-OPEN probe

### Retry Policy (core inter-service calls)
- Max retries: 3
- Backoff: exponential with jitter
- Applied to: Reservation → Payment, Presence → Billing, Billing → Payment

### Timeout Policy
- All gRPC calls have a context deadline
- Default timeout: 5s for core calls, 2s for non-core calls

### Spot Contention
- On concurrent requests for the same spot, the system rejects all but the first via Redis distributed lock
- Rejected Drivers receive an immediate error and must retry or select another spot
- No waitlist or queue is implemented for spot contention
