package gateway

import (
	"sort"
	"strings"
	"time"

	"tracehub/internal/ent"
	"tracehub/internal/ent/channel"
	"tracehub/internal/ent/group"
	"tracehub/internal/middleware"

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
		channels, err := client.Channel.Query().
			Where(
				channel.Enabled(true),
				channel.Schedulable(true),
				channel.HealthStatusEQ(1),
				channel.HasGroupWith(group.IDEQ(gid)),
			).
			All(c.Request.Context())
		if err != nil {
			middleware.WriteOpenAIError(c, 500, "api_error", "读取模型列表失败")
			return
		}
		set := map[string]struct{}{}
		for _, ch := range channels {
			for _, m := range ch.Models {
				set[m] = struct{}{}
			}
			for alias := range ch.ModelAlias {
				set[alias] = struct{}{}
			}
		}
		var ids []string
		for m := range set {
			ids = append(ids, m)
		}
		sort.Strings(ids)
		ts := time.Now().Unix()
		var data []gin.H
		for _, id := range ids {
			owner := "tracehub"
			if strings.HasPrefix(id, "gpt") || strings.Contains(id, "gpt") {
				owner = "openai"
			} else if strings.HasPrefix(id, "claude") {
				owner = "anthropic"
			} else if strings.HasPrefix(id, "gemini") {
				owner = "google"
			} else if strings.HasPrefix(id, "deepseek") {
				owner = "deepseek"
			}
			data = append(data, gin.H{
				"id":       id,
				"object":   "model",
				"created":  ts,
				"owned_by": owner,
			})
		}
		c.JSON(200, gin.H{"object": "list", "data": data})
	}
}
