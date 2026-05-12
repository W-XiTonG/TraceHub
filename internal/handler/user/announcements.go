package userhandler

import (
	"strconv"

	"tracehub/internal/ent"
	"tracehub/internal/ent/announcement"
	"tracehub/internal/pkg/httpx"

	"github.com/gin-gonic/gin"
)

// LatestAnnouncement 返回当前已发布最新一条（用于站内提醒）；无已发布公告时 data 为 null。
func LatestAnnouncement(client *ent.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		a, err := client.Announcement.Query().
			Where(
				announcement.PublishedEQ(true),
				announcement.PublishedAtNotNil(),
			).
			Order(ent.Desc(announcement.FieldPublishedAt), ent.Desc(announcement.FieldID)).
			First(ctx)
		if err != nil {
			if ent.IsNotFound(err) {
				httpx.OK(c, gin.H{"announcement": nil})
				return
			}
			httpx.ServerError(c, "查询失败")
			return
		}
		httpx.OK(c, gin.H{"announcement": publishedAnnouncementJSON(a)})
	}
}

// ListPublishedAnnouncements 已发布公告列表（用户侧），按发布时间倒序。
func ListPublishedAnnouncements(client *ent.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
		if limit < 1 || limit > 50 {
			limit = 20
		}
		ctx := c.Request.Context()
		list, err := client.Announcement.Query().
			Where(
				announcement.PublishedEQ(true),
				announcement.PublishedAtNotNil(),
			).
			Order(ent.Desc(announcement.FieldPublishedAt), ent.Desc(announcement.FieldID)).
			Limit(limit).
			All(ctx)
		if err != nil {
			httpx.ServerError(c, "查询失败")
			return
		}
		items := make([]gin.H, 0, len(list))
		for _, a := range list {
			items = append(items, publishedAnnouncementJSON(a))
		}
		httpx.OK(c, gin.H{"items": items})
	}
}

func publishedAnnouncementJSON(a *ent.Announcement) gin.H {
	return gin.H{
		"id":           a.ID,
		"title":        a.Title,
		"content":      a.Content,
		"published_at": a.PublishedAt,
	}
}
