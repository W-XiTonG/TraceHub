package usagelogmigrate

import (
	"context"
	"database/sql"
	"fmt"
)

// BackfillFailoverFlags 修正历史 usage_logs 的 failover 标记：
// 先全部视为客户端 outcome；仅同 request_id 下存在更高 upstream_attempt 的行标为中间尝试。
func BackfillFailoverFlags(ctx context.Context, db *sql.DB) error {
	const resetSQL = `
UPDATE usage_logs
SET is_client_outcome = true, is_failover_attempt = false
WHERE is_failover_attempt = false OR is_client_outcome = false`
	if _, err := db.ExecContext(ctx, resetSQL); err != nil {
		return fmt.Errorf("reset usage_logs failover flags: %w", err)
	}

	const markIntermediateSQL = `
UPDATE usage_logs AS ul
SET is_client_outcome = false, is_failover_attempt = true
WHERE ul.request_id IS NOT NULL
  AND ul.request_id <> ''
  AND EXISTS (
    SELECT 1 FROM usage_logs AS ul2
    WHERE ul2.request_id = ul.request_id
      AND ul2.upstream_attempt > ul.upstream_attempt
  )`
	if _, err := db.ExecContext(ctx, markIntermediateSQL); err != nil {
		return fmt.Errorf("mark failover intermediate usage_logs: %w", err)
	}
	return nil
}
