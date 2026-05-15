package adminhandler

import (
	"strconv"

	"tracehub/internal/ent"
	"tracehub/internal/gateway/router"
	"tracehub/internal/pkg/httpx"

	"github.com/gin-gonic/gin"
)

// GroupModelOverlaps GET /api/admin/groups/:id/model-overlaps
func GroupModelOverlaps(client *ent.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			httpx.BadRequest(c, "无效 id")
			return
		}
		ctx := c.Request.Context()
		g, err := client.Group.Get(ctx, id)
		if err != nil {
			httpx.BadRequest(c, "分组不存在")
			return
		}
		items, err := router.GroupModelChannelOverlap(ctx, client, id, g.ModelRouting)
		if err != nil {
			httpx.ServerError(c, err.Error())
			return
		}
		if items == nil {
			items = []router.ModelOverlapItem{}
		}
		httpx.OK(c, gin.H{"items": items})
	}
}
