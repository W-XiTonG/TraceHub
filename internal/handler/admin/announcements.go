package adminhandler

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"tracehub/internal/ent"
	"tracehub/internal/ent/announcement"
	"tracehub/internal/middleware"
	"tracehub/internal/pkg/announcementaudience"
	"tracehub/internal/pkg/config"
	"tracehub/internal/pkg/httpx"
	"tracehub/internal/service/announcementnotify"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

const (
	audienceAll      = "all"
	audienceSpecific = "specific"
)

func ListAnnouncements(client *ent.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
		ps, _ := strconv.Atoi(c.DefaultQuery("page_size", "30"))
		if page < 1 {
			page = 1
		}
		if ps < 1 || ps > 100 {
			ps = 30
		}
		ctx := c.Request.Context()
		q := client.Announcement.Query()
		if v := c.Query("published"); v == "true" {
			q = q.Where(announcement.PublishedEQ(true))
		} else if v == "false" {
			q = q.Where(announcement.PublishedEQ(false))
		}
		total, err := q.Clone().Count(ctx)
		if err != nil {
			httpx.ServerError(c, "查询失败")
			return
		}
		list, err := q.Order(ent.Desc(announcement.FieldID)).
			Offset((page - 1) * ps).
			Limit(ps).
			All(ctx)
		if err != nil {
			httpx.ServerError(c, "查询失败")
			return
		}
		items := make([]gin.H, 0, len(list))
		for _, a := range list {
			items = append(items, announcementJSON(a))
		}
		httpx.OK(c, gin.H{"items": items, "total": total, "page": page, "page_size": ps})
	}
}

func CreateAnnouncement(client *ent.Client, cfg *config.Config, log *zap.Logger) gin.HandlerFunc {
	notifier := announcementnotify.New(cfg, client, log)
	return func(c *gin.Context) {
		adm := c.MustGet(middleware.CtxUserKey).(*ent.User)
		var req struct {
			Title           string `json:"title"`
			Content         string `json:"content"`
			Publish         bool   `json:"publish"`
			Audience        string `json:"audience"`
			AudienceUserIDs []int  `json:"audience_user_ids"`
			NotifyEmail     bool   `json:"notify_email"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			httpx.BadRequest(c, "参数无效")
			return
		}
		title := strings.TrimSpace(req.Title)
		content := strings.TrimSpace(req.Content)
		if title == "" || content == "" {
			httpx.BadRequest(c, "标题与正文不能为空")
			return
		}
		if len(title) > 500 {
			httpx.BadRequest(c, "标题过长")
			return
		}
		ctx := c.Request.Context()
		audience := strings.TrimSpace(req.Audience)
		if audience == "" {
			audience = audienceAll
		}
		if audience != audienceAll && audience != audienceSpecific {
			httpx.BadRequest(c, "无效的可见范围")
			return
		}
		if audience == audienceSpecific {
			if err := announcementaudience.ValidateAudienceIDs(ctx, client, req.AudienceUserIDs); err != nil {
				httpx.BadRequest(c, err.Error())
				return
			}
		}
		b := client.Announcement.Create().
			SetTitle(title).
			SetContent(content).
			SetCreatedBy(adm.ID)
		if err := applyAudienceMutation(b, audience, req.AudienceUserIDs); err != nil {
			httpx.BadRequest(c, err.Error())
			return
		}
		var emailResult announcementnotify.Result
		if req.Publish {
			now := time.Now()
			b = b.SetPublished(true).SetPublishedAt(now)
		}
		a, err := b.Save(ctx)
		if err != nil {
			httpx.ServerError(c, "创建失败")
			return
		}
		auditLog(client, adm.ID, "announcement.create", "announcement", strconv.Itoa(a.ID), nil, c)
		if req.Publish {
			auditLog(client, adm.ID, "announcement.publish", "announcement", strconv.Itoa(a.ID), nil, c)
			emailResult = notifier.NotifyOnPublish(ctx, a, req.NotifyEmail)
			if req.NotifyEmail {
				auditLog(client, adm.ID, "announcement.email_notify", "announcement", strconv.Itoa(a.ID), emailNotifyDetail(emailResult), c)
			}
		}
		httpx.OK(c, announcementResponse(a, emailResult))
	}
}

func UpdateAnnouncement(client *ent.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		adm := c.MustGet(middleware.CtxUserKey).(*ent.User)
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			httpx.BadRequest(c, "无效 id")
			return
		}
		var req struct {
			Title           *string `json:"title"`
			Content         *string `json:"content"`
			Audience        *string `json:"audience"`
			AudienceUserIDs []int   `json:"audience_user_ids"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			httpx.BadRequest(c, "参数无效")
			return
		}
		if req.Title == nil && req.Content == nil && req.Audience == nil && req.AudienceUserIDs == nil {
			httpx.BadRequest(c, "无更新字段")
			return
		}
		ctx := c.Request.Context()
		existing, err := client.Announcement.Get(ctx, id)
		if err != nil {
			if ent.IsNotFound(err) {
				httpx.BadRequest(c, "公告不存在")
				return
			}
			httpx.ServerError(c, "查询失败")
			return
		}
		if existing.Published && (req.Audience != nil || req.AudienceUserIDs != nil) {
			httpx.BadRequest(c, "已发布公告不可修改可见范围")
			return
		}
		up := client.Announcement.UpdateOneID(id)
		if req.Title != nil {
			t := strings.TrimSpace(*req.Title)
			if t == "" {
				httpx.BadRequest(c, "标题不能为空")
				return
			}
			if len(t) > 500 {
				httpx.BadRequest(c, "标题过长")
				return
			}
			up = up.SetTitle(t)
		}
		if req.Content != nil {
			co := strings.TrimSpace(*req.Content)
			if co == "" {
				httpx.BadRequest(c, "正文不能为空")
				return
			}
			up = up.SetContent(co)
		}
		if req.Audience != nil || req.AudienceUserIDs != nil {
			audience := audienceAll
			if req.Audience != nil {
				audience = strings.TrimSpace(*req.Audience)
			} else if len(existing.AudienceUserIds) > 0 || existing.AudienceUserID != nil {
				audience = audienceSpecific
			}
			if audience != audienceAll && audience != audienceSpecific {
				httpx.BadRequest(c, "无效的可见范围")
				return
			}
			ids := req.AudienceUserIDs
			if ids == nil {
				ids = existing.AudienceUserIds
			}
			if audience == audienceSpecific {
				if err := announcementaudience.ValidateAudienceIDs(ctx, client, ids); err != nil {
					httpx.BadRequest(c, err.Error())
					return
				}
			}
			if err := applyAudienceUpdate(up, audience, ids); err != nil {
				httpx.BadRequest(c, err.Error())
				return
			}
		}
		a, err := up.Save(ctx)
		if err != nil {
			if ent.IsNotFound(err) {
				httpx.BadRequest(c, "公告不存在")
				return
			}
			httpx.ServerError(c, "更新失败")
			return
		}
		auditLog(client, adm.ID, "announcement.update", "announcement", strconv.Itoa(id), nil, c)
		httpx.OK(c, announcementResponse(a, announcementnotify.Result{}))
	}
}

func DeleteAnnouncement(client *ent.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		adm := c.MustGet(middleware.CtxUserKey).(*ent.User)
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			httpx.BadRequest(c, "无效 id")
			return
		}
		ctx := c.Request.Context()
		if err := client.Announcement.DeleteOneID(id).Exec(ctx); err != nil {
			if ent.IsNotFound(err) {
				httpx.BadRequest(c, "公告不存在")
				return
			}
			httpx.ServerError(c, "删除失败")
			return
		}
		auditLog(client, adm.ID, "announcement.delete", "announcement", strconv.Itoa(id), nil, c)
		httpx.OKMsg(c, "已删除", nil)
	}
}

func PublishAnnouncement(client *ent.Client, cfg *config.Config, log *zap.Logger) gin.HandlerFunc {
	notifier := announcementnotify.New(cfg, client, log)
	return func(c *gin.Context) {
		adm := c.MustGet(middleware.CtxUserKey).(*ent.User)
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			httpx.BadRequest(c, "无效 id")
			return
		}
		var req struct {
			NotifyEmail bool `json:"notify_email"`
		}
		_ = c.ShouldBindJSON(&req)
		ctx := c.Request.Context()
		now := time.Now()
		a, err := client.Announcement.UpdateOneID(id).
			SetPublished(true).
			SetPublishedAt(now).
			Save(ctx)
		if err != nil {
			if ent.IsNotFound(err) {
				httpx.BadRequest(c, "公告不存在")
				return
			}
			httpx.ServerError(c, "发布失败")
			return
		}
		auditLog(client, adm.ID, "announcement.publish", "announcement", strconv.Itoa(id), nil, c)
		emailResult := notifier.NotifyOnPublish(ctx, a, req.NotifyEmail)
		if req.NotifyEmail {
			auditLog(client, adm.ID, "announcement.email_notify", "announcement", strconv.Itoa(id), emailNotifyDetail(emailResult), c)
		}
		httpx.OK(c, announcementResponse(a, emailResult))
	}
}

func UnpublishAnnouncement(client *ent.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		adm := c.MustGet(middleware.CtxUserKey).(*ent.User)
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			httpx.BadRequest(c, "无效 id")
			return
		}
		ctx := c.Request.Context()
		a, err := client.Announcement.UpdateOneID(id).
			SetPublished(false).
			ClearPublishedAt().
			Save(ctx)
		if err != nil {
			if ent.IsNotFound(err) {
				httpx.BadRequest(c, "公告不存在")
				return
			}
			httpx.ServerError(c, "操作失败")
			return
		}
		auditLog(client, adm.ID, "announcement.unpublish", "announcement", strconv.Itoa(id), nil, c)
		httpx.OK(c, announcementResponse(a, announcementnotify.Result{}))
	}
}

func applyAudienceMutation(b *ent.AnnouncementCreate, audience string, ids []int) error {
	if audience == audienceAll {
		return nil
	}
	norm := announcementaudience.NormalizeIDs(ids)
	if len(norm) == 0 {
		return fmt.Errorf("指定用户时至少选择一位用户")
	}
	b.SetAudienceUserIds(norm)
	return nil
}

func applyAudienceUpdate(up *ent.AnnouncementUpdateOne, audience string, ids []int) error {
	up.ClearAudienceUserID()
	if audience == audienceAll {
		up.ClearAudienceUserIds()
		return nil
	}
	norm := announcementaudience.NormalizeIDs(ids)
	if len(norm) == 0 {
		return fmt.Errorf("指定用户时至少选择一位用户")
	}
	up.SetAudienceUserIds(norm)
	return nil
}

func announcementJSON(a *ent.Announcement) gin.H {
	h := gin.H{
		"id":         a.ID,
		"title":      a.Title,
		"content":    a.Content,
		"published":  a.Published,
		"created_at": a.CreatedAt,
		"updated_at": a.UpdatedAt,
	}
	if a.CreatedBy != 0 {
		h["created_by"] = a.CreatedBy
	}
	if a.PublishedAt != nil {
		h["published_at"] = *a.PublishedAt
	}
	if a.AudienceUserID != nil {
		h["audience_user_id"] = *a.AudienceUserID
	}
	if len(a.AudienceUserIds) > 0 {
		h["audience_user_ids"] = a.AudienceUserIds
	}
	if announcementaudience.IsGlobal(a) {
		h["audience"] = audienceAll
	} else if len(a.AudienceUserIds) > 0 {
		h["audience"] = audienceSpecific
	}
	return h
}

func announcementResponse(a *ent.Announcement, email announcementnotify.Result) gin.H {
	h := announcementJSON(a)
	if email.Requested {
		h["email_notify"] = email
	}
	return h
}

func emailNotifyDetail(r announcementnotify.Result) map[string]any {
	d := map[string]any{
		"requested": r.Requested,
	}
	if r.SkippedReason != "" {
		d["skipped_reason"] = r.SkippedReason
	}
	if r.RecipientCount > 0 {
		d["recipient_count"] = r.RecipientCount
	}
	if r.Sent > 0 {
		d["sent"] = r.Sent
	}
	if r.Failed > 0 {
		d["failed"] = r.Failed
	}
	return d
}
