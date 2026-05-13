package userhandler

import (
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"tracehub/internal/ent"
	"tracehub/internal/ent/feedback"
	"tracehub/internal/middleware"
	"tracehub/internal/pkg/feedbackpreset"
	"tracehub/internal/pkg/httpx"

	"github.com/gin-gonic/gin"
)

const (
	feedbackTitleMax  = 200
	feedbackDetailMax = 12000
	feedbackDailyCap  = 20
)

// CreateFeedback 提交问题反馈（需登录）。
func CreateFeedback(client *ent.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		u := c.MustGet(middleware.CtxUserKey).(*ent.User)
		var req struct {
			PresetKey string `json:"preset_key"`
			Title     string `json:"title"`
			Detail    string `json:"detail"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			httpx.BadRequest(c, "参数无效")
			return
		}
		req.PresetKey = strings.TrimSpace(req.PresetKey)
		req.Title = strings.TrimSpace(req.Title)
		req.Detail = strings.TrimSpace(req.Detail)
		if !feedbackpreset.ValidPresetKey(req.PresetKey) {
			httpx.BadRequest(c, "无效的反馈类型")
			return
		}
		if req.Title == "" || utf8.RuneCountInString(req.Title) > feedbackTitleMax {
			httpx.BadRequest(c, "标题长度不合法")
			return
		}
		if len(req.Detail) < 10 || utf8.RuneCountInString(req.Detail) > feedbackDetailMax {
			httpx.BadRequest(c, "请补充详细描述（10～12000 字）")
			return
		}
		ctx := c.Request.Context()
		start := time.Now().Truncate(24 * time.Hour)
		todayN, err := client.Feedback.Query().
			Where(feedback.UserIDEQ(u.ID), feedback.CreatedAtGTE(start)).
			Count(ctx)
		if err != nil {
			httpx.ServerError(c, "提交失败")
			return
		}
		if todayN >= feedbackDailyCap {
			httpx.BadRequest(c, "今日反馈次数已达上限，请明日再试")
			return
		}
		fb, err := client.Feedback.Create().
			SetUserID(u.ID).
			SetPresetKey(req.PresetKey).
			SetTitle(req.Title).
			SetDetail(req.Detail).
			Save(ctx)
		if err != nil {
			httpx.ServerError(c, "提交失败")
			return
		}
		httpx.OK(c, gin.H{"id": fb.ID})
	}
}

// ListMyFeedbacks 当前用户反馈列表。
func ListMyFeedbacks(client *ent.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		u := c.MustGet(middleware.CtxUserKey).(*ent.User)
		page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
		ps, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
		if page < 1 {
			page = 1
		}
		if ps < 1 || ps > 50 {
			ps = 20
		}
		ctx := c.Request.Context()
		q := client.Feedback.Query().Where(feedback.UserIDEQ(u.ID))
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
			items = append(items, gin.H{
				"id":            fb.ID,
				"preset_key":    fb.PresetKey,
				"title":         fb.Title,
				"detail":        fb.Detail,
				"review_state":  fb.ReviewState,
				"reward_points": fb.RewardPoints,
				"created_at":    fb.CreatedAt,
				"reviewed_at":   fb.ReviewedAt,
			})
		}
		httpx.OK(c, gin.H{"items": items, "total": total, "page": page, "page_size": ps})
	}
}

// GetFeedback 单条反馈详情（仅本人）。
func GetFeedback(client *ent.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		u := c.MustGet(middleware.CtxUserKey).(*ent.User)
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil || id <= 0 {
			httpx.BadRequest(c, "无效 id")
			return
		}
		ctx := c.Request.Context()
		fb, err := client.Feedback.Query().
			Where(feedback.IDEQ(id), feedback.UserIDEQ(u.ID)).
			Only(ctx)
		if err != nil {
			if ent.IsNotFound(err) {
				httpx.Err(c, http.StatusNotFound, 40400, "不存在")
				return
			}
			httpx.ServerError(c, "查询失败")
			return
		}
		httpx.OK(c, gin.H{
			"id":            fb.ID,
			"preset_key":    fb.PresetKey,
			"title":         fb.Title,
			"detail":        fb.Detail,
			"review_state":  fb.ReviewState,
			"reward_points": fb.RewardPoints,
			"created_at":    fb.CreatedAt,
			"reviewed_at":   fb.ReviewedAt,
		})
	}
}

// FeedbackThrottleMeta 返回用户当日已提交条数（用于前端提示，软限制）。
func FeedbackThrottleMeta(client *ent.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		u := c.MustGet(middleware.CtxUserKey).(*ent.User)
		ctx := c.Request.Context()
		start := time.Now().Truncate(24 * time.Hour)
		n, err := client.Feedback.Query().
			Where(
				feedback.UserIDEQ(u.ID),
				feedback.CreatedAtGTE(start),
			).
			Count(ctx)
		if err != nil {
			httpx.ServerError(c, "查询失败")
			return
		}
		httpx.OK(c, gin.H{"today_count": n, "daily_limit": feedbackDailyCap})
	}
}
