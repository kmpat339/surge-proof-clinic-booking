# Surge-Proof Clinic Appointment Booking

A distributed clinic appointment booking service that stays up during a sign-up rush, never gives the same slot to two people, and doesn't reward the fastest connection.

CS6650 Building Scalable Distributed Systems (Northeastern, Fall 2026) final project.

## Why we're building it

When COVID vaccine eligibility opened in early 2021, booking sites failed across the US: Massachusetts' VaxFinder crashed when about 1 million residents became eligible at once, Minnesota's site peaked at 10,000 hits/sec and had to close to new registrants, and the CDC's $44M VAMS system was reported to crash and randomly cancel appointments. Slots also went to whoever refreshed fastest, and seniors lost out. This project builds and measures a booking system designed for exactly that kind of rush.

## Design (planned)

- **Booking service:** stateless Go servers on AWS ECS behind a load balancer, with autoscaling
- **Slot store interface:** hold, confirm, release, list open, with two implementations:
  - **DynamoDB store:** conditional writes ("book only if still open"), partitioned by clinic
  - **Raft store:** one 3-node Raft group per region (hashicorp/raft); a booking commits once a majority agrees
- **Holds:** a picked slot is reserved for 5 minutes; one active hold per patient
- **Idempotency keys:** a retried request books only once
- **Cache:** Redis cache-aside for slot lists and clinic info
- **Waiting room:** SQS-based queue that admits users in random order during a rush
- **API:** loosely follows HL7 FHIR Slot/Appointment; all patient data is synthetic

## Experiments

| # | Experiment | Tradeoff | Owner |
|---|---|---|---|
| E1 | Scale-out: 1 → 2 → 4 → 8 servers | Where does throughput stop scaling? | Divyam |
| E2 | Double-booking under contention | Correctness vs. throughput | Khush |
| E3 | Cache freshness | DB load vs. stale slot lists | Milena |
| E4 | Flash crowd: autoscaling vs. waiting room | Survival and fairness vs. added wait | Milena |
| E5 | Failure mid-rush | Availability vs. correctness | Divyam |
| E6 | DynamoDB vs. Raft store | Control and depth vs. complexity and coordination cost | Nishit |

## Team

| Member | Owns |
|---|---|
| Khush Patel | Booking API, slot-store interface, DynamoDB store, holds, idempotency, double-booking check |
| Divyam Patel | Terraform infrastructure, Locust test suite, fault injection, dashboards |
| Milena Perez-Gerus | Redis cache, SQS waiting room |
| Nishit Chaudhary | Raft-replicated slot store |

## Repo layout (planned, may change)

| Folder | What goes there |
|---|---|
| `src/` | Go booking service (API, slot-store interface, DynamoDB store) |
| `raft/` | Raft slot store |
| `terraform/` | AWS infrastructure (ECS, ALB, DynamoDB, Redis, SQS) |
| `locust/` | Load tests (ramp, spike, flash crowd) |
| `experiments/` | Scripts and results for E1–E6 |
| `docs/` | Proposal and reports |

## Status

Proposal and team charter submitted (Nov 23). Next: the booking core on DynamoDB.

- [`docs/proposal.pdf`](docs/proposal.pdf): initial project proposal
- [`docs/hw9-proposal-and-team-charter.pdf`](docs/hw9-proposal-and-team-charter.pdf): full proposal and team charter, with each experiment's methods and preliminary results
- [`src/slotstore`](src/slotstore): shared slot-store interface and the in-memory double-booking race test (E2 preliminary result)
