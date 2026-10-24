package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/selamawitsh/iqub-platform/services/iqub-service/internal/domain/model"
	"github.com/selamawitsh/iqub-platform/services/iqub-service/internal/domain/repository"
)

// IqubRepository stores everything in PostgreSQL. It implements the same
// contract as the memory repository, so no other layer changes.
//
// It stores what the use cases decided. The business rules (who may confirm,
// when a round is complete, how big a payout is) stay in the domain. Where
// two requests could race, the repository locks rows and checks the stored
// state still matches what the use case saw, and returns model.ErrConflict
// if it does not.
type IqubRepository struct {
	db *sql.DB
}

// Compile-time check that this type satisfies the domain's contract.
var _ repository.IqubRepository = (*IqubRepository)(nil)

func NewIqubRepository(db *sql.DB) *IqubRepository {
	return &IqubRepository{db: db}
}

// ---------------------------------------------------------------------
// Groups
// ---------------------------------------------------------------------

// Save inserts a new group, or adds new members to an existing one.
//
// For an existing group it never changes status or other fields, and it
// never removes members. Status moves only through SaveStart and
// SaveConfirmation, so a request holding an old copy of the group (say, a
// join that loaded it just before the admin started it) cannot move the
// group backwards or erase another member's join. The group row is locked
// while members are added, so simultaneous joins cannot exceed the maximum.
func (r *IqubRepository) Save(ctx context.Context, iqub model.Iqub) error {
	return r.inTx(ctx, func(tx *sql.Tx) error {
		var (
			storedStatus string
			maxMembers   int
			isNew        bool
		)
		err := tx.QueryRowContext(ctx,
			`SELECT status, max_members FROM iqubs WHERE id = $1 FOR UPDATE`, iqub.ID,
		).Scan(&storedStatus, &maxMembers)
		switch {
		case errors.Is(err, sql.ErrNoRows):
			isNew = true
			maxMembers = iqub.MaxMembers
			_, err := tx.ExecContext(ctx, `
				INSERT INTO iqubs
					(id, name, contribution_amount, frequency, max_members, status,
					 admin_id, invite_code, created_at, started_at)
				VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
				iqub.ID, iqub.Name, iqub.ContributionAmount, string(iqub.Frequency),
				iqub.MaxMembers, string(iqub.Status), iqub.AdminID, iqub.InviteCode,
				iqub.CreatedAt, nullTime(iqub.StartedAt))
			if err != nil {
				return translate(err)
			}
		case err != nil:
			return err
		}

		existing, err := memberIDs(ctx, tx, iqub.ID)
		if err != nil {
			return err
		}
		have := make(map[string]bool, len(existing))
		for _, id := range existing {
			have[id] = true
		}
		var added []string
		for _, id := range iqub.MemberIDs {
			if !have[id] {
				added = append(added, id)
			}
		}
		if len(added) == 0 {
			return nil
		}

		if !isNew && storedStatus != string(model.StatusOpen) {
			return fmt.Errorf("%w: group is not open for joining", model.ErrConflict)
		}
		if len(existing)+len(added) > maxMembers {
			return fmt.Errorf("%w: group is full", model.ErrConflict)
		}
		for k, id := range added {
			_, err := tx.ExecContext(ctx, `
				INSERT INTO iqub_members(iqub_id, member_id, join_order)
				VALUES ($1, $2, $3)`, iqub.ID, id, len(existing)+k+1)
			if err != nil {
				return translate(err)
			}
		}
		return nil
	})
}

func (r *IqubRepository) FindByID(ctx context.Context, id string) (model.Iqub, error) {
	return r.findOne(ctx, `WHERE i.id = $1`, id)
}

func (r *IqubRepository) FindByInviteCode(ctx context.Context, code string) (model.Iqub, error) {
	return r.findOne(ctx, `WHERE i.invite_code = $1`, code)
}

// ListByMember never returns nil, so JSON output is [] and not null.
func (r *IqubRepository) ListByMember(ctx context.Context, memberID string) ([]model.Iqub, error) {
	return r.queryIqubs(ctx,
		`WHERE i.id IN (SELECT iqub_id FROM iqub_members WHERE member_id = $1)`, memberID)
}

// SaveStart marks the group ACTIVE and stores its rounds as one unit.
// It only works on a group that is still OPEN, so a group cannot be
// started twice.
func (r *IqubRepository) SaveStart(ctx context.Context, iqub model.Iqub, rounds []model.Round) error {
	return r.inTx(ctx, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx, `
			UPDATE iqubs SET status = $2, started_at = $3
			WHERE id = $1 AND status = 'OPEN'`,
			iqub.ID, string(iqub.Status), nullTime(iqub.StartedAt))
		if err != nil {
			return translate(err)
		}
		if n, _ := res.RowsAffected(); n == 0 {
			var one int
			err := tx.QueryRowContext(ctx, `SELECT 1 FROM iqubs WHERE id = $1`, iqub.ID).Scan(&one)
			if errors.Is(err, sql.ErrNoRows) {
				return model.ErrNotFound
			}
			if err != nil {
				return err
			}
			return fmt.Errorf("%w: group has already started", model.ErrConflict)
		}
		for _, rd := range rounds {
			_, err := tx.ExecContext(ctx, `
				INSERT INTO rounds(id, iqub_id, number, due_date, status, recipient_id)
				VALUES ($1,$2,$3,$4,$5,$6)`,
				rd.ID, rd.IqubID, rd.Number, rd.DueDate, string(rd.Status), rd.RecipientID)
			if err != nil {
				return translate(err)
			}
		}
		return nil
	})
}

// ---------------------------------------------------------------------
// Rounds
// ---------------------------------------------------------------------

func (r *IqubRepository) ListRounds(ctx context.Context, iqubID string) ([]model.Round, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, iqub_id, number, due_date, status, recipient_id
		FROM rounds WHERE iqub_id = $1 ORDER BY number`, iqubID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]model.Round, 0)
	for rows.Next() {
		var (
			rd     model.Round
			status string
		)
		if err := rows.Scan(&rd.ID, &rd.IqubID, &rd.Number, &rd.DueDate, &status, &rd.RecipientID); err != nil {
			return nil, err
		}
		rd.Status = model.RoundStatus(status)
		rd.DueDate = rd.DueDate.UTC()
		out = append(out, rd)
	}
	return out, rows.Err()
}

// ---------------------------------------------------------------------
// Contributions
// ---------------------------------------------------------------------

// SaveContribution inserts or updates a contribution. The database rule
// UNIQUE (round_id, member_id) rejects a second contribution from the same
// member in the same round, even if two requests arrive at the same moment.
func (r *IqubRepository) SaveContribution(ctx context.Context, c model.Contribution) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO contributions
			(id, iqub_id, round_id, member_id, amount, status, recorded_at, confirmed_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		ON CONFLICT (id) DO UPDATE
		SET status = EXCLUDED.status, confirmed_at = EXCLUDED.confirmed_at`,
		c.ID, c.IqubID, c.RoundID, c.MemberID, c.Amount,
		string(c.Status), c.RecordedAt, nullTime(c.ConfirmedAt))
	return translate(err)
}

func (r *IqubRepository) FindContribution(ctx context.Context, id string) (model.Contribution, error) {
	list, err := r.queryContributions(ctx, `WHERE id = $1`, id)
	if err != nil {
		return model.Contribution{}, err
	}
	if len(list) == 0 {
		return model.Contribution{}, model.ErrNotFound
	}
	return list[0], nil
}

func (r *IqubRepository) ListContributions(ctx context.Context, roundID string) ([]model.Contribution, error) {
	return r.queryContributions(ctx, `WHERE round_id = $1 ORDER BY recorded_at, id`, roundID)
}

// SaveConfirmation saves a confirmation and all its effects in one database
// transaction: everything is saved or nothing is (SRS NFR-R1).
//
// The use case decided whether this confirmation completes the round. Here,
// under a lock on the group, the repository checks that decision against
// the stored state. If another confirmation committed in between, so that
// the decision is out of date, nothing is saved and the result is
// model.ErrConflict: the caller retries and decides again with fresh data.
// That makes it impossible to leave a fully confirmed round open, or to pay
// the same round out twice.
func (r *IqubRepository) SaveConfirmation(ctx context.Context, res repository.ConfirmationResult) error {
	return r.inTx(ctx, func(tx *sql.Tx) error {
		c := res.Contribution

		// Confirmations in one group now run one at a time.
		var groupStatus string
		err := tx.QueryRowContext(ctx,
			`SELECT status FROM iqubs WHERE id = $1 FOR UPDATE`, c.IqubID,
		).Scan(&groupStatus)
		if errors.Is(err, sql.ErrNoRows) {
			return model.ErrNotFound
		}
		if err != nil {
			return err
		}
		if groupStatus != string(model.StatusActive) {
			return fmt.Errorf("%w: group is not active", model.ErrConflict)
		}

		// The contribution must still be RECORDED and belong to the current round.
		if err := expectOneRow(ctx, tx, "contribution can no longer be confirmed", `
			UPDATE contributions SET status = $2, confirmed_at = $3
			WHERE id = $1 AND status = 'RECORDED'
			  AND round_id IN (SELECT id FROM rounds WHERE iqub_id = $4 AND status = 'CURRENT')`,
			c.ID, string(c.Status), nullTime(c.ConfirmedAt), c.IqubID); err != nil {
			return err
		}

		// With this confirmation counted, is the round really complete?
		var members, confirmedCount int
		err = tx.QueryRowContext(ctx, `
			SELECT (SELECT COUNT(*) FROM iqub_members WHERE iqub_id = $1),
			       (SELECT COUNT(*) FROM contributions WHERE round_id = $2 AND status = 'CONFIRMED')`,
			c.IqubID, c.RoundID).Scan(&members, &confirmedCount)
		if err != nil {
			return err
		}
		complete := members > 0 && confirmedCount == members
		if complete != (res.CompletedRound != nil) {
			return fmt.Errorf("%w: the round changed while this request was running, please retry", model.ErrConflict)
		}

		if rd := res.CompletedRound; rd != nil {
			if err := expectOneRow(ctx, tx, "round is no longer current",
				`UPDATE rounds SET status = $2 WHERE id = $1 AND status = 'CURRENT'`,
				rd.ID, string(rd.Status)); err != nil {
				return err
			}
		}
		if rd := res.NextRound; rd != nil {
			if err := expectOneRow(ctx, tx, "next round is not pending",
				`UPDATE rounds SET status = $2 WHERE id = $1 AND status = 'PENDING'`,
				rd.ID, string(rd.Status)); err != nil {
				return err
			}
		}
		if g := res.Iqub; g != nil {
			if err := expectOneRow(ctx, tx, "group is no longer active",
				`UPDATE iqubs SET status = $2 WHERE id = $1 AND status = 'ACTIVE'`,
				g.ID, string(g.Status)); err != nil {
				return err
			}
		}
		for _, e := range res.LedgerEntries {
			_, err := tx.ExecContext(ctx, `
				INSERT INTO ledger_entries (id, iqub_id, round_id, member_id, type, amount, created_at)
				VALUES ($1,$2,$3,$4,$5,$6,$7)`,
				e.ID, e.IqubID, e.RoundID, e.MemberID, string(e.Type), e.Amount, e.CreatedAt)
			if err != nil {
				return translate(err)
			}
		}
		return nil
	})
}

// ---------------------------------------------------------------------
// Ledger
// ---------------------------------------------------------------------

func (r *IqubRepository) ListLedger(ctx context.Context, iqubID string) ([]model.LedgerEntry, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, iqub_id, round_id, member_id, type, amount, created_at
		FROM ledger_entries WHERE iqub_id = $1 ORDER BY seq`, iqubID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]model.LedgerEntry, 0)
	for rows.Next() {
		var (
			e    model.LedgerEntry
			kind string
		)
		if err := rows.Scan(&e.ID, &e.IqubID, &e.RoundID, &e.MemberID, &kind, &e.Amount, &e.CreatedAt); err != nil {
			return nil, err
		}
		e.Type = model.LedgerType(kind)
		e.CreatedAt = e.CreatedAt.UTC()
		out = append(out, e)
	}
	return out, rows.Err()
}

// ---------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------

// inTx runs fn inside a transaction: commit if it returns nil, roll back
// if it returns an error.
func (r *IqubRepository) inTx(ctx context.Context, fn func(tx *sql.Tx) error) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }() // does nothing after a successful commit
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit()
}

// expectOneRow runs an UPDATE and returns a conflict if it changed nothing.
func expectOneRow(ctx context.Context, tx *sql.Tx, conflictMsg, query string, args ...any) error {
	res, err := tx.ExecContext(ctx, query, args...)
	if err != nil {
		return translate(err)
	}
	if n, _ := res.RowsAffected(); n != 1 {
		return fmt.Errorf("%w: %s", model.ErrConflict, conflictMsg)
	}
	return nil
}

const selectIqubs = `
	SELECT i.id, i.name, i.contribution_amount, i.frequency, i.max_members, i.status,
	       i.admin_id, i.invite_code, i.created_at, i.started_at, m.member_id
	FROM iqubs i
	JOIN iqub_members m ON m.iqub_id = i.id `

const orderIqubs = ` ORDER BY i.created_at, i.id, m.join_order`

func (r *IqubRepository) findOne(ctx context.Context, where string, arg any) (model.Iqub, error) {
	list, err := r.queryIqubs(ctx, where, arg)
	if err != nil {
		return model.Iqub{}, err
	}
	if len(list) == 0 {
		return model.Iqub{}, model.ErrNotFound
	}
	return list[0], nil
}

// queryIqubs runs the group query. It returns one row per member, and the
// loop below folds those rows back into one Iqub per group.
func (r *IqubRepository) queryIqubs(ctx context.Context, where string, args ...any) ([]model.Iqub, error) {
	rows, err := r.db.QueryContext(ctx, selectIqubs+where+orderIqubs, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]model.Iqub, 0)
	for rows.Next() {
		var (
			q         model.Iqub
			freq      string
			status    string
			startedAt sql.NullTime
			memberID  string
		)
		err := rows.Scan(&q.ID, &q.Name, &q.ContributionAmount, &freq, &q.MaxMembers, &status,
			&q.AdminID, &q.InviteCode, &q.CreatedAt, &startedAt, &memberID)
		if err != nil {
			return nil, err
		}
		if n := len(out); n > 0 && out[n-1].ID == q.ID {
			out[n-1].MemberIDs = append(out[n-1].MemberIDs, memberID)
			continue
		}
		q.Frequency = model.Frequency(freq)
		q.Status = model.Status(status)
		q.CreatedAt = q.CreatedAt.UTC()
		if startedAt.Valid {
			q.StartedAt = startedAt.Time.UTC()
		}
		q.MemberIDs = []string{memberID}
		out = append(out, q)
	}
	return out, rows.Err()
}

func (r *IqubRepository) queryContributions(ctx context.Context, where string, args ...any) ([]model.Contribution, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, iqub_id, round_id, member_id, amount, status, recorded_at, confirmed_at
		FROM contributions `+where, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]model.Contribution, 0)
	for rows.Next() {
		var (
			c           model.Contribution
			status      string
			confirmedAt sql.NullTime
		)
		if err := rows.Scan(&c.ID, &c.IqubID, &c.RoundID, &c.MemberID, &c.Amount, &status,
			&c.RecordedAt, &confirmedAt); err != nil {
			return nil, err
		}
		c.Status = model.ContributionStatus(status)
		c.RecordedAt = c.RecordedAt.UTC()
		if confirmedAt.Valid {
			c.ConfirmedAt = confirmedAt.Time.UTC()
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// memberIDs returns a group's members in join order.
func memberIDs(ctx context.Context, tx *sql.Tx, iqubID string) ([]string, error) {
	rows, err := tx.QueryContext(ctx,
		`SELECT member_id FROM iqub_members WHERE iqub_id = $1 ORDER BY join_order`, iqubID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// nullTime stores a zero time as NULL.
func nullTime(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return t
}

// translate turns database errors into the domain's error kinds, using the
// PostgreSQL error code instead of guessing from the message text. The
// messages are fixed on purpose: they end up in HTTP responses, so they must
// never contain table, constraint or SQL details. Anything unrecognised
// passes through and becomes a 500 in the presentation layer.
func translate(err error) error {
	if err == nil {
		return nil
	}
	var coded interface{ SQLState() string }
	if errors.As(err, &coded) {
		switch coded.SQLState() {
		case "23505": // unique_violation
			return fmt.Errorf("%w: already exists or was changed by another request", model.ErrConflict)
		case "23503": // foreign_key_violation
			return fmt.Errorf("%w: referenced record does not exist", model.ErrNotFound)
		case "23514": // check_violation
			return fmt.Errorf("%w: value not allowed", model.ErrValidation)
		}
	}
	return err
}
