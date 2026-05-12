package adminhandler

import (
	"strconv"
	"strings"
	"time"

	"tracehub/internal/ent"
	"tracehub/internal/ent/announcement"
	"tracehub/internal/middleware"
	"tracehub/internal/pkg/httpx"

	"github.com/gin-gonic/gin"
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

func CreateAnnouncement(client *ent.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		adm := c.MustGet(middleware.CtxUserKey).(*ent.User)
		var req struct {
			Title   string `json:"title"`
			Content string `json:"content"`
			Publish bool   `json:"publish"`
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
		b := client.Announcement.Create().
			SetTitle(title).
			SetContent(content).
			SetCreatedBy(adm.ID)
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
		}
		httpx.OK(c, announcementJSON(a))
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
			Title   *string `json:"title"`
			Content *string `json:"content"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			httpx.BadRequest(c, "参数无效")
			return
		}
		if req.Title == nil && req.Content == nil {
			httpx.BadRequest(c, "无更新字段")
			return
		}
		ctx := c.Request.Context()
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
		httpx.OK(c, announcementJSON(a))
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

func PublishAnnouncement(client *ent.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		adm := c.MustGet(middleware.CtxUserKey).(*ent.User)
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			httpx.BadRequest(c, "无效 id")
			return
		}
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
		httpx.OK(c, announcementJSON(a))
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
		httpx.OK(c, announcementJSON(a))
	}
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
	return h
}
