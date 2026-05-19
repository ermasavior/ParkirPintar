# ParkirPintar

## Functional Requirements

### Reservation
1. Driver can view real-time parking availability for the parking area
2. Driver can reserve a spot using system-assigned mode (fastest, system picks any available spot matching vehicle type--car or motorcycle)
3. Driver can reserve a spot using user-selected mode (Driver picks a specific spot)
4. System locks the selected spot during reservation to prevent double-booking
5. A confirmed reservation holds the spot for 1 hour from confirmation time
6. If Driver does not check in within 1 hour, the reservation expires and the spot is released back to inventory
7. A booking fee of 5,000 IDR is charged and paid directly after reservation confirmation — reservation is only confirmed after payment succeeds

### Presence
8. Driver can check in upon arriving at the parking area — billing session starts at check-in time
9. Driver can check out when finished parking — billing session ends at check-out time

### Billing & Pricing
10. First hour of parking costs 5,000 IDR; each subsequent started hour costs 5,000 IDR
11. If the parking session crosses midnight, an overnight fee of 20,000 IDR is charged per midnight crossed
12. No overstay penalty; additional time beyond reservation window is billed at the standard hourly rate
13. Billing is calculated from actual session duration, not locked at booking time
14. Driver can view the invoice after check-out

### Payment
15. Driver pays the booking fee immediately after reservation confirmation via payment gateway
16. Driver pays the parking fee at check-out via payment gateway, with handling for both success and failure scenarios
17. System must support QRIS payment method

### Assumptions
- The 5,000 IDR mentioned on reservation expiry refers to the booking fee already paid at confirmation — it is non-refundable. No additional charge is applied when a reservation expires without check-in.
- The overnight fee of 20,000 IDR is charged **per midnight crossed**, not as a one-time flat fee for the entire stay. The spec states "flat 20,000 IDR" in the context of a single overnight example; we interpret this as the unit cost per night, consistent with the "no overstay penalty, same hourly rate" principle. A 3-day stay crossing 3 midnights is charged 3 × 20,000 = 60,000 IDR in overnight fees.
- Pricing constants (booking fee, hourly rate, overnight fee) are defined as compile-time constants in the pricing engine. In production these would be sourced from a configurable store (e.g. a `pricing_config` DB table), but are hardcoded here per the fixed values stated in the assessment spec.
- The Notification Service is implemented as a stub that logs events to stdout. In production it would dispatch push notifications, SMS, or email. The event contract (NATS subjects and payloads) is fully defined and production-ready.
- Driver authentication and authorization are assumed to be handled by the API Gateway. Services trust the `driver_id` passed in requests without re-validating identity.


## Non-Functional Requirements

### Performance
1. Spot locking during reservation must complete within a short time window to minimize contention between concurrent Drivers
2. System-assigned reservation mode must respond faster than user-selected mode, as it requires no spot browsing

### Consistency
3. The system must prevent double-booking for the same spot and overlapping time windows
4. Spot inventory must reflect accurate availability in real time across all concurrent reservation requests
5. `CreateReservation` must be idempotent — retrying the same request must not create duplicate reservations
6. `Checkout/Invoice` must be idempotent — retrying the same checkout must not generate duplicate charges

### Availability
7. The system must implement retry and timeout strategies for inter-service communication
8. The system must implement circuit breakers to isolate failures in non-core services (e.g., notification, search)
9. Non-core service failures must not block core flows (reservation, check-in, check-out, billing)
10. The system must support graceful degradation when non-core services are unavailable

### Communication
11. All service-to-service communication must use gRPC over HTTP/2 or streaming events
12. All services must be written in Go

### Observability
13. All services must emit structured logs
14. All services must support distributed tracing across service boundaries

### Maintainability
15. The pricing engine must be implemented as a reusable, isolated component
16. The locking mechanism must be implemented as a reusable, isolated component
17. Configuration loading must be handled by a shared config loader component

### Scalability
18. The system is scoped to a single parking area with a fixed inventory of 150 car spots and 250 motorcycle spots; the architecture must handle concurrent reservations within this capacity without data races
