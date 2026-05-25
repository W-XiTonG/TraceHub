package gateway

import (
	"context"
	"log/slog"
	"strconv"

	"tracehub/internal/ent"
	"tracehub/internal/gateway/router"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

const HeaderTraceHubUpstreamStatus = "X-TraceHub-Upstream-Status"

// resolveVideoJobSelected 视频 job 后续请求优先固定到创建时的渠道；失败则 fallback。
func resolveVideoJobSelected(ctx context.Context, client *ent.Client, rdb *redis.Client, gid int, u *ent.User, model, videoID string, fallback *router.Selected) *router.Selected {
	if videoID == "" || fallback == nil {
		return fallback
	}
	meta := loadVideoJobMeta(ctx, rdb, int64(u.ID), videoID)
	if meta.ChannelID <= 0 {
		return fallback
	}
	sel, err := router.SelectChannelByID(ctx, client, gid, meta.ChannelID, model)
	if err != nil {
		slog.Warn("video job channel pin failed, fallback to model select",
			"user_id", u.ID, "video_id", videoID, "channel_id", meta.ChannelID, "err", err)
		return fallback
	}
	return sel
}

func setVideoJobTraceHeaders(c *gin.Context, channelID, upstreamStatus int) {
	if channelID > 0 {
		c.Header(HeaderTraceHubChannelID, strconv.Itoa(channelID))
	}
	if upstreamStatus > 0 {
		c.Header(HeaderTraceHubUpstreamStatus, strconv.Itoa(upstreamStatus))
	}
}
