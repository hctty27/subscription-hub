package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/hctty27/subscription-hub/backend/internal/billing"
)

type Renewal struct {
	ID          int64  `json:"id"`
	ItemID      int64  `json:"item_id"`
	DueDate     string `json:"due_date"`
	PaymentID   int64  `json:"payment_id"`
	NextDueDate string `json:"next_due_date"`
	PaidDate    string `json:"paid_date"`
	AmountMinor int64  `json:"amount_minor"`
	Currency    string `json:"currency"`
	CreatedAt   string `json:"created_at"`
}

func (s *Store) CurrentRenewal(ctx context.Context, item Item, today time.Time) (string, bool, error) {
	candidate, err := billing.NextDue(item.FirstDueDate, item.PeriodUnit, item.PeriodCount, today)
	if err != nil {
		return "", false, err
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO item_renewal_state(item_id,due_date)
		VALUES(?,?) ON CONFLICT(item_id) DO NOTHING`, item.ID, candidate)
	if err != nil {
		return "", false, err
	}
	var due string
	var completed bool
	err = s.db.QueryRowContext(ctx,
		"SELECT due_date,completed FROM item_renewal_state WHERE item_id=?", item.ID).Scan(&due, &completed)
	return due, completed, err
}

// SettleRenewal records an actual payment and moves the actionable due date in one transaction.
// The next cycle always uses the original calendar anchor, including month-end clamping.
func (s *Store) SettleRenewal(ctx context.Context, payment Payment, expectedDue string, today time.Time) (Renewal, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Renewal{}, err
	}
	defer tx.Rollback()
	item, err := scanItem(tx.QueryRowContext(ctx, "SELECT "+itemColumns+" FROM items WHERE id=?", payment.ItemID))
	if errors.Is(err, sql.ErrNoRows) {
		return Renewal{}, ErrNotFound
	}
	if err != nil {
		return Renewal{}, err
	}
	if item.Status != "active" || item.Currency != payment.Currency {
		return Renewal{}, ErrConflict
	}
	var due string
	var completed bool
	err = tx.QueryRowContext(ctx, "SELECT due_date,completed FROM item_renewal_state WHERE item_id=?", item.ID).Scan(&due, &completed)
	if errors.Is(err, sql.ErrNoRows) {
		due, err = billing.NextDue(item.FirstDueDate, item.PeriodUnit, item.PeriodCount, today)
		if err != nil {
			return Renewal{}, err
		}
		_, err = tx.ExecContext(ctx, "INSERT INTO item_renewal_state(item_id,due_date) VALUES(?,?)", item.ID, due)
		if err != nil {
			return Renewal{}, constraintError(err)
		}
		completed = false
	} else if err != nil {
		return Renewal{}, err
	}
	if completed || due != expectedDue {
		return Renewal{}, ErrConflict
	}
	next := ""
	if item.PeriodUnit != "none" {
		dueDate, err := time.Parse(billing.DateLayout, due)
		if err != nil {
			return Renewal{}, err
		}
		next, err = billing.NextDue(item.FirstDueDate, item.PeriodUnit, item.PeriodCount, dueDate.AddDate(0, 0, 1))
		if err != nil {
			return Renewal{}, err
		}
		if next <= due {
			return Renewal{}, fmt.Errorf("续费周期计算未能推进")
		}
	}
	res, err := tx.ExecContext(ctx,
		"INSERT INTO payments(item_id,amount_minor,currency,paid_date,notes) VALUES(?,?,?,?,?)",
		item.ID, payment.AmountMinor, payment.Currency, payment.PaidDate, payment.Notes)
	if err != nil {
		return Renewal{}, constraintError(err)
	}
	paymentID, err := res.LastInsertId()
	if err != nil {
		return Renewal{}, err
	}
	res, err = tx.ExecContext(ctx,
		"INSERT INTO item_renewals(item_id,due_date,payment_id,next_due_date) VALUES(?,?,?,?)",
		item.ID, due, paymentID, next)
	if err != nil {
		return Renewal{}, constraintError(err)
	}
	renewalID, err := res.LastInsertId()
	if err != nil {
		return Renewal{}, err
	}
	storedDue := next
	if item.PeriodUnit == "none" {
		storedDue = due
	}
	_, err = tx.ExecContext(ctx, `UPDATE item_renewal_state SET due_date=?,completed=?,
		updated_at=strftime('%Y-%m-%dT%H:%M:%fZ','now') WHERE item_id=?`,
		storedDue, item.PeriodUnit == "none", item.ID)
	if err != nil {
		return Renewal{}, err
	}
	// A completed cycle no longer needs an unread reminder.
	if _, err := tx.ExecContext(ctx, `UPDATE reminder_notifications
		SET read_at=COALESCE(read_at,strftime('%Y-%m-%dT%H:%M:%fZ','now'))
		WHERE item_id=? AND due_date=?`, item.ID, due); err != nil {
		return Renewal{}, err
	}
	if err := tx.Commit(); err != nil {
		return Renewal{}, err
	}
	return Renewal{
		ID: renewalID, ItemID: item.ID, DueDate: due, PaymentID: paymentID,
		NextDueDate: next, PaidDate: payment.PaidDate,
		AmountMinor: payment.AmountMinor, Currency: payment.Currency,
	}, nil
}

func (s *Store) ListRenewals(ctx context.Context, itemID int64, limit int) ([]Renewal, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT r.id,r.item_id,r.due_date,r.payment_id,
		r.next_due_date,p.paid_date,p.amount_minor,p.currency,r.created_at
		FROM item_renewals r JOIN payments p ON p.id=r.payment_id
		WHERE r.item_id=? ORDER BY r.id DESC LIMIT ?`, itemID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Renewal, 0)
	for rows.Next() {
		var r Renewal
		if err := rows.Scan(&r.ID, &r.ItemID, &r.DueDate, &r.PaymentID, &r.NextDueDate,
			&r.PaidDate, &r.AmountMinor, &r.Currency, &r.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
