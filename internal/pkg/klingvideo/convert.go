package klingvideo

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
)

// PrepareKlingVideoCreateBody 将 OpenAI Video 创建请求转为可灵 text2video / image2video 请求体与路径。
func PrepareKlingVideoCreateBody(openAIBody []byte) (kind CreateKind, body []byte, err error) {
	var m map[string]any
	if err := json.Unmarshal(openAIBody, &m); err != nil {
		return "", nil, err
	}
	model := stringField(m, "model")
	if model == "" {
		return "", nil, errors.New("missing model")
	}
	prompt := stringField(m, "prompt")
	img := extractImageFromOpenAI(m)
	if img == "" && prompt == "" {
		return "", nil, errors.New("missing prompt or image")
	}

	out := map[string]any{
		"model_name": model,
	}
	if prompt != "" {
		out["prompt"] = prompt
	}
	if d := parseDuration(m); d > 0 {
		out["duration"] = d
	}
	if ar := sizeToAspectRatio(stringField(m, "size")); ar != "" {
		out["aspect_ratio"] = ar
	}
	if mode := inferMode(m); mode != "" {
		out["mode"] = mode
	}
	if np := stringField(m, "negative_prompt"); np != "" {
		out["negative_prompt"] = np
	}

	if img != "" {
		out["image"] = img
		kind = CreateImage2Video
	} else {
		kind = CreateText2Video
	}

	body, err = json.Marshal(out)
	return kind, body, err
}

func stringField(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return strings.TrimSpace(v)
	}
	return ""
}

func parseDuration(m map[string]any) int {
	for _, k := range []string{"seconds", "duration"} {
		if v, ok := m[k]; ok {
			switch t := v.(type) {
			case string:
				if n, err := strconv.Atoi(strings.TrimSpace(t)); err == nil {
					return clampDuration(n)
				}
			case float64:
				return clampDuration(int(t))
			case int:
				return clampDuration(t)
			}
		}
	}
	return 5
}

func clampDuration(n int) int {
	if n <= 5 {
		return 5
	}
	if n >= 10 {
		return 10
	}
	return n
}

func sizeToAspectRatio(size string) string {
	size = strings.ToLower(strings.TrimSpace(size))
	switch {
	case strings.Contains(size, "9:16"), strings.Contains(size, "720x1280"), strings.Contains(size, "1080x1920"):
		return "9:16"
	case strings.Contains(size, "1:1"), strings.Contains(size, "1024x1024"):
		return "1:1"
	case strings.Contains(size, "16:9"), strings.Contains(size, "1280x720"), strings.Contains(size, "1920x1080"):
		return "16:9"
	default:
		return "16:9"
	}
}

func inferMode(m map[string]any) string {
	if mode := strings.ToLower(stringField(m, "mode")); mode != "" {
		switch mode {
		case "professional", "pro", "hd", "high":
			return "pro"
		case "standard", "std", "fast":
			return "std"
		default:
			return mode
		}
	}
	size := strings.ToLower(stringField(m, "size"))
	if strings.Contains(size, "1080") || strings.Contains(size, "1792") || strings.Contains(size, "pro") {
		return "pro"
	}
	return "std"
}

func extractImageFromOpenAI(m map[string]any) string {
	for _, k := range []string{"image_url", "image", "image_data"} {
		if s, ok := m[k].(string); ok && strings.TrimSpace(s) != "" {
			return strings.TrimSpace(s)
		}
	}
	if u, ok := m["image_url"].(map[string]any); ok {
		if s, ok := u["url"].(string); ok && strings.TrimSpace(s) != "" {
			return strings.TrimSpace(s)
		}
	}
	if raw, ok := m["content"].([]any); ok {
		for _, item := range raw {
			part, ok := item.(map[string]any)
			if !ok {
				continue
			}
			typ := strings.ToLower(stringField(part, "type"))
			if typ != "image_url" && typ != "image" {
				continue
			}
			if iu, ok := part["image_url"].(map[string]any); ok {
				if s, ok := iu["url"].(string); ok && strings.TrimSpace(s) != "" {
					return strings.TrimSpace(s)
				}
			}
			if s, ok := part["url"].(string); ok && strings.TrimSpace(s) != "" {
				return strings.TrimSpace(s)
			}
		}
	}
	return ""
}

// ImageDataFromBytes 将图片字节编码为 data URL（multipart 图生视频）。
func ImageDataFromBytes(mime string, raw []byte) string {
	mime = strings.TrimSpace(mime)
	if mime == "" {
		mime = "image/png"
	}
	return "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(raw)
}
