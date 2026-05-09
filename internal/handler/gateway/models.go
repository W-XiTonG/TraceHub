package gateway

import (
	"tracehub/internal/ent"
	"tracehub/internal/middleware"
	"tracehub/internal/service/availmodels"

	"github.com/gin-gonic/gin"
)

func ModelsList(client *ent.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		uAny, _ := c.Get(middleware.CtxGateUserKey)
		u := uAny.(*ent.User)
		gid := u.GroupID
		if gid <= 0 {
			middleware.WriteOpenAIError(c, 403, "permission_error", "账户未分配分组")
			return
		}
		items, err := availmodels.ListForGroup(c.Request.Context(), client, gid)
		if err != nil {
			middleware.WriteOpenAIError(c, 500, "api_error", "读取模型列表失败")
			return
		}
		data := make([]gin.H, 0, len(items))
		for _, it := range items {
			data = append(data, gin.H{
				"id":        it.ID,
				"object":    it.Object,
				"created":   it.Created,
				"owned_by":  it.OwnedBy,
			})
		}
		c.JSON(200, gin.H{"object": "list", "data": data})
	}
}
