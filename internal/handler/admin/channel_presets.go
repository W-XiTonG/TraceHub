package adminhandler

import (
	"context"
	"net/http"
	"sort"
	"strings"
	"time"

	"tracehub/internal/ent"
	"tracehub/internal/ent/modelcatalogentry"
	"tracehub/internal/pkg/channelmeta"
	"tracehub/internal/pkg/httpx"
	"tracehub/internal/service/upstreamprobe"

	"github.com/gin-gonic/gin"
)

func builtInModelIDsForChannelType(channelType string) []string {
	return channelmeta.BuiltInPresetModels(channelType)
}

func catalogModelIDsForChannelType(ctx context.Context, client *ent.Client, channelType string) ([]string, error) {
	ct := strings.TrimSpace(strings.ToLower(channelType))
	q := client.ModelCatalogEntry.Query().
		Where(
			modelcatalogentry.Visible(true),
			modelcatalogentry.Enabled(true),
		)
	rows, err := q.All(ctx)
	if err != nil {
		return nil, err
	}
	var ids []string
	for _, row := range rows {
		id := strings.TrimSpace(row.ModelID)
		if id == "" {
			continue
		}
		ps := strings.TrimSpace(strings.ToLower(row.ProviderSlug))
		if ps == "" || ps == ct {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	return ids, nil
}

func mergeUniqueSorted(a, b []string) []string {
	set := make(map[string]struct{})
	for _, x := range a {
		x = strings.TrimSpace(x)
		if x != "" {
			set[x] = struct{}{}
		}
	}
	for _, x := range b {
		x = strings.TrimSpace(x)
		if x != "" {
			set[x] = struct{}{}
		}
	}
	out := make([]string, 0, len(set))
	for x := range set {
		out = append(out, x)
	}
	sort.Strings(out)
	return out
}

func metaJSON(m channelmeta.Meta) gin.H {
	return gin.H{
		"channel_type":     m.ChannelType,
		"display_name":     m.DisplayName,
		"default_base_url": m.DefaultBaseURL,
		"capabilities":     m.Capabilities,
		"video_protocol":   m.VideoProtocol,
		"admin_hints":      m.AdminHints,
	}
}

// ChannelTypes GET /api/admin/channel-types — 全部已注册渠道类型元数据。
func ChannelTypes() gin.HandlerFunc {
	return func(c *gin.Context) {
		all := channelmeta.All()
		items := make([]gin.H, 0, len(all))
		for _, m := range all {
			items = append(items, metaJSON(m))
		}
		httpx.OK(c, gin.H{"items": items})
	}
}

// ChannelModelPresets GET /api/admin/channel-model-presets?channel_type=openai
// 返回内置推荐、模型目录、合并列表及渠道类型元数据，供管理后台批量填入渠道模型列表。
func ChannelModelPresets(client *ent.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		ct := c.Query("channel_type")
		if strings.TrimSpace(ct) == "" {
			httpx.BadRequest(c, "缺少 channel_type")
			return
		}
		ctx := c.Request.Context()
		builtIn := builtInModelIDsForChannelType(ct)
		catalog, err := catalogModelIDsForChannelType(ctx, client, ct)
		if err != nil {
			httpx.ServerError(c, "读取模型目录失败")
			return
		}
		merged := mergeUniqueSorted(builtIn, catalog)
		resp := gin.H{
			"channel_type":     ct,
			"built_in":         builtIn,
			"catalog":          catalog,
			"merged":           merged,
			"default_base_url": channelmeta.DefaultBaseURL(ct),
		}
		if m, ok := channelmeta.Lookup(ct); ok {
			resp["meta"] = metaJSON(m)
			resp["display_name"] = m.DisplayName
			resp["capabilities"] = m.Capabilities
			resp["video_protocol"] = m.VideoProtocol
			resp["admin_hints"] = m.AdminHints
		}
		httpx.OK(c, resp)
	}
}

// FetchUpstreamModelList POST /api/admin/channel-models/fetch-upstream
// 使用请求体内的 api_key 调用上游 OpenAI 兼容 GET /v1/models，返回 id 列表（不落库、不记录密钥）。
func FetchUpstreamModelList() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			APIBaseURL  string `json:"api_base_url"`
			APIKey      string `json:"api_key"`
			ChannelType string `json:"channel_type"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			httpx.BadRequest(c, "参数无效")
			return
		}
		key := strings.TrimSpace(req.APIKey)
		if key == "" {
			httpx.BadRequest(c, "api_key 必填")
			return
		}
		baseURL := strings.TrimSpace(req.APIBaseURL)
		ctx, cancel := context.WithTimeout(c.Request.Context(), 55*time.Second)
		defer cancel()
		ids, fetchedURL, err := upstreamprobe.FetchUpstreamModelIDs(ctx, req.ChannelType, baseURL, key)
		if err != nil {
			msg := err.Error()
			if strings.Contains(msg, "invalid api_base_url") ||
				strings.Contains(msg, "api_base_url 仅支持") ||
				strings.Contains(msg, "主机不可访问") ||
				strings.Contains(msg, "无法拼接 models URL") ||
				strings.Contains(msg, "缺少凭证") {
				httpx.BadRequest(c, msg)
				return
			}
			if strings.Contains(msg, "上游 HTTP") {
				httpx.Err(c, http.StatusBadGateway, 50202, msg)
				return
			}
			if strings.Contains(msg, "解析上游 JSON") {
				httpx.Err(c, http.StatusBadGateway, 50203, msg+"。请确认该地址为 OpenAI 兼容 /v1/models 接口")
				return
			}
			httpx.Err(c, http.StatusBadGateway, 50201, "连接上游失败（请检查网络、代理与 api_base_url）: "+msg)
			return
		}
		resolved, _ := upstreamprobe.ResolveModelsListBaseURL(req.ChannelType, baseURL)
		httpx.OK(c, gin.H{
			"model_ids":         ids,
			"count":             len(ids),
			"resolved_base_url": resolved,
			"fetched_url":       fetchedURL,
			"default_base_url":  channelmeta.DefaultBaseURL(req.ChannelType),
		})
	}
}
