package adminhandler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"tracehub/internal/ent"
	"tracehub/internal/ent/modelcatalogentry"
	"tracehub/internal/pkg/httpx"
	"tracehub/internal/service/billing"

	"github.com/gin-gonic/gin"
)

// modelMatchesChannelType 判断内置模型 ID 是否大致属于某渠道类型（用于推荐列表过滤）。
func modelMatchesChannelType(modelID, channelType string) bool {
	m := strings.ToLower(strings.TrimSpace(modelID))
	ct := strings.ToLower(strings.TrimSpace(channelType))
	switch ct {
	case "openai":
		return strings.HasPrefix(m, "gpt") || strings.HasPrefix(m, "o1") ||
			strings.HasPrefix(m, "o3") || strings.Contains(m, "embedding")
	case "deepseek":
		return strings.HasPrefix(m, "deepseek")
	case "claude":
		return strings.HasPrefix(m, "claude")
	case "gemini":
		return strings.HasPrefix(m, "gemini")
	default:
		return true
	}
}

func builtInModelIDsForChannelType(channelType string) []string {
	var out []string
	for _, id := range billing.DefaultModelIDs() {
		if modelMatchesChannelType(id, channelType) {
			out = append(out, id)
		}
	}
	return out
}

func catalogModelIDsForChannelType(ctx context.Context, client *ent.Client, channelType string) ([]string, error) {
	ct := strings.TrimSpace(strings.ToLower(channelType))
	q := client.ModelCatalogEntry.Query().
		Where(
			modelcatalogentry.Visible(true),
			modelcatalogentry.Enabled(true),
		)
	// 目录里 provider_slug 与渠道类型一致，或留空表示通用，都纳入推荐
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

// ChannelModelPresets GET /api/admin/channel-model-presets?channel_type=openai
// 返回内置推荐、模型目录、合并列表，供管理后台批量填入渠道模型列表。
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
		httpx.OK(c, gin.H{
			"channel_type": ct,
			"built_in":     builtIn,
			"catalog":      catalog,
			"merged":       merged,
		})
	}
}

type openAIModelsResp struct {
	Data []struct {
		ID string `json:"id"`
	} `json:"data"`
}

var upstreamModelsHTTP = &http.Client{
	Timeout: 60 * time.Second,
}

// normalizeOpenAICompatBaseURL 将「仅域名」或缺少 /v1 的 Base URL 规范为 OpenAI 兼容根路径 …/v1，
// 避免误请求 https://api.openai.com/models（404）而非 …/v1/models。
func normalizeOpenAICompatBaseURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "https://api.openai.com/v1", nil
	}
	raw = strings.TrimRight(raw, "/")
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return "", fmt.Errorf("invalid api_base_url")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", fmt.Errorf("api_base_url 仅支持 http/https")
	}
	path := strings.TrimSuffix(u.Path, "/")
	if path == "" || path == "/" {
		u.Path = "/v1"
	} else if !strings.HasSuffix(path, "/v1") {
		u.Path = path + "/v1"
	} else {
		u.Path = path
	}
	u.RawQuery = ""
	u.Fragment = ""
	return strings.TrimRight(u.String(), "/"), nil
}

func extractModelIDsFromBody(body []byte) ([]string, error) {
	var parsed openAIModelsResp
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, err
	}
	set := make(map[string]struct{})
	for _, d := range parsed.Data {
		id := strings.TrimSpace(d.ID)
		if id != "" {
			set[id] = struct{}{}
		}
	}
	if len(set) > 0 {
		out := make([]string, 0, len(set))
		for id := range set {
			out = append(out, id)
		}
		sort.Strings(out)
		return out, nil
	}
	// 少数中转返回非标准字段时，尝试从顶层 JSON 收集含 id 的对象数组
	var generic map[string]json.RawMessage
	if err := json.Unmarshal(body, &generic); err != nil {
		return nil, err
	}
	for _, key := range []string{"data", "models", "items"} {
		rawList, ok := generic[key]
		if !ok {
			continue
		}
		var arr []map[string]interface{}
		if err := json.Unmarshal(rawList, &arr); err != nil {
			continue
		}
		for _, obj := range arr {
			if id, ok := obj["id"].(string); ok && strings.TrimSpace(id) != "" {
				set[strings.TrimSpace(id)] = struct{}{}
			}
		}
		if len(set) > 0 {
			out := make([]string, 0, len(set))
			for id := range set {
				out = append(out, id)
			}
			sort.Strings(out)
			return out, nil
		}
	}
	return []string{}, nil
}

// FetchUpstreamModelList POST /api/admin/channel-models/fetch-upstream
// 使用请求体内的 api_key 调用上游 OpenAI 兼容 GET /v1/models，返回 id 列表（不落库、不记录密钥）。
func FetchUpstreamModelList() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			APIBaseURL string `json:"api_base_url"`
			APIKey     string `json:"api_key"`
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
		base, err := normalizeOpenAICompatBaseURL(req.APIBaseURL)
		if err != nil {
			httpx.BadRequest(c, err.Error())
			return
		}
		modelsURL := base + "/models"
		u, err := url.Parse(modelsURL)
		if err != nil {
			httpx.BadRequest(c, "无法拼接 models URL")
			return
		}
		ctx, cancel := context.WithTimeout(c.Request.Context(), 55*time.Second)
		defer cancel()
		httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
		if err != nil {
			httpx.ServerError(c, "构建请求失败")
			return
		}
		httpReq.Header.Set("Authorization", "Bearer "+key)
		httpReq.Header.Set("Accept", "application/json")
		httpReq.Header.Set("User-Agent", "TraceHub/1.0 (admin-fetch-models)")
		resp, err := upstreamModelsHTTP.Do(httpReq)
		if err != nil {
			httpx.Err(c, http.StatusBadGateway, 50201, "连接上游失败（请检查网络、代理与 api_base_url）: "+err.Error())
			return
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			httpx.ServerError(c, "读取上游响应失败")
			return
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			msg := fmt.Sprintf("上游 HTTP %s，请求 URL: %s。响应片段: %s", resp.Status, u.String(), string(truncateBody(body, 400)))
			httpx.Err(c, http.StatusBadGateway, 50202, msg)
			return
		}
		ids, err := extractModelIDsFromBody(body)
		if err != nil {
			httpx.Err(c, http.StatusBadGateway, 50203, "解析上游 JSON 失败: "+err.Error()+"。请确认该地址为 OpenAI 兼容 /v1/models 接口")
			return
		}
		httpx.OK(c, gin.H{
			"model_ids": ids,
			"count":     len(ids),
			"resolved_base_url": base,
			"fetched_url":       modelsURL,
		})
	}
}

func truncateBody(b []byte, max int) []byte {
	if len(b) <= max {
		return b
	}
	return b[:max]
}
