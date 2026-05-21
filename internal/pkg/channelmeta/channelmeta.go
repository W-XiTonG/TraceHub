// Package channelmeta 集中维护管理端渠道类型元数据（默认 URL、能力、预设模型等）。
// 网关转发：volcengine 对话等走 UsesOpenAICompatRelay，视频仍走既有 volcvideo 分支。
package channelmeta

import (
	"strings"

	"tracehub/internal/service/billing"
)

// VideoProtocol 描述视频上游协议形态（供管理端展示与未来扩展）。
const (
	VideoProtocolOpenAI    = "openai_videos"
	VideoProtocolVolcArk   = "volc_ark_tasks"
	VideoProtocolKlingNative = "kling_native"
)

// Meta 单种 channel_type 的运营与默认配置元数据。
type Meta struct {
	ChannelType    string   `json:"channel_type"`
	DisplayName    string   `json:"display_name"`
	DefaultBaseURL string   `json:"default_base_url"`
	Capabilities   []string `json:"capabilities"`
	VideoProtocol  string   `json:"video_protocol,omitempty"`
	PresetModels   []string `json:"preset_models"`
	AdminHints     []string `json:"admin_hints"`
}

var klingPresetModels = []string{
	"kling-v3-omni",
	"kling-v3",
	"kling-v2.6-pro",
	"kling-v2.6-std",
	"kling-v2.5-turbo",
	"kling-video-o1",
}

var volcenginePresetModels = []string{
	"doubao-seedance-2-0-260128",
	"doubao-seedance-2-0-fast-260128",
	"doubao-seedance-1-0-pro-250528",
	"doubao-seedance-1-0-pro-fast-251015",
	"doubao-seedance-1-0-lite-250528",
	"doubao-seedance-1-0-lite-i2v-250428",
	"doubao-seedance-1-5-pro-251215",
	"seedance-1-0-pro",
}

var registry = []Meta{
	{
		ChannelType:    "openai",
		DisplayName:    "OpenAI",
		DefaultBaseURL: "https://api.openai.com/v1",
		Capabilities:   []string{"chat", "embeddings", "images", "audio", "video"},
		VideoProtocol:  VideoProtocolOpenAI,
		AdminHints: []string{
			"支持对话、Embeddings、图像、音频、视频（OpenAI 兼容 /v1）。",
			"api_base_url 留空时使用官方默认地址；可填代理或兼容网关。",
		},
	},
	{
		ChannelType:    "deepseek",
		DisplayName:    "DeepSeek",
		DefaultBaseURL: "https://api.deepseek.com/v1",
		Capabilities:   []string{"chat", "embeddings"},
		VideoProtocol:  VideoProtocolOpenAI,
		AdminHints: []string{
			"OpenAI 兼容对话与 Embeddings；api_base_url 留空使用 DeepSeek 官方地址。",
		},
	},
	{
		ChannelType:    "claude",
		DisplayName:    "Claude (Anthropic)",
		DefaultBaseURL: "https://api.anthropic.com/v1",
		Capabilities:   []string{"chat"},
		AdminHints: []string{
			"对话走 Anthropic Messages API；请勿在本渠道挂仅视频模型。",
		},
	},
	{
		ChannelType:    "gemini",
		DisplayName:    "Gemini",
		DefaultBaseURL: "https://generativelanguage.googleapis.com",
		Capabilities:   []string{"chat"},
		AdminHints: []string{
			"对话走 Gemini generateContent；api_key 作为 query 参数传递。",
		},
	},
	{
		ChannelType:    "volcengine",
		DisplayName:    "火山方舟",
		DefaultBaseURL: "https://ark.cn-beijing.volces.com/api/v3",
		Capabilities:   []string{"chat", "embeddings", "images", "audio", "video"},
		VideoProtocol:  VideoProtocolVolcArk,
		PresetModels:   append([]string(nil), volcenginePresetModels...),
		AdminHints: []string{
			"对话、Embeddings、图像、音频等走方舟 OpenAI 兼容 /api/v3/*（与 openai 渠道相同 relay）。",
			"视频/3D（/v1/videos）仍经网关转换为方舟 contents/generations/tasks，非原生 /videos 透传。",
			"Realtime、Responses 等依赖方舟是否开通；未开通时由上游返回 4xx。",
			"图生视频建议 JSON + 公网 image_url；multipart input_reference 仅部分 3D 模型支持。",
			"模型 ID 请使用方舟 Model ID，必要时配置 model_alias → ep- 接入点。",
		},
	},
	{
		ChannelType:    "kling",
		DisplayName:    "可灵 Kling",
		DefaultBaseURL: "https://api-beijing.klingai.com",
		Capabilities:   []string{"video"},
		VideoProtocol:  VideoProtocolKlingNative,
		PresetModels:   append([]string(nil), klingPresetModels...),
		AdminHints: []string{
			"仅视频能力；网关将 OpenAI /v1/videos 转为可灵 text2video / image2video。",
			"api_key 填写 AccessKey:SecretKey（英文冒号分隔），网关自动签发 JWT。",
			"api_base_url 留空使用官方北京域名；模型 ID 写入 models，可用 model_alias 映射上游 model_name。",
			"图生视频请提供 image_url 或 multipart input_reference；复杂能力（特效/Omni/对口型等）首期不转换。",
		},
	},
}

var byType map[string]Meta

func init() {
	byType = make(map[string]Meta, len(registry))
	for _, m := range registry {
		byType[strings.ToLower(strings.TrimSpace(m.ChannelType))] = m
	}
}

// All 返回已注册渠道类型元数据（顺序稳定）。
func All() []Meta {
	out := make([]Meta, len(registry))
	for i, m := range registry {
		out[i] = m
		out[i].PresetModels = append([]string(nil), m.PresetModels...)
		out[i].Capabilities = append([]string(nil), m.Capabilities...)
		out[i].AdminHints = append([]string(nil), m.AdminHints...)
	}
	return out
}

// Lookup 按 channel_type 查找元数据；未知类型返回零值 Meta 与 false。
func Lookup(channelType string) (Meta, bool) {
	ct := strings.ToLower(strings.TrimSpace(channelType))
	if ct == "" {
		return Meta{}, false
	}
	m, ok := byType[ct]
	if !ok {
		return Meta{}, false
	}
	m.PresetModels = append([]string(nil), m.PresetModels...)
	m.Capabilities = append([]string(nil), m.Capabilities...)
	m.AdminHints = append([]string(nil), m.AdminHints...)
	return m, true
}

// DefaultBaseURL 返回渠道类型默认 API 根地址（与 router.DefaultBaseURL 一致）。
func DefaultBaseURL(channelType string) string {
	if m, ok := Lookup(channelType); ok {
		return m.DefaultBaseURL
	}
	return ""
}

// ModelMatchesChannelType 判断内置模型 ID 是否大致属于某渠道类型（推荐列表过滤）。
func ModelMatchesChannelType(modelID, channelType string) bool {
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
	case "volcengine":
		return strings.HasPrefix(m, "doubao-") || strings.HasPrefix(m, "skylark") ||
			strings.Contains(m, "seedance") || strings.Contains(m, "doubao-seedance") ||
			strings.HasPrefix(m, "seedance")
	case "kling":
		return strings.HasPrefix(m, "kling-") || strings.HasPrefix(m, "kling/")
	default:
		return true
	}
}

// BuiltInPresetModels 返回某渠道类型的内置推荐模型 ID（不含目录）。
func BuiltInPresetModels(channelType string) []string {
	if m, ok := Lookup(channelType); ok && len(m.PresetModels) > 0 {
		return append([]string(nil), m.PresetModels...)
	}
	var out []string
	for _, id := range billing.DefaultModelIDs() {
		if ModelMatchesChannelType(id, channelType) {
			out = append(out, id)
		}
	}
	return out
}

// IsVolcengineChannelType 是否为火山方舟渠道类型（供探测等管理端逻辑使用）。
func IsVolcengineChannelType(channelType string) bool {
	return strings.EqualFold(strings.TrimSpace(channelType), "volcengine")
}

// IsKlingChannelType 是否为可灵渠道类型。
func IsKlingChannelType(channelType string) bool {
	return strings.EqualFold(strings.TrimSpace(channelType), "kling")
}
