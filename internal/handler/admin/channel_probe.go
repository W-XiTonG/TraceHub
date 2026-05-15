package adminhandler

import (
	"strconv"

	"tracehub/internal/ent"
	"tracehub/internal/middleware"
	"tracehub/internal/pkg/httpx"
	"tracehub/internal/service/upstreamprobe"

	"github.com/gin-gonic/gin"
)

// ProbeChannelUpstream POST /api/admin/channels/:id/probe-upstream-models
// 立即对单渠道执行上游 /v1/models 探测并写回 model_upstream_probe。
func ProbeChannelUpstream(d ChannelDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		adm := c.MustGet(middleware.CtxUserKey).(*ent.User)
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			httpx.BadRequest(c, "无效 id")
			return
		}
		ctx := c.Request.Context()
		ch, err := d.Client.Channel.Get(ctx, id)
		if err != nil {
			httpx.BadRequest(c, "不存在")
			return
		}
		if err := upstreamprobe.ProbeChannel(ctx, d.Client, d.AESKey, ch); err != nil {
			httpx.ServerError(c, "探测失败")
			return
		}
		fresh, err := d.Client.Channel.Get(ctx, id)
		if err != nil {
			httpx.ServerError(c, "读取渠道失败")
			return
		}
		auditLog(d.Client, adm.ID, "channel.probe_upstream_models", "channel", strconv.Itoa(id), nil, c)
		httpx.OK(c, gin.H{"model_upstream_probe": fresh.ModelUpstreamProbe})
	}
}
