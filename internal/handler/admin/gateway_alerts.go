package adminhandler

import (
	"context"
	"strconv"
	"time"

	"tracehub/internal/ent"
	"tracehub/internal/ent/admingatewayalert"
	"tracehub/internal/ent/admingatewayalertread"
	"tracehub/internal/middleware"
	"tracehub/internal/pkg/httpx"

	"github.com/gin-gonic/gin"
)

// ListGatewayAlerts GET /api/admin/gateway-alerts
func ListGatewayAlerts(client *ent.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		adm := c.MustGet(middleware.CtxUserKey).(*ent.User)
		ctx := c.Request.Context()
		limit := 50
		if q := c.Query("limit"); q != "" {
			if n, err := strconv.Atoi(q); err == nil && n > 0 && n <= 200 {
				limit = n
			}
		}
		lastRead := loadGatewayAlertLastRead(ctx, client, adm.ID)
		rows, err := client.AdminGatewayAlert.Query().
			Order(ent.Desc(admingatewayalert.FieldCreatedAt)).
			Limit(limit).
			All(ctx)
		if err != nil {
			httpx.ServerError(c, "查询告警失败")
			return
		}
		unread, err := client.AdminGatewayAlert.Query().
			Where(admingatewayalert.CreatedAtGT(lastRead)).
			Count(ctx)
		if err != nil {
			httpx.ServerError(c, "统计未读失败")
			return
		}
		items := make([]gin.H, 0, len(rows))
		for _, r := range rows {
			items = append(items, gin.H{
				"id":          r.ID,
				"event_kind":  r.EventKind,
				"severity":    r.Severity,
				"title":       r.Title,
				"summary":     r.Summary,
				"payload":     r.PayloadJSON,
				"request_id":  r.RequestID,
				"user_id":     r.UserID,
				"status_code": r.StatusCode,
				"model":       r.Model,
				"channel_id":  r.ChannelID,
				"created_at":  r.CreatedAt,
				"unread":      r.CreatedAt.After(lastRead),
			})
		}
		httpx.OK(c, gin.H{
			"items":         items,
			"unread_count":  unread,
			"last_read_at":  lastRead,
		})
	}
}

// MarkGatewayAlertsRead POST /api/admin/gateway-alerts/mark-read
func MarkGatewayAlertsRead(client *ent.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		adm := c.MustGet(middleware.CtxUserKey).(*ent.User)
		ctx := c.Request.Context()
		now := time.Now()
		ex, err := client.AdminGatewayAlertRead.Query().
			Where(admingatewayalertread.AdminIDEQ(adm.ID)).
			Only(ctx)
		if ent.IsNotFound(err) {
			if _, err := client.AdminGatewayAlertRead.Create().
				SetAdminID(adm.ID).
				SetLastReadAt(now).
				Save(ctx); err != nil {
				httpx.ServerError(c, "更新已读失败")
				return
			}
		} else if err != nil {
			httpx.ServerError(c, "查询已读状态失败")
			return
		} else if err := client.AdminGatewayAlertRead.UpdateOne(ex).
			SetLastReadAt(now).
			Exec(ctx); err != nil {
			httpx.ServerError(c, "更新已读失败")
			return
		}
		httpx.OK(c, gin.H{"last_read_at": now})
	}
}

func loadGatewayAlertLastRead(ctx context.Context, client *ent.Client, adminID int) time.Time {
	row, err := client.AdminGatewayAlertRead.Query().
		Where(admingatewayalertread.AdminIDEQ(adminID)).
		Only(ctx)
	if err != nil || row == nil {
		return time.Time{}
	}
	return row.LastReadAt
}
