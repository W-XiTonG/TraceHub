package userhandler

import (
	"context"
	"database/sql"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"tracehub/internal/ent"
	"tracehub/internal/middleware"
	"tracehub/internal/pkg/httpx"

	"github.com/gin-gonic/gin"
)

// BalanceLedger 合并「API 调用扣费」「兑换码入账」「管理员余额调整」「反馈审核奖励」为统一时间线（PostgreSQL）。
func BalanceLedger(db *sql.DB) gin.HandlerFunc {
	if db == nil {
		return func(c *gin.Context) {
			httpx.ServerError(c, "余额流水服务未初始化")
		}
	}
	return func(c *gin.Context) {
		u := c.MustGet(middleware.CtxUserKey).(*ent.User)
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
		uid := u.ID
		rid := strconv.Itoa(uid)

		entryType := strings.TrimSpace(c.Query("entry_type"))
		if entryType != "" {
			switch entryType {
			case "api_usage", "redeem", "admin_adjust", "feedback_reward":
			default:
				entryType = ""
			}
		}
		dir := strings.TrimSpace(c.Query("direction"))
		if dir != "" && dir != "in" && dir != "out" {
			dir = ""
		}
		var startPtr, endPtr *time.Time
		if st := strings.TrimSpace(c.Query("start_time")); st != "" {
			if t0, err := time.Parse(time.RFC3339, st); err == nil {
				startPtr = &t0
			}
		}
		if et := strings.TrimSpace(c.Query("end_time")); et != "" {
			if t0, err := time.Parse(time.RFC3339, et); err == nil {
				endPtr = &t0
			}
		}

		total, err := countBalanceLedger(ctx, db, uid, rid, entryType, startPtr, endPtr, dir)
		if err != nil {
			httpx.ServerError(c, "查询失败")
			return
		}
		rows, err := queryBalanceLedgerPage(ctx, db, uid, rid, entryType, startPtr, endPtr, dir, ps, offset)
		if err != nil {
			httpx.ServerError(c, "查询失败")
			return
		}
		httpx.OK(c, gin.H{"items": rows, "total": total, "page": page, "page_size": ps})
	}
}

func countBalanceLedger(ctx context.Context, db *sql.DB, userID int, resourceID, entryType string, start, end *time.Time, direction string) (int64, error) {
	var startArg any
	if start != nil {
		startArg = *start
	}
	var endArg any
	if end != nil {
		endArg = *end
	}
	const q = `
SELECT COUNT(*) FROM (
  SELECT event_time, entry_type, delta
  FROM (
    SELECT
      ul.request_time AS event_time,
      'api_usage'::text AS entry_type,
      (-ul.total_cost)::bigint AS delta
    FROM usage_logs ul
    WHERE ul.user_id = $1 AND ul.total_cost <> 0

    UNION ALL

    SELECT
      rl.created_at,
      'redeem',
      rl.amount::bigint
    FROM redeem_ledgers rl
    WHERE rl.user_id = $1

    UNION ALL

    SELECT
      aal.created_at,
      'admin_adjust',
      COALESCE(NULLIF(trim(aal.detail->>'delta'), ''), '0')::numeric::bigint
    FROM admin_audit_logs aal
    WHERE aal.action = 'user.balance'
      AND aal.resource = 'user'
      AND aal.resource_id = $2
      AND aal.detail ? 'delta'
      AND COALESCE(NULLIF(trim(aal.detail->>'delta'), ''), '0')::numeric <> 0

    UNION ALL

    SELECT
      COALESCE(f.reviewed_at, f.updated_at) AS event_time,
      'feedback_reward'::text AS entry_type,
      f.reward_points AS delta
    FROM feedbacks f
    WHERE f.user_id = $1 AND f.review_state = 3 AND f.reward_points <> 0
  ) u
  WHERE ($3::text = '' OR entry_type = $3)
    AND ($4::timestamptz IS NULL OR event_time >= $4::timestamptz)
    AND ($5::timestamptz IS NULL OR event_time <= $5::timestamptz)
    AND ($6::text = '' OR ($6 = 'in' AND delta > 0) OR ($6 = 'out' AND delta < 0))
) c`
	var n int64
	if err := db.QueryRowContext(ctx, q, userID, resourceID, entryType, startArg, endArg, direction).Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}

func queryBalanceLedgerPage(ctx context.Context, db *sql.DB, userID int, resourceID, entryType string, start, end *time.Time, direction string, limit, offset int) ([]gin.H, error) {
	var startArg any
	if start != nil {
		startArg = *start
	}
	var endArg any
	if end != nil {
		endArg = *end
	}
	const q = `
SELECT event_time, entry_type, delta, balance_after, sort_src, sort_id, detail
FROM (
  SELECT
    ul.request_time AS event_time,
    'api_usage'::text AS entry_type,
    (-ul.total_cost)::bigint AS delta,
    NULL::bigint AS balance_after,
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
      'channel_id', ul.channel_id,
      'response_time_ms', ul.response_time_ms
    )::jsonb AS detail
  FROM usage_logs ul
  WHERE ul.user_id = $1 AND ul.total_cost <> 0

  UNION ALL

  SELECT
    rl.created_at,
    'redeem',
    rl.amount::bigint,
    rl.balance_after,
    1,
    rl.id,
    json_build_object(
      'redeem_ledger_id', rl.id,
      'redeem_code_id', rl.redeem_code_id,
      'code_hint', rl.code_hint
    )::jsonb
  FROM redeem_ledgers rl
  WHERE rl.user_id = $1

  UNION ALL

  SELECT
    aal.created_at,
    'admin_adjust',
    COALESCE(NULLIF(trim(aal.detail->>'delta'), ''), '0')::numeric::bigint,
    NULL::bigint,
    2,
    aal.id,
    json_build_object(
      'audit_log_id', aal.id,
      'admin_id', aal.admin_id,
      'delta', aal.detail->'delta'
    )::jsonb
  FROM admin_audit_logs aal
  WHERE aal.action = 'user.balance'
    AND aal.resource = 'user'
    AND aal.resource_id = $2
    AND aal.detail ? 'delta'
    AND COALESCE(NULLIF(trim(aal.detail->>'delta'), ''), '0')::numeric <> 0

  UNION ALL

  SELECT
    COALESCE(f.reviewed_at, f.updated_at) AS event_time,
    'feedback_reward'::text AS entry_type,
    f.reward_points AS delta,
    f.balance_after_reward,
    3 AS sort_src,
    f.id AS sort_id,
    json_build_object(
      'feedback_id', f.id,
      'title', f.title,
      'preset_key', f.preset_key
    )::jsonb AS detail
  FROM feedbacks f
  WHERE f.user_id = $1 AND f.review_state = 3 AND f.reward_points <> 0
) x
WHERE ($3::text = '' OR entry_type = $3)
  AND ($4::timestamptz IS NULL OR event_time >= $4::timestamptz)
  AND ($5::timestamptz IS NULL OR event_time <= $5::timestamptz)
  AND ($6::text = '' OR ($6 = 'in' AND delta > 0) OR ($6 = 'out' AND delta < 0))
ORDER BY event_time DESC, sort_src ASC, sort_id DESC
LIMIT $7 OFFSET $8`

	rs, err := db.QueryContext(ctx, q, userID, resourceID, entryType, startArg, endArg, direction, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rs.Close()

	out := make([]gin.H, 0, limit)
	for rs.Next() {
		var (
			eventTime    sql.NullTime
			entryType    string
			delta        int64
			balanceAfter sql.NullInt64
			sortSrc      int
			sortID       int
			detailRaw    []byte
		)
		if err := rs.Scan(&eventTime, &entryType, &delta, &balanceAfter, &sortSrc, &sortID, &detailRaw); err != nil {
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
			"entry_type":    entryType,
			"delta":         delta,
			"direction":     dir,
			"amount_abs":    abs64(delta),
			"balance_after": nil,
			"detail":        detail,
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

func abs64(n int64) int64 {
	if n < 0 {
		return -n
	}
	return n
}
