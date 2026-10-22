# Software Requirements Specification: Iqub Platform

| | |
|---|---|
| Version | 0.2 (draft) |
| Date | October 2026 |
| Author | Selamawit Shimeles Derkera |
| Status | Draft, expected to change as milestones are built |

### Change log

| Version | Summary |
|---|---|
| 0.1 | Initial draft. |
| 0.2 | Resolved design gaps found during system design: added state definitions (4.2), due-date rule, payout member-count rule, overdue-once rule, server-set contribution amount, local contact copy in notification-service (FR-N7), per-recipient notification uniqueness (FR-N4), event envelope and RabbitMQ topology (section 5), API conventions (section 6), architecture decisions (section 10), milestones with definitions of done (section 11), and open questions (section 12). Clarified that services never call each other directly. |

---

## 1. Introduction

### 1.1 Purpose

This document defines what the Iqub Platform must do and the qualities it must have. It is the reference for design decisions, testing, and deciding when a milestone is finished.

### 1.2 Scope

The Iqub Platform is a backend system, built as Go microservices, that manages rotating savings groups (iqubs). Members join a group, contribute a fixed amount every period, and each period one member receives the full pool until everyone has received it once.

The system is API-only. It does not move real money and does not send real SMS messages. Payments and notifications are simulated, so the focus stays on architecture, correctness, and deployment.

### 1.3 Definitions

| Term | Meaning |
|---|---|
| Iqub | A rotating savings group. Also called a group in this document. |
| Member | A registered person who can belong to groups. |
| Group admin | The member who created a group and manages it. |
| Contribution | The fixed amount each member pays in each round. |
| Round | One period of the cycle. Exactly one member is the recipient of each round. |
| Payout | The pool paid to the round's recipient: contribution amount multiplied by the number of members when the group started. |
| Minor unit | The smallest currency unit (santim). 100 santim = 1 ETB. |
| JWT | JSON Web Token, used to prove who is making a request. |
| Event | A message announcing that something happened (for example `round.completed`), published to RabbitMQ. |
| Dead-letter queue (DLQ) | A queue that holds messages that failed processing after all retries. |
| Ledger | An append-only record of money movements (contributions and payouts). |
| Idempotent | Safe to process more than once: the second time changes nothing. |

### 1.4 Intended audience

The author while building, reviewers and recruiters reading the repository, and any future contributor.

---

## 2. Overall Description

### 2.1 Product perspective

```
          client (curl / Postman / future frontend)
                          |  public HTTPS
                          v
                  +---------------+
                  |    gateway    |   only public component
                  +---------------+
                   |      |      |
                   v      v      v
         +-----------+ +-----------+ +--------------+
         |  member   | |   iqub    | | notification |
         |  service  | |  service  | |   service    |
         | (own db)  | | (own db)  | |   (own db)   |
         +-----------+ +-----------+ +--------------+
               |             |              ^
               v  publish    v  publish     | consume
         +----------------------------------------+
         |                RabbitMQ                |
         +----------------------------------------+
```

Each service owns its own database. No service reads another service's database. The gateway is the only component that talks to services synchronously. Services never call each other directly: they share identity through the gateway's `X-User-Id` header and share facts through RabbitMQ events.

### 2.2 User classes

| User | Description |
|---|---|
| Member | Registers, logs in, joins groups, records contributions, views notifications. |
| Group admin | A member who also created a group. Starts the group and confirms contributions. |

There is no platform-wide administrator in version 1.

### 2.3 Operating environment

Linux containers (Docker). Local development with Docker Compose. Production on AWS (EC2 first, container services later).

### 2.4 Constraints

- Language: Go 1.22 or higher for all services.
- Database: PostgreSQL, one database per service.
- Messaging: RabbitMQ.
- Single currency: Ethiopian Birr (ETB).
- No real payment or SMS integration.
- All configuration through environment variables.
- Must run within a small AWS budget, with a billing alarm in place.

### 2.5 Assumptions

- A group has between 2 and 50 members.
- One admin per group, and the admin is also a contributor and a payout recipient, as in a traditional iqub. The admin confirms their own contribution like everyone else's.
- Every member of a group, including the round's recipient, contributes in every round. This is why the payout equals contribution amount times number of members.
- The author builds and operates the system alone, so simplicity is preferred over completeness.

---

## 3. Functional Requirements

Priority: **MUST** (needed for the milestone to be done), **SHOULD** (important, do if time allows), **COULD** (nice to have).

### 3.1 Member service

| ID | Requirement | Priority |
|---|---|---|
| FR-M1 | A person can register with full name, phone number, and password. Phone numbers are unique. | MUST |
| FR-M2 | A member can log in with phone number and password and receives a signed JWT access token. | MUST |
| FR-M3 | A member can view their own profile. | MUST |
| FR-M4 | The service publishes a `member.registered` event after a successful registration. | MUST |
| FR-M5 | A member can update their full name and password. | SHOULD |
| FR-M6 | A member can obtain a new access token using a refresh token. | COULD |

### 3.2 Iqub service

| ID | Requirement | Priority |
|---|---|---|
| FR-I1 | A member can create a group with name, contribution amount, frequency (WEEKLY, BIWEEKLY, MONTHLY), and maximum members. The creator becomes the admin and first member. The group starts OPEN with a unique invite code. | MUST |
| FR-I2 | A member can list the groups they belong to and view the details of any of them. | MUST |
| FR-I3 | A member can join a group using its invite code while the group is OPEN and not full. | MUST |
| FR-I4 | The admin can start a group that has at least 2 members. The system draws a random payout order, creates one round per member (the member count at that moment, see BR-9), sets round 1 as current, and sets the group to ACTIVE. | MUST |
| FR-I5 | Members can view the payout schedule: round number, due date (BR-10), and recipient. | MUST |
| FR-I6 | A member can record their contribution for the current round. The amount is set by the system from the group's contribution amount, never taken from the request (BR-12). The admin can confirm a recorded contribution. | MUST |
| FR-I7 | A member cannot contribute twice in the same round. | MUST |
| FR-I8 | When every member's contribution for the current round is confirmed, the round is marked COMPLETE, the payout is recorded for the recipient in the ledger, and the next round becomes current. All of this happens in one database transaction (NFR-R1). | MUST |
| FR-I9 | When the last round completes, the group is marked COMPLETED. | MUST |
| FR-I10 | Only members of a group can view its data. Only the admin can start the group or confirm contributions. | MUST |
| FR-I11 | The service keeps an append-only ledger of contributions and payouts that members can view. | SHOULD |
| FR-I12 | Contributions not confirmed by the round's due date are flagged overdue and a `contribution.overdue` event is published once per contribution (BR-11). | SHOULD |
| FR-I13 | The admin can remove a member from a group before it starts. | COULD |

### 3.3 Notification service

| ID | Requirement | Priority |
|---|---|---|
| FR-N1 | The service consumes the events listed in section 5. | MUST |
| FR-N2 | For each event it creates a notification message from a template for each affected member and delivers it through a pluggable channel. The first channel writes to the log and the database (simulated SMS). | MUST |
| FR-N3 | A member can list their own notifications. | MUST |
| FR-N4 | Processing the same event twice must not create a duplicate notification. Uniqueness is per event and per recipient, because one event (for example `group.started`) notifies many members. | MUST |
| FR-N5 | Events that fail processing are retried, then moved to a dead-letter queue. | SHOULD |
| FR-N6 | A real SMS provider can be added without changing the event handling code. | COULD |
| FR-N7 | The service keeps a local copy of each member's name and phone, built from `member.registered` events, so it can address messages without calling the member service. | MUST |

### 3.4 Gateway

| ID | Requirement | Priority |
|---|---|---|
| FR-G1 | The gateway is the single public entry point and routes requests to the correct service by path. | MUST |
| FR-G2 | The gateway validates the JWT on protected routes, rejects invalid or missing tokens with 401, and forwards the member's identity to services in an `X-User-Id` header. It removes any `X-User-Id` header sent by the client before setting its own. | MUST |
| FR-G3 | Registration, login, and health endpoints are public. | MUST |
| FR-G4 | The gateway generates an `X-Request-Id` for each request and passes it to services and logs. | SHOULD |
| FR-G5 | The gateway rate-limits requests per client IP. | SHOULD |
| FR-G6 | Cross-origin (CORS) settings are configurable. | COULD |

---

## 4. Business Rules

### 4.1 Rules

| ID | Rule |
|---|---|
| BR-1 | All money amounts are stored and processed as 64-bit integers in minor units. Floating point is never used for money. |
| BR-2 | Payout amount = contribution amount x number of members in the group. |
| BR-3 | Group status moves only in this order: OPEN, ACTIVE, COMPLETED. It never moves backwards. |
| BR-4 | A member can belong to a given group only once. |
| BR-5 | The payout order is drawn when the group starts and cannot be changed afterwards. |
| BR-6 | A round completes only when all members' contributions for it are confirmed. |
| BR-7 | Contributions and ledger entries are never deleted. Corrections are made with a new reversing entry. |
| BR-8 | The iqub service identifies members only by their ID. It never reads the member database. |
| BR-9 | The number of rounds and the payout amount are based on the number of members at the moment the group starts, not on the group's maximum size. |
| BR-10 | Round N is due at the group's start date plus N periods. A period is 7 days (WEEKLY), 14 days (BIWEEKLY), or one calendar month (MONTHLY). |
| BR-11 | A contribution is flagged overdue at most once, and its `contribution.overdue` event is published at most once. |
| BR-12 | The contribution amount is always the group's configured amount. A client cannot choose or change it. |

### 4.2 Status model

| Entity | Statuses | Allowed transitions |
|---|---|---|
| Group | OPEN, ACTIVE, COMPLETED | OPEN to ACTIVE (admin starts, at least 2 members). ACTIVE to COMPLETED (last round completes). No other moves. |
| Round | PENDING, CURRENT, COMPLETE | PENDING to CURRENT (previous round completes, or group starts for round 1). CURRENT to COMPLETE (all contributions confirmed). Exactly one round is CURRENT while the group is ACTIVE. |
| Contribution | RECORDED, CONFIRMED | RECORDED to CONFIRMED (admin confirms). Overdue is a flag on a RECORDED or missing contribution, not a status. |

Actions outside the table are rejected with a 409 (see section 6).

---

## 5. Domain Events

### 5.1 Event catalogue

| Event | Published by | Consumed by | Key fields in `data` |
|---|---|---|---|
| `member.registered` | member-service | notification-service | member_id, full_name, phone |
| `group.started` | iqub-service | notification-service | group_id, member_ids, payout_order |
| `contribution.confirmed` | iqub-service | notification-service | group_id, round_number, member_id, amount |
| `round.completed` | iqub-service | notification-service | group_id, round_number, recipient_id, payout_amount |
| `contribution.overdue` | iqub-service | notification-service | group_id, round_number, member_id |

### 5.2 Event envelope

Every event is a JSON message with the same outer shape:

```json
{
  "event_id": "uuid",
  "event_type": "round.completed",
  "occurred_at": "2026-10-22T10:00:00Z",
  "version": 1,
  "data": { }
}
```

The unique `event_id` lets consumers detect duplicates. Amounts in `data` are integers in minor units.

### 5.3 RabbitMQ topology and delivery

- One topic exchange named `iqub.events`. The routing key is the event type.
- The notification service has its own queue, bound to the routing keys it consumes.
- Failed messages are retried a limited number of times, then moved to a dead-letter queue (FR-N5).
- Delivery is at-least-once. Consumers must therefore be idempotent (FR-N4, NFR-R5).
- Publishers publish after the database transaction commits. If publishing fails, the failure is logged and counted. This is a known limitation of version 1 (see ADR-005).

---

## 6. API Summary

All routes are reached through the gateway. "Auth" means a valid JWT is required.

| Method | Path | Service | Auth |
|---|---|---|---|
| POST | /api/auth/register | member | No |
| POST | /api/auth/login | member | No |
| GET | /api/members/me | member | Yes |
| PATCH | /api/members/me | member | Yes |
| POST | /api/iqubs | iqub | Yes |
| GET | /api/iqubs | iqub | Yes |
| GET | /api/iqubs/{id} | iqub | Yes |
| POST | /api/iqubs/join | iqub | Yes |
| POST | /api/iqubs/{id}/start | iqub | Yes (admin) |
| GET | /api/iqubs/{id}/schedule | iqub | Yes |
| POST | /api/iqubs/{id}/contributions | iqub | Yes |
| POST | /api/iqubs/{id}/contributions/{cid}/confirm | iqub | Yes (admin) |
| GET | /api/iqubs/{id}/ledger | iqub | Yes |
| GET | /api/notifications | notification | Yes |
| GET | /health | every service | No |
| GET | /ready | every service | No |

Each service publishes an OpenAPI specification in its own folder.

### 6.1 API conventions

- Requests and responses are JSON. Money amounts are integers in minor units (santim).
- Errors use one shape: `{"error": {"code": "GROUP_FULL", "message": "..."}}`. The `code` is stable and machine-readable.
- Status codes: 400 malformed or invalid input, 401 missing or invalid token, 403 authenticated but not allowed (for example a non-admin starting a group), 404 not found, 409 the request conflicts with current state (group full, group not OPEN, duplicate contribution, round not current).
- A non-member asking for a group they do not belong to receives 404, so the existence of groups is not leaked (FR-I10).
- Services read the caller's identity only from `X-User-Id`. They never parse the JWT themselves.

---

## 7. Data Model (high level)

**member-service:** Member (id, full_name, phone, password_hash, created_at). Phone is unique.

**iqub-service:**
- Group (id, name, contribution_amount, frequency, max_members, invite_code, status, admin_member_id, created_at, started_at). Invite code is unique.
- GroupMember (group_id, member_id, payout_position, joined_at). Primary key (group_id, member_id) enforces BR-4. Unique (group_id, payout_position).
- Round (id, group_id, number, due_date, status, recipient_member_id). Unique (group_id, number).
- Contribution (id, round_id, member_id, amount, status, recorded_at, confirmed_at, overdue_flagged_at). Unique (round_id, member_id) enforces FR-I7.
- LedgerEntry (id, group_id, round_id, member_id, type, amount, created_at). Type is CONTRIBUTION, PAYOUT, or REVERSAL. Rows are never updated or deleted.

**notification-service:**
- Notification (id, member_id, event_id, type, message, created_at). Unique (event_id, member_id) enforces FR-N4.
- MemberContact (member_id, full_name, phone). Filled from `member.registered` (FR-N7).

Cross-service references (such as `member_id` in the iqub database) are plain IDs with no foreign key (BR-8).

---

## 8. Non-Functional Requirements

### 8.1 Security

| ID | Requirement |
|---|---|
| NFR-S1 | Passwords are hashed with bcrypt and never logged or returned. |
| NFR-S2 | JWTs are signed and have an expiry. The signing secret comes from the environment, never from the repository. Version 1 uses a shared secret (HS256); asymmetric keys are a later option (ADR-006). |
| NFR-S3 | Only the gateway is reachable from the internet. Other services accept traffic only from the internal network. This is required because services trust the `X-User-Id` header (ADR-003). |
| NFR-S4 | All input is validated. All database queries use parameters, never string concatenation. |
| NFR-S5 | The CI pipeline scans Go dependencies (govulncheck) and container images (Trivy) and fails on high-severity findings. |
| NFR-S6 | AWS credentials follow least privilege. No credentials are stored in code or images. |

### 8.2 Reliability and data integrity

| ID | Requirement |
|---|---|
| NFR-R1 | Round completion and payout recording happen in one database transaction. |
| NFR-R2 | Database constraints enforce uniqueness rules (BR-4, FR-I7, FR-N4), not only application code. |
| NFR-R3 | Each service shuts down gracefully on SIGTERM and finishes in-flight requests. |
| NFR-R4 | Each service exposes `/health` (alive) and `/ready` (dependencies reachable). |
| NFR-R5 | Consumers reconnect automatically after a RabbitMQ restart and handle duplicate messages safely. |
| NFR-R6 | If notification-service is down, the other services keep working and events are processed when it returns. |
| NFR-R7 | The overdue job is safe to run repeatedly and from restarts: it never publishes the same overdue event twice (BR-11). |

### 8.3 Performance

| ID | Requirement |
|---|---|
| NFR-P1 | Read endpoints respond in under 300 ms at the 95th percentile with 50 concurrent users on a single small instance. This is verified with a load test, not assumed. |
| NFR-P2 | Each service container image is smaller than 30 MB. |

### 8.4 Observability

| ID | Requirement |
|---|---|
| NFR-O1 | Logs are structured JSON and include the request ID. |
| NFR-O2 | Each service exposes basic metrics (request count, latency, errors). |

### 8.5 Maintainability and quality

| ID | Requirement |
|---|---|
| NFR-M1 | Each service has its own `go.mod`, Dockerfile, and CI path filter, so it can be built and deployed alone. |
| NFR-M2 | Code passes gofmt, go vet, and a linter in CI. |
| NFR-M3 | Business logic (round completion, payout calculation, status transitions) has at least 60% unit test coverage. |
| NFR-M4 | Configuration is only through environment variables. |
| NFR-M5 | Commits follow the Conventional Commits format. |
| NFR-M6 | Each service follows onion architecture: the domain layer imports nothing from outer layers, and outside concerns (storage, events, time, randomness) are reached through interfaces defined inward. |

### 8.6 Deployability

| ID | Requirement |
|---|---|
| NFR-D1 | The whole system starts locally with one `docker compose up`. |
| NFR-D2 | Every push to `main` builds, tests, scans, publishes images to ECR, and deploys, in under 10 minutes. |
| NFR-D3 | A pull request runs build and tests only, with no deployment. |
| NFR-D4 | Deployment can be rolled back by redeploying a previous image tag. |

### 8.7 Cost

| ID | Requirement |
|---|---|
| NFR-C1 | A billing alarm is configured before any AWS resource is created. |
| NFR-C2 | The system runs on a single small instance, and all resources can be stopped or deleted when not in use. |

### 8.8 Documentation

| ID | Requirement |
|---|---|
| NFR-DOC1 | The README contains an architecture diagram, local run instructions, and the live URL. |
| NFR-DOC2 | Important design decisions are recorded as short architecture decision records (ADRs). Section 10 is the starting set. |
| NFR-DOC3 | A short runbook explains how to check health, read logs, and roll back. |

---

## 9. Out of Scope (version 1)

- Real payment integration (for example Telebirr or bank APIs)
- Real SMS or email delivery
- Identity verification (KYC)
- Web or mobile frontend
- Multiple currencies
- Late penalties, interest, or dispute handling
- A platform-wide administrator role
- Transactional outbox for guaranteed event publishing (planned improvement, ADR-005)
- Asymmetric JWT signing and key rotation (planned improvement, ADR-006)

---

## 10. Architecture Decisions

These are the starting ADRs. Each should later get its own short file with context, decision, and consequences.

| ID | Decision | Why | Trade-off accepted |
|---|---|---|---|
| ADR-001 | One database per service. | Services can change their schema independently. | No cross-service joins. Data needed elsewhere travels as events. |
| ADR-002 | No direct service-to-service calls. Identity comes from the gateway header, shared facts come from events. | Fewer failure chains: one slow service cannot stall another. | Services may hold small copies of data (FR-N7) that can be briefly stale. |
| ADR-003 | Services trust `X-User-Id` from the gateway; the network is the trust boundary. | Auth logic lives in one place. | If a service is reachable from outside, identity can be forged. NFR-S3 is therefore mandatory. |
| ADR-004 | RabbitMQ topic exchange, at-least-once delivery, idempotent consumers. | Simple, well understood, enough for this scale. | Duplicates are possible, so every consumer must deduplicate. |
| ADR-005 | Publish events after the database commit; no outbox in version 1. | Keeps version 1 simple. | If the broker is down at publish time, an event can be lost. Mitigation: log and count failures, add an outbox later. |
| ADR-006 | HS256 shared-secret JWTs in version 1. | Simplest to operate alone. | The gateway holds the signing secret. Move to RS256 so the gateway holds only a public key. |
| ADR-007 | Onion architecture in each service, with interfaces for repositories, transactions, event publishing, clock, and randomness. | Domain rules can be tested without a database, real time, or real randomness. Storage can be swapped without touching business rules. | More files and indirection than a flat layout. |
| ADR-008 | Money is an int64 in minor units. | Exact arithmetic. | Conversion to ETB is only done when displaying. |
| ADR-009 | Database constraints enforce invariants, in addition to application checks. | Correct even when two requests race. | Constraint violations must be mapped to clean 409 responses. |

---

## 11. Milestones and Definition of Done

| Milestone | Scope | Done when |
|---|---|---|
| M1: Iqub core in memory | FR-I1 to FR-I10, BR-1 to BR-12, in-memory repositories | Domain and application tests pass with at least 60% coverage on business logic. A full cycle (create, join, start, record, confirm, group COMPLETED) works through the HTTP API with curl. |
| M2: PostgreSQL | NFR-R1, NFR-R2, ADR-009 | Migrations create the schema with all unique constraints. Integration tests prove a duplicate contribution returns 409 and a failed completion leaves no partial data. |
| M3: Member service | FR-M1 to FR-M4, NFR-S1, NFR-S2 | Register and login work. JWT is signed with expiry. `member.registered` is published. |
| M4: Gateway | FR-G1 to FR-G4 | Protected routes return 401 without a valid token. Services receive a trusted `X-User-Id` and `X-Request-Id`. Client-supplied `X-User-Id` is ignored. |
| M5: Events and notifications | FR-N1 to FR-N4, FR-N7, section 5 | A confirmed contribution produces a notification end to end. Replaying an event creates no duplicate. Stopping notification-service and restarting it processes the waiting events. |
| M6: Containers | NFR-D1, NFR-R3, NFR-R4, NFR-M1, NFR-P2 | `docker compose up` starts everything. Health and ready endpoints work. Images are under 30 MB. Only the gateway port is published. |
| M7: CI/CD and AWS | NFR-D2 to NFR-D4, NFR-S5, NFR-S6, NFR-C1, NFR-C2, NFR-DOC1 to NFR-DOC3 | Billing alarm exists first. A push to `main` deploys in under 10 minutes. A rollback to a previous tag is demonstrated. |
| M8: Hardening | SHOULD items: FR-I11, FR-I12, FR-N5, FR-G5, FR-M5, NFR-O1, NFR-O2, NFR-P1 | Overdue events publish once. A poison message lands in the DLQ. The load test meets the target. |

---
