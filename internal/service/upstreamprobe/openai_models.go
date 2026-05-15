package upstreamprobe

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

	"tracehub/internal/pkg/safehttp"

	"github.com/google/uuid"
)

// NormalizeOpenAICompatBaseURL 将 Base URL 规范为 OpenAI 兼容 …/v1 根路径。
func NormalizeOpenAICompatBaseURL(raw string) (string, error) {
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

type openAIModelsResp struct {
	Data []struct {
		ID string `json:"id"`
	} `json:"data"`
}

// ParseModelIDsFromResponseBody 解析 OpenAI 兼容 /v1/models JSON 为 id 列表。
func ParseModelIDsFromResponseBody(body []byte) ([]string, error) {
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
		return sortedKeys(set), nil
	}
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
			return sortedKeys(set), nil
		}
	}
	return []string{}, nil
}

func sortedKeys(set map[string]struct{}) []string {
	out := make([]string, 0, len(set))
	for id := range set {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

var modelsHTTPClient = &http.Client{Timeout: 45 * time.Second}

// FetchUpstreamModelIDs 请求 GET {base}/models，返回上游模型 id 列表。
func FetchUpstreamModelIDs(ctx context.Context, apiBaseURL, bearerToken string) ([]string, string, error) {
	key := strings.TrimSpace(bearerToken)
	if key == "" {
		return nil, "", fmt.Errorf("缺少凭证")
	}
	base, err := NormalizeOpenAICompatBaseURL(apiBaseURL)
	if err != nil {
		return nil, "", err
	}
	modelsURL := base + "/models"
	if err := safehttp.EgressURLHostSafe(modelsURL); err != nil {
		return nil, "", fmt.Errorf("api_base_url 主机不可访问: %w", err)
	}
	u, err := url.Parse(modelsURL)
	if err != nil {
		return nil, "", fmt.Errorf("无法拼接 models URL")
	}
	reqCtx, cancel := context.WithTimeout(ctx, 44*time.Second)
	defer cancel()
	httpReq, err := http.NewRequestWithContext(reqCtx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, "", err
	}
	httpReq.Header.Set("Authorization", "Bearer "+key)
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("User-Agent", "TraceHub/1.0 (upstream-model-probe)")
	httpReq.Header.Set("X-Request-ID", uuid.NewString())
	resp, err := modelsHTTPClient.Do(httpReq)
	if err != nil {
		return nil, modelsURL, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, modelsURL, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, modelsURL, fmt.Errorf("上游 HTTP %s", resp.Status)
	}
	ids, err := ParseModelIDsFromResponseBody(body)
	if err != nil {
		return nil, modelsURL, fmt.Errorf("解析上游 JSON: %w", err)
	}
	return ids, modelsURL, nil
}
