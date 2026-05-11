package gateway

import (
	"fmt"
	"strings"

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

// ModelsGet OpenAI 兼容 GET /v1/models/{model}：返回当前分组可用列表中的单项。
func ModelsGet(client *ent.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := strings.TrimSpace(c.Param("model_id"))
		if id == "" {
			middleware.WriteOpenAIError(c, 400, "invalid_request_error", "缺少 model id")
			return
		}
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
		for _, it := range items {
			if it.ID == id {
				c.JSON(200, gin.H{
					"id":       it.ID,
					"object":   it.Object,
					"created":  it.Created,
					"owned_by": it.OwnedBy,
				})
				return
			}
		}
		middleware.WriteOpenAIError(c, 404, "invalid_request_error", fmt.Sprintf("模型 %s 不存在或当前分组不可访问", id))
	}
}
