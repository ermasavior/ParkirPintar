# ParkirPintar

ParkirPintar is a backend system for a smart parking area, built as a Go microservices. It handles the full driver journey — searching for available spots, reserving one, checking in and out, and paying via QRIS — across five independent services communicating over gRPC and NATS JetStream.

The system is designed around two core principles: no double-booking and idempotent operations (all writes accept a UUID idempotency key).

### CI/CD Status

| Service | Pipeline |
|---|---|
| `reservation` | [![Golang CI/CD](https://github.com/ermasavior/parkirpintar-reservation/actions/workflows/cicd.yml/badge.svg)](https://github.com/ermasavior/parkirpintar-reservation/actions/workflows/cicd.yml) |
| `search` | [![Golang CI/CD](https://github.com/ermasavior/parkirpintar-search/actions/workflows/cicd.yml/badge.svg)](https://github.com/ermasavior/parkirpintar-search/actions/workflows/cicd.yml) |
| `presence` | [![Golang CI/CD](https://github.com/ermasavior/parkirpintar-presence/actions/workflows/cicd.yml/badge.svg)](https://github.com/ermasavior/parkirpintar-presence/actions/workflows/cicd.yml) |
| `billing` | [![Golang CI/CD](https://github.com/ermasavior/parkirpintar-billing/actions/workflows/cicd.yml/badge.svg)](https://github.com/ermasavior/parkirpintar-billing/actions/workflows/cicd.yml) |
| `payment` | [![Golang CI/CD](https://github.com/ermasavior/parkirpintar-payment/actions/workflows/cicd.yml/badge.svg)](https://github.com/ermasavior/parkirpintar-payment/actions/workflows/cicd.yml) |

---

## Architecture

![High-Level Architecture](docs/HLD.drawio.png)

The system is composed of five core services and two stubs, all communicating over gRPC. Async payment outcomes flow through NATS JetStream. Full design documentation is in [`docs/`](docs/).

| Service | Port | Role |
|---|---|---|
| `reservation` | 8082 | Reserve spots, lock inventory, schedule expiry |
| `search` | 8083 | Real-time spot availability |
| `presence` | 8085 | Check-in / check-out sessions |
| `billing` | 8084 | Fee calculation, invoices, payment retry |
| `payment` | 8086 gRPC / 8087 webhook | QRIS payment lifecycle |
| `payment-gateway` | 8088 | Stub — simulates external QRIS gateway |
| `notification` | — | Stub — simulates push/SMS/email delivery |

**Shared infrastructure:** PostgreSQL · Redis (spot locking) · NATS JetStream (async events)

---

## How It Works

A driver's full journey through the system:

```
1. Search          GET availability by vehicle type (car / motorcycle)
2. Reserve         Lock a spot → PENDING_PAYMENT → pay booking fee (5,000 IDR via QRIS)
3. Confirm         Payment gateway callback → reservation CONFIRMED → 1-hour window starts
4. Check In        Arrive at lot → session ACTIVE
5. Check Out       Leave → invoice calculated → pay parking fee via QRIS
6. Pay             Payment gateway callback → invoice PAID
```

Pricing: 5,000 IDR/started hour + 20,000 IDR per midnight crossed. Booking fee is non-refundable.

Sequence diagrams for each step: [`docs/sequence-diagram/`](docs/sequence-diagram/)

---

## API

A Postman collection covering every endpoint is available at [`docs/ParkirPintar.postman_collection.json`](docs/ParkirPintar.postman_collection.json).

Import it into Postman and update the per-service host variables to point at each service directly:

| Variable | Default | Service |
|---|---|---|
| `gateway_host` | `localhost:8082` | reservation (or whichever service you're calling) |
| `payment_service_host` | `localhost:8087` | payment webhook |


---

## Assumptions

- **Booking fee on expiry** — the 5,000 IDR booking fee paid at confirmation is non-refundable. No additional charge is applied when a reservation expires without check-in.
- **Overnight fee** — 20,000 IDR is charged *per midnight crossed*, not as a one-time flat fee. A 3-day stay crossing 3 midnights incurs 3 × 20,000 = 60,000 IDR in overnight fees.
- **Pricing constants** — booking fee, hourly rate, and overnight fee are compile-time constants in the pricing engine. In production these would come from a configurable store (e.g. a `pricing_config` table).
- **Notification Service** — implemented as a stub that logs events to stdout. The NATS event contract is fully defined and production-ready; only the delivery mechanism is stubbed.
- **Authentication** — driver auth is assumed to be handled by the API Gateway. Services trust the `driver_id` passed in requests without re-validating identity.

Full requirements and assumptions: [`docs/requirements.md`](docs/requirements.md)

---

## Data Model

Five tables: `spots → reservations → sessions → invoices`, with `payments` linking to both reservations (booking fee) and invoices (parking fee) via a polymorphic `reference_id`.

![High-Level Architecture](docs/erd.png)

Full ERD Explanation: [`docs/erd.md`](docs/erd.md)

**State lifecycles**

| Entity | States |
|---|---|
| Spot | `AVAILABLE → LOCKED → AVAILABLE` |
| Reservation | `PENDING_PAYMENT → CONFIRMED → CHECKED_IN → COMPLETED` (or `CANCELLED` / `EXPIRED`) |
| Session | `ACTIVE → COMPLETED` |
| Invoice | `PENDING_PAYMENT → PAID` (or `PAYMENT_FAILED`) |
| Payment | `PENDING → SUCCESS / FAILED / EXPIRED` |

State diagrams: [`docs/state-diagram/`](docs/state-diagram/)

---

## Key Design Decisions

**Idempotency** — `CreateReservation`, `CalculateAndCreateInvoice`, and `CreatePayment` all accept a UUID `idempotency_key`. Retries return the cached result with no side effects.

**Distributed locking** — Redis `SET NX` (TTL 30s) locks a spot before the DB write. Prevents double-booking under concurrent requests. The lock is released once the DB spot status is set to `LOCKED`.

**Async payment** — Payment results arrive via HTTP webhook from the gateway. The Payment Service validates the HMAC-SHA256 signature, updates the payment record, and publishes to NATS JetStream (`payment.booking.done` / `payment.parking.done`). Downstream services (Reservation, Billing) consume these events to update their own state.

**Reservation expiry** — A DB polling scheduler inside the Reservation Service runs every 30s. It atomically marks expired reservations and releases their spots in a single transaction. Missed expirations are caught on the next poll.

**Circuit breaker** — All outbound gRPC calls are wrapped with `gobreaker` (5 failures → OPEN, 30s recovery). Non-core service failures (Search, Notification) never block core flows.

**Pricing engine** — Isolated in `billing/pkg/pricing`. Input: `checked_in_at`, `checked_out_at`. Output: `parking_fee`, `overnight_fee`, `total`. Booking fee is stored separately on the invoice and not included in `total_idr`.

Full design: [`docs/high-level-design.md`](docs/high-level-design.md) · [`docs/low-level-design.md`](docs/low-level-design.md)

---

## Prerequisites

- Go 1.25+
- Docker + Docker Compose

---

## Running Locally

```bash
# Start all infrastructure and services
docker compose up -d

# Tail logs for a specific service
docker compose logs -f reservation-service
```

Each service reads config from `.env.docker` (compose) and `.env` (local). Copy the example and fill in your values:

```bash
cp billing/.env.example        billing/.env
cp reservation/.env.example    reservation/.env
cp presence/.env.example       presence/.env
cp payment/.env.example        payment/.env
cp search/.env.example         search/.env
```

### Database migrations (for Development)

Migrations live in `db/migrations/`. For local development, apply them with:

```bash
make migrate-docker   # runs migrations inside the running postgres container
```

Or with the `golang-migrate` CLI:

```bash
migrate -path db/migrations \
  -database "postgres://parkir:parkir@localhost:5432/parkir_pintar?sslmode=disable" up
```

---

## Unit Tests

Each service has its own test suite (`testify` + `gomock` + `pgxmock`).

```bash
# Single service
cd billing && make test

# All services
for svc in billing reservation presence payment search; do
  echo "=== $svc ===" && (cd $svc && make test)
done
```

---

## End-to-End Tests

The `e2e/` module runs every service as a real Docker container — real Postgres, Redis, and NATS, no mocks. Tests call the actual gRPC APIs and observe state changes across service boundaries. Payment outcomes are driven by posting a signed webhook directly to the payment service (the same HTTP contract the real gateway uses), making results deterministic without an external dependency.

Each test calls `NewSuite(t)`, which spins up the full stack, runs a one-shot `migrate` container to seed the DB, discovers the dynamically assigned host ports, and wires up gRPC clients. `t.Cleanup()` tears everything down after the test.

### Run

```bash
make build-e2e   # build all service images (required once)
make test-e2e    # run all E2E tests
```

### Test coverage

| Group | File | Scenarios |
|---|---|---|
| Happy path | `happy_path_test.go` | Full flow, overnight fee, booking failure, parking retry |
| Reservation | `reservation_billing_test.go` | Create, idempotency, duplicate driver, NATS payment events |
| Search | `search_test.go` | Availability by vehicle type, list spots, count decreases after reserve |
| Presence | `presence_test.go` | Check-in, check-out, get session, rejection cases |
| Payment | `payment_test.go` | Create, idempotency, status, webhook callbacks (SUCCESS/FAILED/EXPIRED) |

<details>
<summary>Full test results</summary>

```
--- PASS: TestE2E_FullHappyPath (16.21s)
--- PASS: TestE2E_FullHappyPath_Overnight (14.39s)
--- PASS: TestE2E_BookingPaymentFailed_ReservationCancelled (12.54s)
--- PASS: TestE2E_ParkingPaymentFailed_InvoicePaymentFailed_RetryPayment (13.38s)
--- PASS: TestE2E_CreateReservation_SystemAssigned (11.94s)
--- PASS: TestE2E_CreateReservation_Idempotency (12.11s)
--- PASS: TestE2E_CreateReservation_DuplicateDriver (12.38s)
--- PASS: TestE2E_GetReservation (11.87s)
--- PASS: TestE2E_ParkingPaymentDone_Success (13.02s)
--- PASS: TestE2E_ParkingPaymentDone_Failed (12.44s)
--- PASS: TestE2E_ParkingPaymentDone_Expired (12.19s)
--- PASS: TestE2E_CreatePayment_BookingFee (13.51s)
--- PASS: TestE2E_CreatePayment_ParkingFee (11.82s)
--- PASS: TestE2E_CreatePayment_Idempotency (13.35s)
--- PASS: TestE2E_GetPaymentStatus_Pending (11.75s)
--- PASS: TestE2E_GetPaymentStatus_NotFound (13.31s)
--- PASS: TestE2E_WebhookCallback_BookingFeeSuccess (12.40s)
--- PASS: TestE2E_WebhookCallback_BookingFeeFailed (12.67s)
--- PASS: TestE2E_WebhookCallback_ParkingFeeExpired (12.17s)
--- PASS: TestE2E_CheckIn_Success (13.23s)
--- PASS: TestE2E_CheckIn_UnconfirmedReservation (11.63s)
--- PASS: TestE2E_CheckIn_WrongDriver (12.53s)
--- PASS: TestE2E_GetSession (12.68s)
--- PASS: TestE2E_CheckOut_Success (15.67s)
--- PASS: TestE2E_CheckOut_SessionCompleted (15.42s)
--- PASS: TestE2E_GetSession_AfterCheckOut (11.58s)
--- PASS: TestE2E_GetAvailability_Car (13.45s)
--- PASS: TestE2E_GetAvailability_Motorcycle (11.84s)
--- PASS: TestE2E_ListSpots_Car (12.24s)
--- PASS: TestE2E_ListSpots_Motorcycle (14.24s)
--- PASS: TestE2E_GetAvailability_DecreasesAfterReservation (14.07s)
PASS
```

</details>

---

## Project Structure

```
ParkirPintar/
├── billing/                    # Invoice calculation, pricing engine, payment retry
├── reservation/                # Spot reservation, expiry scheduler
├── presence/                   # Check-in / check-out
├── payment/                    # QRIS payment lifecycle + webhook handler
├── search/                     # Spot availability queries
├── stubs/
│   ├── payment-gateway/        # Simulated QRIS gateway (HTML pay page + callback)
│   └── notification/           # Simulated notification delivery (logs to stdout)
├── db/
│   └── migrations/             # Shared PostgreSQL migrations (schema + seed data)
├── e2e/                        # End-to-end tests (testcontainers-compose)
├── docs/
│   ├── requirements.md
│   ├── high-level-design.md
│   ├── low-level-design.md
│   ├── erd.md
│   ├── HLD.drawio.png
│   ├── sequence-diagram/       # Per-flow sequence diagrams (.wsd + .png)
│   └── state-diagram/          # Per-entity state diagrams (.wsd + .png)
├── docker-compose.yml          # Local development stack
├── docker-compose.integration.yml  # Isolated stack for E2E tests
└── Makefile
```

---

## Makefile

```bash
make migrate-docker   # Apply DB migrations into the running postgres container
make build-e2e        # Build all service Docker images
make test-e2e         # Build images + run E2E tests
```
