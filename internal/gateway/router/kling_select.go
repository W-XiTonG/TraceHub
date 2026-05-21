package router

import (
	"context"
	"fmt"

	"tracehub/internal/ent"
	"tracehub/internal/pkg/channelmeta"
	"tracehub/internal/pkg/klingvideo"

	"github.com/redis/go-redis/v9"
)

// SelectKlingChannel 仅从 channel_type=kling 的渠道中选路。
func SelectKlingChannel(ctx context.Context, rdb *redis.Client, client *ent.Client, userID int64, userGroupID int, requestedModel string, stickySession string) (*Selected, error) {
	return SelectKlingChannelExcluding(ctx, rdb, client, userID, userGroupID, requestedModel, stickySession, nil)
}

// SelectKlingChannelExcluding 排除指定渠道后选 kling 渠道（跳过非 kling 的同名模型渠道）。
func SelectKlingChannelExcluding(ctx context.Context, rdb *redis.Client, client *ent.Client, userID int64, userGroupID int, requestedModel string, stickySession string, excludeIDs []int) (*Selected, error) {
	exclude := append([]int(nil), excludeIDs...)
	const maxTries = 32
	for try := 0; try < maxTries; try++ {
		sel, err := SelectChannelExcluding(ctx, rdb, client, userID, userGroupID, requestedModel, stickySession, exclude)
		if err != nil {
			return nil, fmt.Errorf("没有可用的可灵渠道支持模型 %s", requestedModel)
		}
		if channelmeta.IsKlingChannelType(sel.Channel.ChannelType) {
			return sel, nil
		}
		exclude = append(exclude, sel.Channel.ID)
	}
	return nil, fmt.Errorf("没有可用的可灵渠道支持模型 %s", requestedModel)
}

// PatchKlingModelInBody 同步 model_name/model 为上游模型名。
func PatchKlingModelInBody(body []byte, upstream string) ([]byte, error) {
	if upstream == "" || len(body) == 0 {
		return body, nil
	}
	return klingvideo.PatchModelInBody(body, upstream)
}
