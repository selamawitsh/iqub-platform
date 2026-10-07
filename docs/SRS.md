# Software Requirements Specification: Iqub Platform

| | |
|---|---|
| Version | 0.1 (draft) |
| Date | October 2026 |
| Author | Selamawit Shimeles Derkera |
| Status | Draft, expected to change as milestones are built |

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
| Payout | The pool paid to the round's recipient: contribution amount multiplied by number of members. |
| Minor unit | The smallest currency unit (santim). 100 santim = 1 ETB. |
| JWT | JSON Web Token, used to prove who is making a request. |

### 1.4 Intended audience

The author while building, reviewers and recruiters reading the repository, and any future contributor.

---

## 2. Overall Description

### 2.1 Product perspective

```
        client (curl / Postman / future frontend)
                      |
                      v
              +---------------+
              |    gateway    |   only public component
              +---------------+
               |      |      |
               v      v      v
        +--------+ +--------+ +--------------+
        | member | |  iqub  | | notification |
        |service | |service | |   service    |
        +--------+ +--------+ +--------------+
            |          |             ^   |
            v          v             |   v
        [members]   [iqub db]  events via RabbitMQ  [notifications db]
           db
```

Each service owns its own database. No service reads another service's database. Services communicate through the gateway (synchronous HTTP) and RabbitMQ (asynchronous events).

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
- Single currency: Ethiopian Birr (ETB).
- No real payment or SMS integration.
- All configuration through environment variables.
- Must run within a small AWS budget, with a billing alarm in place.

### 2.5 Assumptions

- A group has between 2 and 50 members.
- One admin per group, and the admin is also a contributor and a payout recipient, as in a traditional iqub.
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
| FR-I1 | A member can create a group with name, contribution amount, frequency (WEEKLY, BIWEEKLY, MONTHLY), and maximum members. The creator becomes the admin and first member. | MUST |
| FR-I2 | A member can list the groups they belong to and view the details of any of them. | MUST |
| FR-I3 | A member can join a group using its invite code while the group is OPEN and not full. | MUST |
| FR-I4 | The admin can start a group that has at least 2 members. The system draws a random payout order, creates one round per member, and sets the group to ACTIVE. | MUST |
| FR-I5 | Members can view the payout schedule: round number, due date, and recipient. | MUST |
| FR-I6 | A member can record their contribution for the current round. The admin can confirm it. | MUST |
| FR-I7 | A member cannot contribute twice in the same round. | MUST |
| FR-I8 | When every member's contribution for the current round is confirmed, the round is marked COMPLETE, the payout is recorded for the recipient, and the next round becomes current. | MUST |
| FR-I9 | When the last round completes, the group is marked COMPLETED. | MUST |
| FR-I10 | Only members of a group can view its data. Only the admin can start the group or confirm contributions. | MUST |
| FR-I11 | The service keeps an append-only ledger of contributions and payouts that members can view. | SHOULD |
| FR-I12 | Contributions not confirmed by the round's due date are flagged overdue and a `contribution.overdue` event is published. | SHOULD |
| FR-I13 | The admin can remove a member from a group before it starts. | COULD |

### 3.3 Notification service

| ID | Requirement | Priority |
|---|---|---|
| FR-N1 | The service consumes the events listed in section 5. | MUST |
| FR-N2 | For each event it creates a notification message from a template and delivers it through a pluggable channel. The first channel writes to the log and the database (simulated SMS). | MUST |
| FR-N3 | A member can list their own notifications. | MUST |
| FR-N4 | Processing the same event twice must not create a duplicate notification. | MUST |
| FR-N5 | Events that fail processing are retried, then moved to a dead-letter queue. | SHOULD |
| FR-N6 | A real SMS provider can be added without changing the event handling code. | COULD |

### 3.4 Gateway

| ID | Requirement | Priority |
|---|---|---|
| FR-G1 | The gateway is the single public entry point and routes requests to the correct service by path. | MUST |
| FR-G2 | The gateway validates the JWT on protected routes, rejects invalid or missing tokens with 401, and forwards the member's identity to services in an `X-User-Id` header. | MUST |
| FR-G3 | Registration, login, and health endpoints are public. | MUST |
| FR-G4 | The gateway generates an `X-Request-Id` for each request and passes it to services and logs. | SHOULD |
| FR-G5 | The gateway rate-limits requests per client IP. | SHOULD |
| FR-G6 | Cross-origin (CORS) settings are configurable. | COULD |

---

## 4. Business Rules

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

---

## 5. Domain Events

| Event | Published by | Consumed by | Key fields |
|---|---|---|---|
| `member.registered` | member-service | notification-service | event_id, member_id, full_name, phone |
| `group.started` | iqub-service | notification-service | event_id, group_id, member_ids, payout_order |
| `contribution.confirmed` | iqub-service | notification-service | event_id, group_id, round_number, member_id, amount |
| `round.completed` | iqub-service | notification-service | event_id, group_id, round_number, recipient_id, payout_amount |
| `contribution.overdue` | iqub-service | notification-service | event_id, group_id, round_number, member_id |

Every event carries a unique `event_id` so that consumers can detect duplicates.

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

---

## 7. Data Model (high level)

**member-service:** Member (id, full_name, phone, password_hash, created_at).

**iqub-service:**
- Group (id, name, contribution_amount, frequency, max_members, invite_code, status, admin_member_id, created_at, started_at)
- GroupMember (group_id, member_id, payout_position, joined_at)
- Round (id, group_id, number, due_date, status, recipient_member_id)
- Contribution (id, round_id, member_id, amount, status, recorded_at, confirmed_at)
- LedgerEntry (id, group_id, round_id, member_id, type, amount, created_at)

**notification-service:** Notification (id, member_id, event_id, type, message, created_at), with a unique constraint on event_id.

---

## 8. Non-Functional Requirements

### 8.1 Security

| ID | Requirement |
|---|---|
| NFR-S1 | Passwords are hashed with bcrypt and never logged or returned. |
| NFR-S2 | JWTs are signed and have an expiry. The signing secret comes from the environment, never from the repository. |
| NFR-S3 | Only the gateway is reachable from the internet. Other services accept traffic only from the internal network. |
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
| NFR-DOC2 | Important design decisions are recorded as short architecture decision records (ADRs). |
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

---


