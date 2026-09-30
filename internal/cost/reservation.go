package cost

import (
	"context"
	"crypto/rand"
	"fmt"
	"math"
)

// Reservations are charged before dispatch. Unknown outcomes retain the charge;
// only confirmed usage releases the unused portion. This survives process loss.
type Reservation struct {
	ID       string
	TenantID string
	Day      string
	Amount   float64
}

type reservationLedger interface {
	Reserve(context.Context, Reservation, float64) (bool, error)
	Settle(context.Context, Reservation, float64) error
}

func (t *Tracker) ReserveTenant(ctx context.Context, tenant, model string, prompt, completion int) (Reservation, Decision, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if !t.pricer.HasPrice(model) {
		return Reservation{}, Decision{Reason: "model_price_required", TenantID: tenant}, nil
	}
	amount := t.pricer.Price(model, prompt, completion)
	if math.IsInf(amount, 0) || math.IsNaN(amount) || amount < 0 {
		return Reservation{}, Decision{}, fmt.Errorf("invalid reservation cost")
	}
	budget := t.budgetForTenant(tenant)
	decision := Decision{TenantID: tenant, Limit: budget.Daily}
	if budget.PerReq > 0 && amount > budget.PerReq {
		decision.Reason = "per_request_budget_exceeded"
		return Reservation{}, decision, nil
	}
	r := Reservation{ID: rand.Text(), TenantID: tenant, Day: t.day(), Amount: amount}
	if t.ledger != nil {
		ledger, ok := t.ledger.(reservationLedger)
		if !ok {
			return r, decision, fmt.Errorf("ledger does not support atomic reservations")
		}
		allowed, err := ledger.Reserve(ctx, r, budget.Daily)
		if err != nil {
			return r, decision, err
		}
		decision.Allowed = allowed
	} else {
		key := spendKey(tenant, r.Day)
		decision.Spent = t.spendByDay[key]
		decision.Allowed = budget.Daily <= 0 || decision.Spent+amount <= budget.Daily
		if decision.Allowed {
			t.spendByDay[key] += amount
			t.reservations[r.ID] = r
		}
	}
	if !decision.Allowed {
		decision.Reason = "daily_budget_exceeded"
	}
	return r, decision, nil
}

func (t *Tracker) Settle(ctx context.Context, r Reservation, model string, prompt, completion int) (Usage, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	usage := Usage{PromptTokens: prompt, CompletionTokens: completion, CostUSD: t.pricer.Price(model, prompt, completion)}
	if t.ledger != nil {
		ledger, ok := t.ledger.(reservationLedger)
		if !ok {
			return usage, fmt.Errorf("ledger does not support settlement")
		}
		return usage, ledger.Settle(ctx, r, usage.CostUSD)
	}
	if saved, ok := t.reservations[r.ID]; ok {
		t.spendByDay[spendKey(saved.TenantID, saved.Day)] += usage.CostUSD - saved.Amount
		delete(t.reservations, r.ID)
	}
	return usage, nil
}

func (l *SQLiteLedger) Reserve(ctx context.Context, r Reservation, limit float64) (bool, error) {
	tx, err := l.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `INSERT INTO cost_spend (tenant_id,day,spent_usd,updated_at)
 SELECT ?,?,?,CURRENT_TIMESTAMP WHERE ? <= 0 OR ? <= ?
 ON CONFLICT(tenant_id,day) DO UPDATE SET spent_usd=spent_usd+excluded.spent_usd,updated_at=CURRENT_TIMESTAMP
 WHERE ? <= 0 OR spent_usd+excluded.spent_usd <= ?`, r.TenantID, r.Day, r.Amount, limit, r.Amount, limit, limit, limit)
	if err != nil {
		return false, err
	}
	n, err := result.RowsAffected()
	if err != nil || n == 0 {
		return false, err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO cost_reservations(id,tenant_id,day,amount) VALUES (?,?,?,?)`, r.ID, r.TenantID, r.Day, r.Amount)
	if err != nil {
		return false, err
	}
	return true, tx.Commit()
}

func (l *SQLiteLedger) Settle(ctx context.Context, r Reservation, amount float64) error {
	tx, err := l.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	// Acquire the write lock before reading. The stored reservation is authoritative.
	result, err := tx.ExecContext(ctx, `UPDATE cost_spend SET spent_usd=spent_usd+?-(SELECT amount FROM cost_reservations WHERE id=?),updated_at=CURRENT_TIMESTAMP
 WHERE (tenant_id,day) IN (SELECT tenant_id,day FROM cost_reservations WHERE id=?)`, amount, r.ID, r.ID)
	if err != nil {
		return err
	}
	if _, err = result.RowsAffected(); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM cost_reservations WHERE id=?`, r.ID); err != nil {
		return err
	}
	return tx.Commit()
}
