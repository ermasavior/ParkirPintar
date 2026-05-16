# ParkirPintar — High Level Architecture

## Overview

ParkirPintar is a backend system for a smart parking marketplace serving a single fixed parking area. It is designed as a set of Go microservices communicating via gRPC over HTTP/2, exposed to clients through a single API Gateway. The system prioritizes consistency (no double-booking), availability (graceful degradation of non-core services), and fast reservation response times.

---

## Architecture Diagram

See `hld-architecture.wsd` for the full architecture diagram (PlantUML format).

Open with VS Code PlantUML extension (`Alt+D`) or paste into https://editor.plantuml.com

**Services:** API Gateway · Search · Reservation · Presence · Billing · Payment · Notification

**Infrastructure:** PostgreSQL · Redis (spot locking) · NATS JetStream (async events)

**External:** Payment Gateway (QRIS)

---

## Services

### API Gateway
- Single entry point for all client requests
- Handles TLS termination, authentication, and rate limiting
- Routes requests to downstream services via gRPC

### Search Service *(non-core)*
- Returns real-time parking availability (total and per-floor)
- Filters by vehicle type (car / motorcycle)
- Reads directly from PostgreSQL
- Failure does not block reservation flow

### Reservation Service *(core)*
- Handles `CreateReservation` (system-assigned and user-selected modes)
- Acquires a short-lived distributed lock (Redis) on the spot before confirming
- Creates reservation with status `PENDING_PAYMENT`, calls Payment Service via gRPC to get QRIS code
- On `payment.booking.done` NATS event: updates reservation to `CONFIRMED`, starts 1-hour expiry window
- Manages reservation expiry via DB polling scheduler (runs every 30s, releases expired spots back to inventory)

### Presence Service *(core)*
- Handles Driver check-in and check-out
- On check-in: validates reservation is `CONFIRMED`, creates session (status: `ACTIVE`)
- On check-out: records `checked_out_at`, updates session to `COMPLETED`, releases spot to `AVAILABLE`, calls Billing Service via gRPC to generate invoice + QRIS code

### Billing Service *(core)*
- Implements the pricing engine as a reusable component
- Calculates parking fee from actual session duration (hourly rate + overnight fee if applicable)
- Creates invoice with status `PENDING_PAYMENT`, calls Payment Service via gRPC to get QRIS code
- On `payment.parking.done` NATS event: updates invoice to `PAID`
- `Checkout/Invoice` is idempotent

### Payment Service *(core)*
- Integrates with external payment gateway
- Supports QRIS payment method
- Exposes gRPC `CreatePayment` — returns QRIS code URL immediately (payment is async)
- Receives payment result via HTTP webhook callback from payment gateway
- Publishes `payment.booking.done` or `payment.parking.done` to NATS JetStream on callback
- Handles both success and failure outcomes

### Notification Service *(non-core)*
- Sends Driver notifications: reservation confirmation, expiry warning, payment receipt
- Consumes events from NATS JetStream asynchronously
- Failure is isolated — does not affect core flows

---

## Key Design Decisions

### Distributed Locking
- Redis is used for short-lived spot locks during reservation
- Lock TTL is bounded to prevent indefinite holds if a service crashes mid-flow
- Prevents double-booking under concurrent reservation requests (NFR3, NFR4)

### Idempotency
- `CreateReservation` and `Checkout/Invoice` use idempotency keys
- Duplicate requests return the existing result without side effects (NFR5, NFR6)

### Reservation Expiry
- A DB polling scheduler runs every 30s inside the Reservation Service
- Queries for reservations where `status = CONFIRMED AND expires_at < now()`
- The 1-hour window starts from `confirmed_at` — the moment booking payment succeeds, not when the reservation was created
- Atomically updates reservation to EXPIRED and releases spot to AVAILABLE in one transaction
- Publishes `reservation.expired` event to NATS JetStream for notification dispatch
- Self-healing on restart — missed expirations are caught on next poll
- Stale `PENDING_PAYMENT` reservations (created_at older than 15 minutes) are also cleaned up and spot released

### Graceful Degradation
- Search and Notification are non-core; circuit breakers isolate their failures
- Core flows (reservation, presence, billing, payment) remain unaffected if non-core services are down (NFR8, NFR9)

### gRPC Communication
- All inter-service calls use gRPC over HTTP/2
- Streaming events used for real-time availability updates where applicable (NFR11)

### Observability
- All services emit structured JSON logs with trace IDs
- Distributed tracing propagated via gRPC metadata headers (NFR13, NFR14)

---

## Deployment View (High Level)

```
┌─────────────────────────────────────────────────┐
│                  Cloud / K8s Cluster             │
│                                                  │
│  ┌──────────┐  ┌───────────┐  ┌──────────────┐  │
│  │ Gateway  │  │  Core     │  │  Non-Core    │  │
│  │ (1 pod)  │  │  Services │  │  Services    │  │
│  │          │  │  (N pods) │  │  (N pods)    │  │
│  └──────────┘  └───────────┘  └──────────────┘  │
│                                                  │
│  ┌──────────┐  ┌───────────┐  ┌──────────────┐  │
│  │ Postgres │  │   Redis   │  │  Message     │  │
│  │ (primary │  │ (cluster) │  │  Queue       │  │
│  │ + replica│  │           │  │              │  │
│  └──────────┘  └───────────┘  └──────────────┘  │
└─────────────────────────────────────────────────┘
```
