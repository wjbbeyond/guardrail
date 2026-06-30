package cost

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

type SQLiteLedger struct {
	db *sql.DB
}

func OpenSQLiteLedger(ctx context.Context, dsn string) (*SQLiteLedger, error) {
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite cost ledger: %w", err)
	}
	ledger := &SQLiteLedger{db: db}
	if err := ledger.migrate(ctx); err != nil {
		db.Close()
		return nil, err
	}
	return ledger, nil
}

func (l *SQLiteLedger) AddSpend(ctx context.Context, day string, amount float64) error {
	query := `
INSERT INTO cost_spend (day, spent_usd, updated_at)
VALUES (?, ?, ?)
ON CONFLICT(day) DO UPDATE SET
  spent_usd = spent_usd + excluded.spent_usd,
  updated_at = excluded.updated_at`
	if _, err := l.db.ExecContext(ctx, query, day, amount, time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
		return fmt.Errorf("upsert cost spend: %w", err)
	}
	return nil
}

func (l *SQLiteLedger) Spend(ctx context.Context, day string) (float64, error) {
	var spent float64
	err := l.db.QueryRowContext(ctx, `SELECT spent_usd FROM cost_spend WHERE day = ?`, day).Scan(&spent)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("select cost spend: %w", err)
	}
	return spent, nil
}

func (l *SQLiteLedger) Close() {
	l.db.Close()
}

func (l *SQLiteLedger) migrate(ctx context.Context) error {
	query := `
CREATE TABLE IF NOT EXISTS cost_spend (
  day TEXT PRIMARY KEY,
  spent_usd REAL NOT NULL,
  updated_at TEXT NOT NULL
);`
	if _, err := l.db.ExecContext(ctx, query); err != nil {
		return fmt.Errorf("migrate cost ledger: %w", err)
	}
	return nil
}
