# ParkirPintar — Entity Relationship Diagram

## ERD Diagram

![High-Level Architecture](docs/erd.png)

---

## Entities & Relationships

### spots
Represents a single physical parking spot in the building.
- 150 car spots (5 floors × 30)
- 250 motorcycle spots (5 floors × 50)
- Status transitions: `AVAILABLE → LOCKED → AVAILABLE`

**Relationships:**
- One spot can have many reservations over time (but only one active at a time)
- One spot can have many sessions over time (but only one active at a time)

---

### reservations
Represents a Driver's intent to park — created when a Driver reserves a spot.

**Relationships:**
- Belongs to one `spot`
- Has at most one `session` (only if Driver checks in before expiry)
- Has one `payment` of type `BOOKING_FEE`

**Status lifecycle:**
```
PENDING_PAYMENT → CONFIRMED (booking payment success)
                → CANCELLED (booking payment failed/expired)
CONFIRMED       → CHECKED_IN (Driver checks in)
                → EXPIRED (no check-in within 1 hour)
CHECKED_IN      → COMPLETED (Driver checks out)
```

---

### sessions
Represents an active or completed parking session — created when a Driver checks in.

**Relationships:**
- Belongs to one `reservation`
- Belongs to one `spot`
- Has exactly one `invoice` (created at check-out)

**Status lifecycle:**
```
ACTIVE → COMPLETED (on check-out)
```

---

### invoices
Represents the billing record generated at check-out.

**Relationships:**
- Belongs to one `session`
- References one `reservation` (to include booking fee in total)
- Has one `payment` of type `PARKING_FEE`

**Status lifecycle:**
```
PENDING_PAYMENT → PAID (parking payment success)
                → PAYMENT_FAILED (parking payment failed/expired)
```

**Fee breakdown:**
- `booking_fee_idr` — always 5,000 IDR (carried from reservation)
- `parking_fee_idr` — `ceil(duration_minutes / 60) × 5,000`
- `overnight_fee_idr` — 20,000 IDR if session crosses midnight, else 0
- `total_idr` — sum of all three

---

### payments
Represents a payment transaction against the external payment gateway.

**Relationships:**
- Polymorphic: links to either a `reservation` (booking fee) or an `invoice` (parking fee) via `reference_id` + `payment_type`

**Status lifecycle:**
```
PENDING → SUCCESS
        → FAILED
        → EXPIRED (QRIS code not scanned in time)
```

---

## Key Constraints

| Constraint | Implementation |
|---|---|
| No double-booking | Redis lock + DB spot status check before confirming reservation |
| One active reservation per Driver | Validated in `CreateReservation` flow |
| One active session per spot | Enforced by reservation status (`CHECKED_IN` blocks new reservations) |
| Idempotent reservation | `idempotency_key` UNIQUE on `reservations` |
| Idempotent invoice | `idempotency_key` UNIQUE on `invoices` |
| Idempotent payment | `idempotency_key` UNIQUE on `payments` |
