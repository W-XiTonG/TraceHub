package volcvideo

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
)

// PatchCreateRequestBody 将请求转为火山方舟 POST /contents/generations/tasks 请求体。
func PatchCreateRequestBody(openAIBody []byte) ([]byte, error) {
	return PrepareVolcVideoCreateBody(openAIBody)
}

// PrepareVolcVideoCreateBody 已是方舟原生 content[] 时透传并补全字段；否则由 OpenAI Video 形态转换。
// seed3d / 3dgen 在 OpenAI 形态（prompt / image_url 等）时转为 model + content[]。
func PrepareVolcVideoCreateBody(body []byte) ([]byte, error) {
	var m map[string]any
	if err := json.Unmarshal(body, &m); err != nil {
		return nil, err
	}
	if isArkNativeVideoCreate(m) {
		return json.Marshal(mergeArkNativeCreateFields(m))
	}
	if model, _ := m["model"].(string); IsVolc3DGenModel(model) {
		return BuildArk3DCreateBody(body)
	}
	return BuildArkCreateBody(body)
}

func isArkNativeVideoCreate(m map[string]any) bool {
	content, ok := m["content"].([]any)
	return ok && len(content) > 0
}

// IsVolc3DGenModel 方舟 3D 生成（seed3d / 3dgen）；OpenAI 形态请求会转为 model + content[]。
func IsVolc3DGenModel(model string) bool {
	m := strings.ToLower(strings.TrimSpace(model))
	return strings.Contains(m, "seed3d") || strings.Contains(m, "3dgen")
}

// ImageDataFromBytes 将图片字节编码为方舟 seed3d 可用的 image_data（data URL base64）。
func ImageDataFromBytes(mime string, raw []byte) string {
	mime = strings.TrimSpace(mime)
	if mime == "" {
		mime = "image/png"
	}
	return "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(raw)
}

// mergeArkNativeCreateFields 在透传方舟原生 body 时合并 seconds→duration、size→ratio 等兼容字段（不修改用户已填数值）。
func mergeArkNativeCreateFields(m map[string]any) map[string]any {
	out := make(map[string]any, len(m)+4)
	for k, v := range m {
		out[k] = v
	}
	model, _ := out["model"].(string)
	if !IsVolc3DGenModel(model) {
		if out["duration"] == nil {
			if v, ok := m["seconds"]; ok {
				out["duration"] = parseSecondsValue(v)
			}
		}
		if _, hasRatio := out["ratio"]; !hasRatio {
			if size, ok := m["size"].(string); ok && strings.TrimSpace(size) != "" {
				_, ratio := SizeToVolcResolutionRatio(size)
				if ratio != "" {
					out["ratio"] = ratio
				}
			}
		}
		if _, hasRes := out["resolution"]; !hasRes {
			if size, ok := m["size"].(string); ok && strings.TrimSpace(size) != "" {
				res, _ := SizeToVolcResolutionRatio(size)
				if res != "" {
					out["resolution"] = res
				}
			}
		}
	}
	return out
}

// BuildArkCreateBody OpenAI Video JSON（prompt/seconds/size）→ 方舟 tasks 请求体。
func BuildArkCreateBody(openAIBody []byte) ([]byte, error) {
	var m map[string]any
	if err := json.Unmarshal(openAIBody, &m); err != nil {
		return nil, err
	}
	model, _ := m["model"].(string)
	model = strings.TrimSpace(model)
	if model == "" {
		return nil, errors.New("missing model")
	}
	prompt, _ := m["prompt"].(string)
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		return nil, errors.New("missing prompt")
	}
	content := []map[string]any{{"type": "text", "text": prompt}}
	if img := extractImageURLFromOpenAI(m); img != "" {
		content = append(content, map[string]any{
			"type": "image_url",
			"image_url": map[string]any{
				"url": img,
			},
			"role": "first_frame",
		})
	}
	out := map[string]any{
		"model":   model,
		"content": content,
	}
	if !IsVolc3DGenModel(model) {
		if v, ok := m["seconds"]; ok {
			out["duration"] = parseSecondsValue(v)
		}
		if v, ok := m["duration"]; ok && out["duration"] == nil {
			out["duration"] = parseSecondsValue(v)
		}
	}
	if !IsVolc3DGenModel(model) {
		if size, ok := m["size"].(string); ok && strings.TrimSpace(size) != "" {
			res, ratio := SizeToVolcResolutionRatio(size)
			if res != "" {
				out["resolution"] = res
			}
			if ratio != "" {
				out["ratio"] = ratio
			}
		}
	}
	copyArkOptionalFields(m, out)
	return json.Marshal(out)
}

// BuildArk3DCreateBody 将请求转为 3dgen 要求的 model + content[]（须含 text 与 image_url，url 可为公网链或 data URL base64）。
func BuildArk3DCreateBody(body []byte) ([]byte, error) {
	var m map[string]any
	if err := json.Unmarshal(body, &m); err != nil {
		return nil, err
	}
	model, _ := m["model"].(string)
	model = strings.TrimSpace(model)
	if model == "" {
		return nil, errors.New("missing model")
	}
	prompt, _ := m["prompt"].(string)
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		prompt = extractPromptFromArkContent(m)
	}
	imageURL := extractImageDataFor3D(m)
	if imageURL == "" {
		return nil, errors.New("seed3d 需要参考图：请上传 input_reference、提供 image_url / image_data，或在 content 中传入 image_url")
	}
	if prompt == "" {
		prompt = "generate 3d"
	}
	content := []map[string]any{
		{"type": "text", "text": prompt},
		{
			"type": "image_url",
			"image_url": map[string]any{
				"url": imageURL,
			},
		},
	}
	return json.Marshal(map[string]any{
		"model":   model,
		"content": content,
	})
}

// copyArkOptionalFields 透传方舟扩展字段（Seedance 2.0 等多模态参数）。
func copyArkOptionalFields(src, dst map[string]any) {
	for _, k := range []string{
		"generate_audio", "watermark", "ratio", "resolution", "duration",
		"content", "callback_url",
	} {
		if k == "content" {
			continue
		}
		if v, ok := src[k]; ok && dst[k] == nil {
			dst[k] = v
		}
	}
	if _, hasContent := dst["content"]; hasContent {
		return
	}
	if c, ok := src["content"]; ok {
		dst["content"] = c
	}
}

func extractImageDataFor3D(m map[string]any) string {
	if s, ok := m["image_data"].(string); ok && strings.TrimSpace(s) != "" {
		return strings.TrimSpace(s)
	}
	if u := extractImageURLFromOpenAI(m); u != "" {
		return u
	}
	return extractImageDataFromArkContent(m)
}

func extractPromptFromArkContent(m map[string]any) string {
	content, ok := m["content"].([]any)
	if !ok {
		return ""
	}
	for _, item := range content {
		im, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if strings.TrimSpace(stringFieldAny(im, "type")) != "text" {
			continue
		}
		if t := strings.TrimSpace(stringFieldAny(im, "text")); t != "" {
			return t
		}
	}
	return ""
}

func extractImageDataFromArkContent(m map[string]any) string {
	content, ok := m["content"].([]any)
	if !ok {
		return ""
	}
	for _, item := range content {
		im, ok := item.(map[string]any)
		if !ok {
			continue
		}
		typ := strings.ToLower(strings.TrimSpace(stringFieldAny(im, "type")))
		if typ != "image_url" && typ != "image" {
			continue
		}
		if iu, ok := im["image_url"].(map[string]any); ok {
			if u, ok := iu["url"].(string); ok && strings.TrimSpace(u) != "" {
				return strings.TrimSpace(u)
			}
		}
		if u := stringFieldAny(im, "url"); u != "" {
			return u
		}
	}
	return ""
}

func stringFieldAny(m map[string]any, k string) string {
	v, _ := m[k].(string)
	return strings.TrimSpace(v)
}

func extractImageURLFromOpenAI(m map[string]any) string {
	if u, ok := m["image_url"].(string); ok && strings.TrimSpace(u) != "" {
		return strings.TrimSpace(u)
	}
	ref, ok := m["input_reference"]
	if !ok || ref == nil {
		return ""
	}
	switch t := ref.(type) {
	case string:
		return strings.TrimSpace(t)
	case map[string]any:
		if u, ok := t["url"].(string); ok {
			return strings.TrimSpace(u)
		}
	}
	return ""
}

// ExtractVideoURLFromTaskBody 从 GET …/contents/generations/tasks/{id} 响应取下载地址。
func ExtractVideoURLFromTaskBody(body []byte) (string, bool) {
	var m map[string]any
	if err := json.Unmarshal(body, &m); err != nil {
		return "", false
	}
	if c, ok := m["content"].(map[string]any); ok {
		if u, ok := c["video_url"].(string); ok && strings.TrimSpace(u) != "" {
			return strings.TrimSpace(u), true
		}
	}
	if u, ok := m["video_url"].(string); ok && strings.TrimSpace(u) != "" {
		return strings.TrimSpace(u), true
	}
	return "", false
}

// NormalizeResponseBody 将方舟响应尽量规范为 OpenAI Video 对象字段。
func NormalizeResponseBody(upstreamBody []byte) []byte {
	var m map[string]any
	if err := json.Unmarshal(upstreamBody, &m); err != nil {
		return upstreamBody
	}
	if id, ok := stringField(m, "id", "task_id", "video_id"); ok && m["id"] == nil {
		m["id"] = id
	}
	if st, ok := stringField(m, "status", "state"); ok {
		m["status"] = mapVolcStatus(st)
	}
	if c, ok := m["content"].(map[string]any); ok {
		if d, ok := c["duration"]; ok && m["duration"] == nil {
			m["duration"] = d
		}
		if u, ok := c["video_url"].(string); ok && strings.TrimSpace(u) != "" {
			m["video_url"] = strings.TrimSpace(u)
		}
	}
	if m["seconds"] == nil {
		if d := m["duration"]; d != nil {
			m["seconds"] = formatSeconds(d)
		}
	}
	out, err := json.Marshal(m)
	if err != nil {
		return upstreamBody
	}
	return out
}

func parseSecondsValue(v any) any {
	switch t := v.(type) {
	case string:
		if n, err := strconv.Atoi(strings.TrimSpace(t)); err == nil {
			return n
		}
		return t
	case float64:
		return int(t)
	default:
		return v
	}
}

func formatSeconds(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case float64:
		return strconv.Itoa(int(t))
	case int:
		return strconv.Itoa(t)
	default:
		return "4"
	}
}

func mapVolcStatus(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	switch s {
	case "succeeded", "success", "done":
		return "completed"
	case "running", "processing", "in_progress", "pending":
		if s == "pending" {
			return "queued"
		}
		return "in_progress"
	case "failed", "error":
		return "failed"
	default:
		return s
	}
}

func stringField(m map[string]any, keys ...string) (string, bool) {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
				return s, true
			}
		}
	}
	return "", false
}

// SizeToVolcResolutionRatio 解析 OpenAI size 如 1280x720。
func SizeToVolcResolutionRatio(size string) (resolution, ratio string) {
	size = strings.TrimSpace(strings.ToLower(size))
	parts := strings.Split(size, "x")
	if len(parts) != 2 {
		return "", ""
	}
	w, e1 := strconv.Atoi(strings.TrimSpace(parts[0]))
	h, e2 := strconv.Atoi(strings.TrimSpace(parts[1]))
	if e1 != nil || e2 != nil || w <= 0 || h <= 0 {
		return "", ""
	}
	switch {
	case h >= 1080:
		resolution = "1080p"
	case h >= 720:
		resolution = "720p"
	default:
		resolution = "480p"
	}
	g := gcd(w, h)
	ratio = strconv.Itoa(w/g) + ":" + strconv.Itoa(h/g)
	return resolution, ratio
}

func gcd(a, b int) int {
	for b != 0 {
		a, b = b, a%b
	}
	if a < 1 {
		return 1
	}
	return a
}
