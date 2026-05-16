package adminhandler

import (
	"context"
	"database/sql"
	"encoding/json"
	"strconv"
	"strings"

	"tracehub/internal/pkg/httpx"

	"github.com/gin-gonic/gin"
)

// AdminBalanceLedger 全站额度流水（PostgreSQL）：API 扣费、兑换入账、管理员调账、反馈奖励；支持按用户、邮箱、类型筛选。
func AdminBalanceLedger(db *sql.DB) gin.HandlerFunc {
	if db == nil {
		return func(c *gin.Context) {
			httpx.ServerError(c, "余额流水服务未初始化")
		}
	}
	return func(c *gin.Context) {
		page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
		ps, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
		if page < 1 {
			page = 1
		}
		if ps < 1 || ps > 100 {
			ps = 20
		}
		offset := (page - 1) * ps
		ctx := c.Request.Context()

		uid, _ := strconv.Atoi(strings.TrimSpace(c.Query("user_id")))
		email := strings.TrimSpace(c.Query("email"))
		entryType := strings.TrimSpace(c.Query("entry_type"))
		if entryType != "" {
			switch entryType {
			case "api_usage", "redeem", "admin_adjust", "feedback_reward", "payment_recharge", "payment_refund":
			default:
				httpx.BadRequest(c, "无效的 entry_type")
				return
			}
		}

		total, err := countAdminBalanceLedger(ctx, db, uid, email, entryType)
		if err != nil {
			httpx.ServerError(c, "查询失败")
			return
		}
		rows, err := queryAdminBalanceLedgerPage(ctx, db, uid, email, entryType, ps, offset)
		if err != nil {
			httpx.ServerError(c, "查询失败")
			return
		}
		httpx.OK(c, gin.H{"items": rows, "total": total, "page": page, "page_size": ps})
	}
}

const adminLedgerInnerSQL = `
SELECT
  event_time,
  entry_type,
  delta,
  balance_after,
  user_id,
  user_email,
  sort_src,
  sort_id,
  detail
FROM (
  SELECT
    ul.request_time AS event_time,
    'api_usage'::text AS entry_type,
    (-ul.total_cost)::bigint AS delta,
    NULL::bigint AS balance_after,
    ul.user_id,
    COALESCE(u.email, '')::text AS user_email,
    0 AS sort_src,
    ul.id AS sort_id,
    json_build_object(
      'usage_log_id', ul.id,
      'model', ul.model,
      'model_upstream', ul.model_upstream,
      'prompt_tokens', ul.prompt_tokens,
      'completion_tokens', ul.completion_tokens,
      'total_tokens', ul.total_tokens,
      'total_cost', ul.total_cost,
      'request_id', ul.request_id,
      'status_code', ul.status_code,
      'api_key_id', ul.api_key_id,
      'api_key_name', ak.name,
      'api_key_prefix', ak.key_prefix,
      'channel_id', ul.channel_id,
      'response_time_ms', ul.response_time_ms
    )::jsonb AS detail
  FROM usage_logs ul
  LEFT JOIN users u ON u.id = ul.user_id
  LEFT JOIN api_keys ak ON ak.id = ul.api_key_id
  WHERE ul.total_cost <> 0

  UNION ALL

  SELECT
    rl.created_at,
    'redeem',
    rl.amount::bigint,
    rl.balance_after,
    rl.user_id,
    COALESCE(u2.email, '')::text,
    1,
    rl.id,
    json_build_object(
      'redeem_ledger_id', rl.id,
      'redeem_code_id', rl.redeem_code_id,
      'code_hint', rl.code_hint
    )::jsonb
  FROM redeem_ledgers rl
  LEFT JOIN users u2 ON u2.id = rl.user_id

  UNION ALL

  SELECT
    aal.created_at,
    'admin_adjust',
    COALESCE(NULLIF(trim(aal.detail->>'delta'), ''), '0')::numeric::bigint,
    NULL::bigint,
    CAST(trim(aal.resource_id) AS int),
    COALESCE(u3.email, '')::text,
    2,
    aal.id,
    json_build_object(
      'audit_log_id', aal.id,
      'admin_id', aal.admin_id,
      'delta', aal.detail->'delta'
    )::jsonb
  FROM admin_audit_logs aal
  LEFT JOIN users u3 ON u3.id = CAST(trim(aal.resource_id) AS int)
  WHERE aal.action = 'user.balance'
    AND aal.resource = 'user'
    AND aal.resource_id ~ '^[0-9]+$'
    AND aal.detail ? 'delta'
    AND COALESCE(NULLIF(trim(aal.detail->>'delta'), ''), '0')::numeric <> 0

  UNION ALL

  SELECT
    COALESCE(f.reviewed_at, f.updated_at) AS event_time,
    'feedback_reward'::text AS entry_type,
    f.reward_points AS delta,
    f.balance_after_reward::bigint,
    f.user_id,
    COALESCE(u4.email, '')::text,
    3,
    f.id,
    json_build_object(
      'feedback_id', f.id,
      'title', f.title,
      'preset_key', f.preset_key
    )::jsonb
  FROM feedbacks f
  LEFT JOIN users u4 ON u4.id = f.user_id
  WHERE f.review_state = 3 AND f.reward_points <> 0

  UNION ALL

  SELECT
    pl.created_at,
    CASE WHEN pl.entry_type = 'refund' THEN 'payment_refund' ELSE 'payment_recharge' END,
    pl.delta::bigint,
    pl.balance_after,
    pl.user_id,
    COALESCE(u5.email, '')::text,
    4,
    pl.id,
    json_build_object(
      'payment_ledger_id', pl.id,
      'order_id', pl.order_id,
      'entry_type', pl.entry_type
    )::jsonb
  FROM payment_ledgers pl
  LEFT JOIN users u5 ON u5.id = pl.user_id
) _ledger_q`

func adminLedgerFilterSQL(uid int, email, entryType string) (where string, args []any) {
	args = make([]any, 0, 4)
	n := 1
	if uid > 0 {
		where += " AND user_id = $" + strconv.Itoa(n)
		args = append(args, uid)
		n++
	}
	if email != "" {
		where += " AND user_email ILIKE $" + strconv.Itoa(n)
		args = append(args, "%"+email+"%")
		n++
	}
	if entryType != "" {
		where += " AND entry_type = $" + strconv.Itoa(n)
		args = append(args, entryType)
		n++
	}
	return where, args
}

func countAdminBalanceLedger(ctx context.Context, db *sql.DB, uid int, email, entryType string) (int64, error) {
	where, args := adminLedgerFilterSQL(uid, email, entryType)
	q := "SELECT COUNT(*) FROM (" + adminLedgerInnerSQL + ") x WHERE 1=1" + where
	var n int64
	if err := db.QueryRowContext(ctx, q, args...).Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}

func queryAdminBalanceLedgerPage(ctx context.Context, db *sql.DB, uid int, email, entryType string, limit, offset int) ([]gin.H, error) {
	where, args := adminLedgerFilterSQL(uid, email, entryType)
	n := len(args) + 1
	limArg := n
	offArg := n + 1
	args = append(args, limit, offset)
	q := "SELECT event_time, entry_type, delta, balance_after, user_id, user_email, sort_src, sort_id, detail FROM (" +
		adminLedgerInnerSQL +
		") x WHERE 1=1" + where +
		" ORDER BY event_time DESC, sort_src ASC, sort_id DESC LIMIT $" + strconv.Itoa(limArg) + " OFFSET $" + strconv.Itoa(offArg)

	rs, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rs.Close()

	out := make([]gin.H, 0, limit)
	for rs.Next() {
		var (
			eventTime    sql.NullTime
			entryTypeCol string
			delta        int64
			balanceAfter sql.NullInt64
			userID       sql.NullInt64
			userEmail    string
			sortSrc      int
			sortID       int
			detailRaw    []byte
		)
		if err := rs.Scan(&eventTime, &entryTypeCol, &delta, &balanceAfter, &userID, &userEmail, &sortSrc, &sortID, &detailRaw); err != nil {
			return nil, err
		}
		var detail map[string]any
		if len(detailRaw) > 0 {
			_ = json.Unmarshal(detailRaw, &detail)
		}
		if detail == nil {
			detail = map[string]any{}
		}
		dir := "out"
		if delta > 0 {
			dir = "in"
		}
		item := gin.H{
			"event_time":    eventTime.Time,
			"entry_type":    entryTypeCol,
			"delta":         delta,
			"direction":     dir,
			"amount_abs":    absLedger64(delta),
			"balance_after": nil,
			"user_email":    userEmail,
			"detail":        detail,
		}
		if userID.Valid {
			item["user_id"] = int(userID.Int64)
		}
		if balanceAfter.Valid {
			item["balance_after"] = balanceAfter.Int64
		}
		out = append(out, item)
	}
	if err := rs.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func absLedger64(n int64) int64 {
	if n < 0 {
		return -n
	}
	return n
}
