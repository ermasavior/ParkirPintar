# Solution Development Assessment 2026 – Smart Parking Marketplace (ParkirPintar)

Congratulations on your next assessment, how are you doing so far? Have fun with this assessment, and as always do your best! This time you are going to manage a Smart Parking System named **ParkirPintar**. ParkirPintar supports Drivers who need parking at a single, fixed parking area. The system uses location data from smartphones to detect where the Driver is, inform availability for that parking area, and allow Drivers to reserve a spot quickly. The parking area is operated as one centralized inventory.

The difference between ParkirPintar and other parking apps is that our system is very lite, simple, and fast. A Driver clicks the reserve button, the system will lock the inventory for a short time, and once confirmed the Driver can navigate to the parking area and the assigned spot. The transaction begins when the Driver checks in and ends when the Driver checks out. The parking area is a parking building with **5 floors**; each floor has capacity for **30 cars** and **50 motorcycles** (total capacity: **150 cars** and **250 motorcycles**).

The connection between Driver and Host can be visualized as:

- The system manages a single parking area (one district/area only) with a centralized inventory; there is no Host onboarding or spot publishing.
- Driver will view availability and reserve a spot within that parking area (no multi-area search radius).
- Spot assignment modes:
  - **(1) System-assigned (fastest)** — the system immediately assigns any currently available spot that matches the vehicle type.
  - **(2) User-selected** — the Driver can choose a specific spot, but the spot list may involve a short hold of time during payment accomplishment.
- When the Driver reserves, the system validates capacity and availability, locks the inventory for a short time to prevent double booking, and confirms the reservation.
- Once confirmed, the reservation is active, and the Driver will park in the assigned spot.
- **Reservation hold time:** A booking holds the assigned spot for **1 hour** only. If the Driver does not check in/park within 1 hour after confirmation, the reservation or book the place will cost **5,000 IDR** and will be since booked — especially when slot is confirmed, the reservation expires automatically, and the spot becomes available for other Drivers to book.
- After Driver checks in, the billing will start to count. It will calculate by the time window checked in and the actual parking session duration.
- **Payment:** The system must support checkout via payment gateway integration, including **QRIS** payments.
- **Pricing** is calculated by time. The first hour costs **5,000 IDR**, and each subsequent started hour costs **5,000 IDR**.
- **Overnight fee:** If a parking session is classified as overnight (e.g., crosses midnight), charge a flat **20,000 IDR**.
- **Booking fee** is **5,000 IDR** per successful reservation (charged when the reservation is confirmed).
- **Overstay:** There is no overstay penalty; additional time is billed using the same standard hourly rate.

The rate is not decided from the beginning like competitors that lock a fixed price for the whole booking. Instead, it is calculated from the actual parking session time, aligned with the reservation window and the applicable fees/rules. This solution will be set up as a mini app inside a super app (or as a standalone service).

---

## What You Need to Do

Find a solution for the ParkirPintar backend. What you need to prepare are:

### Documentation (inside `README.md`)

A solution diagrams inside `README.md` for this backend solution, which will be implied by:

- High level design Architecture
- Low level design architecture
- ERD document

### Deliverables

Specific configuration for each tool (queuing, load balancer, cloud system) that also comes with your solution — the configuration files that exist on your solution must be committed on the git repository. The microservice(s) codes that you provide to make sure the reservation and billing backend can be running smoothly for booking and charging only. An end-to-end test to make sure that all the business cases above will work smoothly.

---

## Architecture & Technical Requirements *(important)*

- The system must use **gRPC over HTTP/2 or Streaming event** for service-to-service communication.
- All services must be written in **Go**.
- Suggested microservices (you may merge some if you justify it): `gateway`, `search`, `reservation`, `billing`, `payment`, `presence`, `notification`.
- Provide reusable components: **pricing engine** (rules above), **locking mechanism**, **config loader**, and **structured logging/tracing**.
- Use an **idempotency mechanism** for `CreateReservation` and `Checkout/Invoice` operations.
- **Consistency:** prevent double-booking for the same spot and overlapping time windows.
- **Availability:** define how you handle retries, timeouts, circuit breakers, and graceful degradation when non-core services fail.

---

## Testing Requirements

- Unit tests for pricing rules, overlap detection, and idempotency.
- Integration tests for reservation → billing flow.
- End-to-end test scenarios *(at minimum)*:
  - Happy path reservation
  - Double-book prevention
  - User-selected spot contention/queue
  - Reservation expiry (no-show) and spot release
  - Wrong-spot penalty
  - Cancellation policy
  - Extended stay billing (no overstay penalty)
  - Overnight fee
  - Payment checkout (payment gateway/QRIS; success and failure cases)

---

## Submission Guidelines

For submission, please use any free git cloud solution, and push all the codes and design document in 1 project. Do not forget to give your **git URL** before starting this project. All the documents should be in **MD file format**. For description about your project codes will be assigned on `README.md`, alongside with your document design will be assigned also in `README.md`. For any kind of more detailed document, you may setup a word or docs online and place the url underneath your MD document. If those can use a dummy or stub service also using fake data. Please declare all your assumptions in the `README.md` as well.

---

## Integrity Notice

Ensure that the response you submit is written by yourself, and by submitting your response, you acknowledge that it was prepared solely by you and not use any other outside resources to respond to this. You are eligible to use 3rd party libraries, frameworks, and tools but please make sure that you list it on `README.md` and you have a clear justification why you must use it and will be explained when you present it.
