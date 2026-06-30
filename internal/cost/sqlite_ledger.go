package cost

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/wjbbeyond/guardrail/internal/authn"
	"github.com/wjbbeyond/guardrail/internal/store"

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

func (l *SQLiteLedger) AddSpend(ctx context.Context, tenantID string, day string, amount float64) error {
	if tenantID == "" {
		tenantID = authn.DefaultTenantID
	}
	query := `
INSERT INTO cost_spend (tenant_id, day, spent_usd, updated_at)
VALUES (?, ?, ?, ?)
ON CONFLICT(tenant_id, day) DO UPDATE SET
  spent_usd = spent_usd + excluded.spent_usd,
  updated_at = excluded.updated_at`
	if _, err := l.db.ExecContext(ctx, query, tenantID, day, amount, time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
		return fmt.Errorf("upsert cost spend: %w", err)
	}
	return nil
}

func (l *SQLiteLedger) Spend(ctx context.Context, tenantID string, day string) (float64, error) {
	if tenantID == "" {
		tenantID = authn.DefaultTenantID
	}
	var spent float64
	err := l.db.QueryRowContext(ctx, `SELECT spent_usd FROM cost_spend WHERE tenant_id = ? AND day = ?`, tenantID, day).Scan(&spent)
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
	migrator := store.Migrator{
		Namespace: "cost",
		Migrations: []store.Migration{
			{
				Version: 1,
				Name:    "tenant cost spend",
				Apply:   migrateTenantCostSpend,
			},
		},
	}
	return migrator.Run(ctx, l.db)
}

func migrateTenantCostSpend(ctx context.Context, db store.SQLRunner) error {
	hasTenant, err := costSpendHasTenantID(ctx, db)
	if err != nil {
		return err
	}
	if hasTenant {
		return nil
	}
	query := `
CREATE TABLE IF NOT EXISTS cost_spend_new (
  tenant_id TEXT NOT NULL,
  day TEXT NOT NULL,
  spent_usd REAL NOT NULL,
  updated_at TEXT NOT NULL,
  PRIMARY KEY (tenant_id, day)
);
INSERT INTO cost_spend_new (tenant_id, day, spent_usd, updated_at)
SELECT ?, day, spent_usd, updated_at FROM cost_spend;
DROP TABLE cost_spend;
ALTER TABLE cost_spend_new RENAME TO cost_spend;`
	if _, err := db.ExecContext(ctx, query, authn.DefaultTenantID); err != nil {
		return fmt.Errorf("migrate legacy cost spend: %w", err)
	}
	return nil
}

func costSpendHasTenantID(ctx context.Context, db store.SQLRunner) (bool, error) {
	if _, err := db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS cost_spend (
  tenant_id TEXT NOT NULL,
  day TEXT NOT NULL,
  spent_usd REAL NOT NULL,
  updated_at TEXT NOT NULL,
  PRIMARY KEY (tenant_id, day)
);`); err != nil {
		return false, fmt.Errorf("create cost spend table: %w", err)
	}
	rows, err := db.QueryContext(ctx, `PRAGMA table_info(cost_spend)`)
	if err != nil {
		return false, fmt.Errorf("inspect cost spend table: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name string
		var columnType string
		var notNull int
		var defaultValue sql.NullString
		var primaryKey int
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &primaryKey); err != nil {
			return false, fmt.Errorf("scan cost spend column: %w", err)
		}
		if strings.EqualFold(name, "tenant_id") {
			return true, nil
		}
	}
	if err := rows.Err(); err != nil {
		return false, fmt.Errorf("iterate cost spend columns: %w", err)
	}
	return false, nil
}
