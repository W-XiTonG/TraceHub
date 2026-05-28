package pricingsyncmigrate

import (
	"context"
	"database/sql"
	"fmt"
)

const (
	correctPullPath = "/api/pricing/sync/export"
	correctPushPath = "/api/pricing/sync/import"
)

// FixLegacyPullPaths 将历史错误的默认定价同步路径修正为当前 API 路径。
func FixLegacyPullPaths(ctx context.Context, db *sql.DB) error {
	if db == nil {
		return nil
	}
	if _, err := db.ExecContext(ctx, `
UPDATE model_pricing_sync_targets
SET pull_path = $1
WHERE pull_path = '/api/pricing/export' OR pull_path = ''`, correctPullPath); err != nil {
		return fmt.Errorf("fix pull_path: %w", err)
	}
	if _, err := db.ExecContext(ctx, `
UPDATE model_pricing_sync_targets
SET push_path = $1
	WHERE push_path = '/api/pricing/import' OR push_path = ''`, correctPushPath); err != nil {
		return fmt.Errorf("fix push_path: %w", err)
	}
	return nil
}
