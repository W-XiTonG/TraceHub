package adminhandler

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"tracehub/internal/ent"
	"tracehub/internal/ent/feedback"
	"tracehub/internal/ent/user"
	"tracehub/internal/middleware"
	"tracehub/internal/pkg/httpx"

	"github.com/gin-gonic/gin"
)

const (
	fbStatePending       = 0
	fbStateInvalid     = 1
	fbStateValidNoPay   = 2
	fbStateRewarded     = 3
	maxFeedbackReward   int64 = 50_000_000
)

// ListFeedbacks 管理端反馈列表。
func ListFeedbacks(client *ent.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
		ps, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
		if page < 1 {
			page = 1
		}
		if ps < 1 || ps > 100 {
			ps = 20
		}
		ctx := c.Request.Context()
		q := client.Feedback.Query().WithUser()
		if st := strings.TrimSpace(c.Query("review_state")); st != "" {
			if v, err := strconv.Atoi(st); err == nil && v >= 0 && v <= 3 {
				q = q.Where(feedback.ReviewStateEQ(v))
			}
		}
		if em := strings.TrimSpace(c.Query("email")); em != "" {
			q = q.Where(feedback.HasUserWith(user.EmailContainsFold(em)))
		}
		total, err := q.Clone().Count(ctx)
		if err != nil {
			httpx.ServerError(c, "查询失败")
			return
		}
		rows, err := q.
			Order(ent.Desc(feedback.FieldCreatedAt)).
			Offset((page - 1) * ps).
			Limit(ps).
			All(ctx)
		if err != nil {
			httpx.ServerError(c, "查询失败")
			return
		}
		var items []gin.H
		for _, fb := range rows {
			em := ""
			if fb.Edges.User != nil {
				em = fb.Edges.User.Email
			}
			items = append(items, gin.H{
				"id":                     fb.ID,
				"user_id":                fb.UserID,
				"user_email":             em,
				"preset_key":             fb.PresetKey,
				"title":                  fb.Title,
				"detail":                 fb.Detail,
				"review_state":           fb.ReviewState,
				"reward_points":          fb.RewardPoints,
				"balance_after_reward":   fb.BalanceAfterReward,
				"admin_note":             fb.AdminNote,
				"reviewed_by":            fb.ReviewedBy,
				"reviewed_at":            fb.ReviewedAt,
				"created_at":             fb.CreatedAt,
				"updated_at":             fb.UpdatedAt,
			})
		}
		httpx.OK(c, gin.H{"items": items, "total": total, "page": page, "page_size": ps})
	}
}

// ReviewFeedback 审核单条反馈：标记是否有效 Bug、可选发放额度（入账 balance）。
func ReviewFeedback(client *ent.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		adm := c.MustGet(middleware.CtxUserKey).(*ent.User)
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil || id <= 0 {
			httpx.BadRequest(c, "无效 id")
			return
		}
		var req struct {
			ValidBug     bool   `json:"valid_bug"`
			RewardPoints int64  `json:"reward_points"`
			AdminNote    string `json:"admin_note"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			httpx.BadRequest(c, "参数无效")
			return
		}
		req.AdminNote = strings.TrimSpace(req.AdminNote)
		if req.RewardPoints < 0 {
			httpx.BadRequest(c, "reward_points 不能为负")
			return
		}
		if req.RewardPoints > maxFeedbackReward {
			httpx.BadRequest(c, "单次奖励额度过大")
			return
		}
		if !req.ValidBug && req.RewardPoints > 0 {
			httpx.BadRequest(c, "无效反馈不能发放奖励")
			return
		}
		ctx := c.Request.Context()

		tx, err := client.Tx(ctx)
		if err != nil {
			httpx.ServerError(c, "事务开启失败")
			return
		}
		defer func() { _ = tx.Rollback() }()

		fb, err := tx.Feedback.Query().Where(feedback.IDEQ(id)).Only(ctx)
		if err != nil {
			if ent.IsNotFound(err) {
				httpx.Err(c, http.StatusNotFound, 40400, "不存在")
				return
			}
			httpx.ServerError(c, "查询失败")
			return
		}
		if fb.ReviewState != fbStatePending {
			httpx.BadRequest(c, "该反馈已处理")
			return
		}

		now := time.Now()
		if !req.ValidBug {
			if err := tx.Feedback.UpdateOneID(id).
				SetReviewState(fbStateInvalid).
				SetRewardPoints(0).
				SetAdminNote(req.AdminNote).
				SetReviewedBy(adm.ID).
				SetReviewedAt(now).
				Exec(ctx); err != nil {
				httpx.ServerError(c, "更新失败")
				return
			}
			if err := tx.Commit(); err != nil {
				httpx.ServerError(c, "提交失败")
				return
			}
			auditLog(client, adm.ID, "feedback.review", "feedback", strconv.Itoa(id), map[string]any{"state": fbStateInvalid}, c)
			httpx.OKMsg(c, "已标记为无效", nil)
			return
		}

		if req.RewardPoints == 0 {
			if err := tx.Feedback.UpdateOneID(id).
				SetReviewState(fbStateValidNoPay).
				SetRewardPoints(0).
				SetAdminNote(req.AdminNote).
				SetReviewedBy(adm.ID).
				SetReviewedAt(now).
				Exec(ctx); err != nil {
				httpx.ServerError(c, "更新失败")
				return
			}
			if err := tx.Commit(); err != nil {
				httpx.ServerError(c, "提交失败")
				return
			}
			auditLog(client, adm.ID, "feedback.review", "feedback", strconv.Itoa(id), map[string]any{"state": fbStateValidNoPay}, c)
			httpx.OKMsg(c, "已确认有效（未发奖）", nil)
			return
		}

		if err := tx.User.UpdateOneID(fb.UserID).AddBalance(req.RewardPoints).Exec(ctx); err != nil {
			httpx.ServerError(c, "入账失败")
			return
		}
		nu, err := tx.User.Get(ctx, fb.UserID)
		if err != nil {
			httpx.ServerError(c, "查询余额失败")
			return
		}
		if err := tx.Feedback.UpdateOneID(id).
			SetReviewState(fbStateRewarded).
			SetRewardPoints(req.RewardPoints).
			SetBalanceAfterReward(nu.Balance).
			SetAdminNote(req.AdminNote).
			SetReviewedBy(adm.ID).
			SetReviewedAt(now).
			Exec(ctx); err != nil {
			httpx.ServerError(c, "更新失败")
			return
		}
		if err := tx.Commit(); err != nil {
			httpx.ServerError(c, "提交失败")
			return
		}
		auditLog(client, adm.ID, "feedback.reward", "feedback", strconv.Itoa(id), map[string]any{
			"user_id": fb.UserID, "points": req.RewardPoints, "balance_after": nu.Balance,
		}, c)
		httpx.OKMsg(c, "已发放奖励", gin.H{"new_balance": nu.Balance})
	}
}
