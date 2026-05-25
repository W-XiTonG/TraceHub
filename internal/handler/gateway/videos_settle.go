package gateway

import (
	"context"
	"encoding/json"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"tracehub/internal/ent"
	"tracehub/internal/gateway/router"
	"tracehub/internal/pkg/klingvideo"
	"tracehub/internal/pkg/volcvideo"
	"tracehub/internal/service/billing"

	"github.com/redis/go-redis/v9"
)

const videoPrechargeKeyPrefix = "tracehub:video:precharge:"
const videoJobKeyPrefix = "tracehub:video:job:" // 二期幂等：已结算标记
const videoJobMetaKeyPrefix = "tracehub:video:meta:"

const videoBillingRedisTTL = 7 * 24 * time.Hour

type videoJobMeta struct {
	Model     string `json:"model"`
	Seconds   int    `json:"seconds"`
	Size      string `json:"size"`
	Cost      int64  `json:"cost"`
	ChannelID int    `json:"channel_id,omitempty"`
	KlingKind string `json:"kling_kind,omitempty"` // text2video | image2video | extend | ...
	KlingPath string `json:"kling_path,omitempty"` // 创建时的相对路径，如 videos/effects
}

// videoSettleOutcome 精调结果；Adjust>0 补扣，Adjust<0 退还。
type videoSettleOutcome struct {
	Settled    bool
	Adjust     int64
	Precharge  int64
	FinalCost  int64
	Seconds    int
}

func videoPrechargeRedisKey(userID int64, videoID string) string {
	return videoPrechargeKeyPrefix + strconv.FormatInt(userID, 10) + ":" + videoID
}

// videoJobRedisKey 对应计划 video:job:{user_id}:{video_id}（带 tracehub 前缀）。
func videoJobRedisKey(userID int64, videoID string) string {
	return videoJobKeyPrefix + strconv.FormatInt(userID, 10) + ":" + videoID
}

func videoMetaRedisKey(userID int64, videoID string) string {
	return videoJobMetaKeyPrefix + strconv.FormatInt(userID, 10) + ":" + videoID
}

func klingKindForVideo(ctx context.Context, rdb *redis.Client, userID int64, videoID string) klingvideo.CreateKind {
	meta := loadVideoJobMeta(ctx, rdb, userID, videoID)
	return klingvideo.CreateKind(strings.TrimSpace(meta.KlingKind))
}

func klingPathForVideo(ctx context.Context, rdb *redis.Client, userID int64, videoID string) string {
	meta := loadVideoJobMeta(ctx, rdb, userID, videoID)
	return strings.TrimSpace(meta.KlingPath)
}

func loadVideoJobMeta(ctx context.Context, rdb *redis.Client, userID int64, videoID string) videoJobMeta {
	if rdb == nil || videoID == "" {
		return videoJobMeta{}
	}
	raw, err := rdb.Get(ctx, videoMetaRedisKey(userID, videoID)).Bytes()
	if err != nil || len(raw) == 0 {
		return videoJobMeta{}
	}
	var meta videoJobMeta
	if json.Unmarshal(raw, &meta) != nil {
		return videoJobMeta{}
	}
	return meta
}

func storeVideoJobBillingMeta(ctx context.Context, rdb *redis.Client, userID int64, videoID string, meta videoJobMeta) {
	if rdb == nil || videoID == "" {
		return
	}
	raw, err := json.Marshal(meta)
	if err != nil {
		return
	}
	_ = rdb.Set(ctx, videoPrechargeRedisKey(userID, videoID), meta.Cost, videoBillingRedisTTL).Err()
	_ = rdb.Set(ctx, videoMetaRedisKey(userID, videoID), raw, videoBillingRedisTTL).Err()
}

// trySettleVideoOnComplete 在 GET 任务且 status=completed 时按实际上游 usage/seconds 精调（补扣或退还）。
func trySettleVideoOnComplete(ctx context.Context, eng *billing.Engine, rdb *redis.Client, u *ent.User, model, videoID string, upstreamBody []byte) videoSettleOutcome {
	out := videoSettleOutcome{}
	if eng == nil || videoID == "" || u == nil {
		return out
	}
	status, _ := parseVideoStatusFromJSON(upstreamBody)
	if status != "completed" {
		return out
	}
	if rdb == nil {
		slog.Warn("video settle skipped: redis unavailable", "user_id", u.ID, "video_id", videoID)
		return out
	}

	precharge, err := rdb.Get(ctx, videoPrechargeRedisKey(int64(u.ID), videoID)).Int64()
	if err == redis.Nil {
		return out
	}
	if err != nil {
		slog.Warn("video settle: read precharge failed", "user_id", u.ID, "video_id", videoID, "err", err)
		return out
	}
	out.Precharge = precharge

	jobKey := videoJobRedisKey(int64(u.ID), videoID)
	acquired, err := rdb.SetNX(ctx, jobKey, "settled", videoBillingRedisTTL).Result()
	if err != nil {
		slog.Warn("video settle: setnx failed", "user_id", u.ID, "video_id", videoID, "err", err)
		return out
	}
	if !acquired {
		return out
	}

	size := parseVideoSizeFromJSON(upstreamBody)
	metaRaw, _ := rdb.Get(ctx, videoMetaRedisKey(int64(u.ID), videoID)).Bytes()
	var meta videoJobMeta
	if len(metaRaw) > 0 {
		_ = json.Unmarshal(metaRaw, &meta)
	}
	if size == "" {
		size = meta.Size
	}
	actualSec := parseVideoBillableSeconds(upstreamBody)
	if actualSec <= 0 {
		actualSec = meta.Seconds
	}
	if actualSec <= 0 {
		actualSec = billing.ParseVideoSeconds("")
	}
	out.Seconds = actualSec

	channelID := meta.ChannelID
	br, err := eng.ModelBillRates(ctx, u, model, channelID)
	if err != nil {
		_, _ = rdb.Del(ctx, jobKey).Result()
		return out
	}
	finalCost := br.VideoJobCost(model, billing.VideoJobParams{Seconds: actualSec, Size: size})
	out.FinalCost = finalCost
	diff := finalCost - precharge
	out.Adjust = diff
	if diff == 0 {
		out.Settled = true
		return out
	}
	if diff > 0 {
		if err := eng.DeductBalance(ctx, int64(u.ID), diff); err != nil {
			slog.Warn("video settle: extra deduct failed", "user_id", u.ID, "video_id", videoID, "diff", diff, "err", err)
			_, _ = rdb.Del(ctx, jobKey).Result()
			return out
		}
		out.Settled = true
		return out
	}
	if err := eng.CreditBalance(ctx, int64(u.ID), -diff); err != nil {
		slog.Warn("video settle: refund failed", "user_id", u.ID, "video_id", videoID, "refund", -diff, "err", err)
		_, _ = rdb.Del(ctx, jobKey).Result()
		return out
	}
	out.Settled = true
	return out
}

func isVideoTerminalFailureStatus(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "failed", "error", "cancelled", "canceled":
		return true
	default:
		return false
	}
}

// trySettleVideoJob 按上游 status 在 completed 精调或终端失败全额退预扣（同一 video_id 仅结算一次）。
func trySettleVideoJob(ctx context.Context, eng *billing.Engine, rdb *redis.Client, u *ent.User, model, videoID string, upstreamBody []byte) videoSettleOutcome {
	status, _ := parseVideoStatusFromJSON(upstreamBody)
	switch status {
	case "completed":
		return trySettleVideoOnComplete(ctx, eng, rdb, u, model, videoID, upstreamBody)
	default:
		if isVideoTerminalFailureStatus(status) {
			return trySettleVideoOnFailed(ctx, eng, rdb, u, model, videoID, upstreamBody)
		}
	}
	return videoSettleOutcome{}
}

// trySettleVideoOnFailed 在 GET 任务且 status 为终端失败时全额退还创建预扣。
func trySettleVideoOnFailed(ctx context.Context, eng *billing.Engine, rdb *redis.Client, u *ent.User, model, videoID string, upstreamBody []byte) videoSettleOutcome {
	if eng == nil || videoID == "" || u == nil {
		return videoSettleOutcome{}
	}
	status, _ := parseVideoStatusFromJSON(upstreamBody)
	if !isVideoTerminalFailureStatus(status) {
		return videoSettleOutcome{}
	}
	return tryRefundVideoPrecharge(ctx, eng, rdb, u, videoID)
}

// isVideoPollTerminalHTTPError 轮询 GET 返回的 HTTP 错误是否表示任务已不存在（可退预扣）。
func isVideoPollTerminalHTTPError(statusCode int, body []byte) bool {
	switch statusCode {
	case 404, 410:
		return true
	case 400:
		lower := strings.ToLower(string(body))
		return strings.Contains(lower, "not found") ||
			strings.Contains(lower, "invalid video") ||
			strings.Contains(lower, "does not exist") ||
			strings.Contains(lower, "no such") ||
			strings.Contains(lower, "unknown video")
	default:
		return false
	}
}

// tryRefundVideoPrecharge 全额退还视频创建预扣（SetNX 幂等，与 completed 精调互斥）。
func tryRefundVideoPrecharge(ctx context.Context, eng *billing.Engine, rdb *redis.Client, u *ent.User, videoID string) videoSettleOutcome {
	out := videoSettleOutcome{}
	if eng == nil || videoID == "" || u == nil {
		return out
	}
	if rdb == nil {
		slog.Warn("video failed refund skipped: redis unavailable", "user_id", u.ID, "video_id", videoID)
		return out
	}

	precharge, err := rdb.Get(ctx, videoPrechargeRedisKey(int64(u.ID), videoID)).Int64()
	if err == redis.Nil {
		return out
	}
	if err != nil {
		slog.Warn("video failed refund: read precharge failed", "user_id", u.ID, "video_id", videoID, "err", err)
		return out
	}
	if precharge <= 0 {
		return out
	}
	out.Precharge = precharge

	jobKey := videoJobRedisKey(int64(u.ID), videoID)
	acquired, err := rdb.SetNX(ctx, jobKey, "settled", videoBillingRedisTTL).Result()
	if err != nil {
		slog.Warn("video failed refund: setnx failed", "user_id", u.ID, "video_id", videoID, "err", err)
		return out
	}
	if !acquired {
		return out
	}

	metaRaw, _ := rdb.Get(ctx, videoMetaRedisKey(int64(u.ID), videoID)).Bytes()
	var meta videoJobMeta
	if len(metaRaw) > 0 {
		_ = json.Unmarshal(metaRaw, &meta)
	}
	out.Seconds = meta.Seconds
	if out.Seconds <= 0 {
		out.Seconds = billing.ParseVideoSeconds("")
	}

	if err := eng.CreditBalance(ctx, int64(u.ID), precharge); err != nil {
		slog.Warn("video failed refund: credit failed", "user_id", u.ID, "video_id", videoID, "refund", precharge, "err", err)
		_, _ = rdb.Del(ctx, jobKey).Result()
		return out
	}
	out.Adjust = -precharge
	out.FinalCost = 0
	out.Settled = true
	return out
}

func patchVideoCreateBody(body []byte, upstream string, volc bool) ([]byte, error) {
	patched, err := router.PatchModelInBody(body, upstream)
	if err != nil {
		return nil, err
	}
	if volc {
		return volcvideo.PrepareVolcVideoCreateBody(patched)
	}
	return patched, nil
}

func normalizeVideoUpstreamBody(ct string, body []byte) []byte {
	if isVolcengineVideoChannel(ct) {
		return volcvideo.NormalizeResponseBody(body)
	}
	if isKlingVideoChannel(ct) {
		return klingvideo.NormalizeResponseBody(body)
	}
	return body
}
