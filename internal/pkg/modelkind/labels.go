package modelkind

import (
	"fmt"
	"strings"
	"unicode"
)

const (
	maxAdminLabelCount  = 16
	maxAdminLabelRunes  = 48
)

// NormalizeAdminLabels trims, dedupes admin-provided tags; max 16 tags, each max 48 runes.
// Allowed: Unicode letters and numbers plus ._:- ; ASCII tags are stored lowercased.
func NormalizeAdminLabels(in []string) ([]string, error) {
	if len(in) == 0 {
		return nil, nil
	}
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, raw := range in {
		s := strings.TrimSpace(raw)
		if s == "" {
			continue
		}
		runes := []rune(s)
		if len(runes) > maxAdminLabelRunes {
			return nil, fmt.Errorf("标签过长（最多 %d 字符）: %q", maxAdminLabelRunes, truncateRunes(runes, 20))
		}
		for _, r := range runes {
			if r == '_' || r == '-' || r == ':' || r == '.' {
				continue
			}
			if unicode.IsLetter(r) || unicode.IsNumber(r) {
				continue
			}
			return nil, fmt.Errorf("标签含非法字符: %q", s)
		}
		key := strings.TrimSpace(s)
		if isAllASCII(key) {
			key = strings.ToLower(key)
		}
		if _, ok := seen[key]; ok {
			continue
		}
		if len(out) >= maxAdminLabelCount {
			return nil, fmt.Errorf("最多 %d 个标签", maxAdminLabelCount)
		}
		seen[key] = struct{}{}
		out = append(out, key)
	}
	if len(out) == 0 {
		return nil, nil
	}
	return out, nil
}

func isAllASCII(s string) bool {
	for _, r := range s {
		if r > 127 {
			return false
		}
	}
	return true
}

func truncateRunes(r []rune, n int) string {
	if len(r) <= n {
		return string(r)
	}
	return string(r[:n]) + "…"
}

// LabelsForModel returns stable lowercase label keys for UI (e.g. chat, embedding).
// adminLabels: 后台配置，优先展示，与 metadata / 启发式合并去重。
// catalogMeta 可含 kind、labels（历史/上游同步写入 metadata）。
func LabelsForModel(modelID string, catalogMeta map[string]any, adminLabels []string) []string {
	id := strings.ToLower(strings.TrimSpace(modelID))
	admin := dedupeKeepOrder(adminLabels)
	meta := metaLabels(catalogMeta)
	infer := inferFromID(id)
	videoModel := false
	for _, x := range infer {
		if x == "video" {
			videoModel = true
			break
		}
	}

	out := make([]string, 0, len(admin)+len(meta)+len(infer))
	seen := make(map[string]struct{}, len(admin)+len(meta)+len(infer))

	add := func(s string) {
		s = strings.TrimSpace(strings.ToLower(s))
		if s == "" {
			return
		}
		// 火山方舟等同步目录常标 kind:chat，但 Seedance/Sora 等实为视频；展示以 video 为准。
		if videoModel && s == "chat" {
			return
		}
		if _, ok := seen[s]; ok {
			return
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}

	hasExplicit := len(admin) > 0 || len(meta) > 0
	for _, m := range admin {
		add(m)
	}
	for _, m := range meta {
		add(m)
	}
	for _, x := range infer {
		if hasExplicit && x == "chat" {
			continue
		}
		add(x)
	}
	if len(out) == 0 {
		if videoModel {
			add("video")
		} else {
			add("chat")
		}
	}
	if videoModel {
		out = prioritizeLabel(out, "video")
	}
	return out
}

func prioritizeLabel(labels []string, primary string) []string {
	primary = strings.TrimSpace(strings.ToLower(primary))
	if primary == "" || len(labels) <= 1 {
		return labels
	}
	idx := -1
	for i, s := range labels {
		if s == primary {
			idx = i
			break
		}
	}
	if idx <= 0 {
		return labels
	}
	out := make([]string, 0, len(labels))
	out = append(out, labels[idx])
	out = append(out, labels[:idx]...)
	out = append(out, labels[idx+1:]...)
	return out
}

func metaLabels(catalogMeta map[string]any) []string {
	if catalogMeta == nil {
		return nil
	}
	var out []string
	if k, ok := catalogMeta["kind"].(string); ok {
		if s := strings.TrimSpace(strings.ToLower(k)); s != "" {
			out = append(out, s)
		}
	}
	raw, ok := catalogMeta["labels"].([]any)
	if ok {
		for _, v := range raw {
			s, ok := v.(string)
			if !ok {
				continue
			}
			if t := strings.TrimSpace(strings.ToLower(s)); t != "" {
				out = append(out, t)
			}
		}
	}
	return dedupeKeepOrder(out)
}

// inferFromID 按优先级匹配启发式标签；无法覆盖所有厂商命名，剩余依赖后台 admin_labels。
// 已知局限：纯对话模型名若含 image/video/embed 等子串可能误判；多模态「对话+图」多数仍归为 chat。
func inferFromID(id string) []string {
	if id == "" {
		return []string{"chat"}
	}
	if strings.Contains(id, "moderation") {
		return []string{"moderation"}
	}
	if strings.Contains(id, "rerank") || strings.Contains(id, "reranker") {
		return []string{"rerank"}
	}
	if strings.Contains(id, "text-embedding") || strings.Contains(id, "embedding-") ||
		strings.Contains(id, "-embedding") || strings.HasPrefix(id, "embed-") ||
		(strings.HasPrefix(id, "bge-") && !strings.Contains(id, "rerank")) ||
		strings.HasPrefix(id, "e5-") ||
		strings.HasPrefix(id, "voyage-") ||
		strings.Contains(id, "nomic-embed") || strings.Contains(id, "jina-embed") ||
		strings.Contains(id, "cohere.embed") || strings.Contains(id, "cohere-embed") ||
		strings.Contains(id, "arctic-embed") {
		return []string{"embedding"}
	}
	// 视频生成（需在 image 的泛化 -image 规则之前，避免 xxx-video-image 等被标成 image）
	if strings.Contains(id, "text-to-video") || strings.Contains(id, "image-to-video") ||
		strings.Contains(id, "video-gen") || strings.Contains(id, "t2v") ||
		strings.Contains(id, "i2v") || strings.Contains(id, "-video") ||
		strings.Contains(id, "/video") || strings.Contains(id, "_video") ||
		strings.Contains(id, "seedance") || strings.Contains(id, "doubao-seed") ||
		strings.Contains(id, "seed3d") ||
		strings.Contains(id, "sora") || strings.Contains(id, "kling") ||
		strings.Contains(id, "hunyuan-video") || strings.Contains(id, "hunyuan_video") ||
		strings.Contains(id, "veo-") || strings.Contains(id, "/veo") || strings.HasPrefix(id, "veo") {
		return []string{"video"}
	}
	if strings.Contains(id, "dall-e") || strings.Contains(id, "dalle") ||
		strings.Contains(id, "gpt-image") || strings.Contains(id, "midjourney") ||
		strings.Contains(id, "stable-diffusion") || strings.Contains(id, "flux-") ||
		strings.HasPrefix(id, "flux") || strings.Contains(id, "imagen") ||
		strings.Contains(id, "z-image") ||
		strings.Contains(id, "-image") || strings.Contains(id, "/image") ||
		strings.Contains(id, "_image") ||
		strings.Contains(id, "img2img") || strings.Contains(id, "text-to-image") ||
		strings.Contains(id, "image-gen") ||
		strings.Contains(id, "sdxl") || strings.Contains(id, "ideogram") ||
		strings.Contains(id, "recraft") || strings.Contains(id, "text2img") ||
		strings.Contains(id, "txt2img") {
		return []string{"image"}
	}
	if strings.HasPrefix(id, "tts-") || strings.Contains(id, "whisper") ||
		strings.Contains(id, "speech-") || strings.Contains(id, "audio-speech") ||
		strings.Contains(id, "elevenlabs") || strings.Contains(id, "eleven-") ||
		strings.Contains(id, "eleven_") || strings.Contains(id, "/eleven") || strings.Contains(id, "fish-speech") ||
		strings.Contains(id, "fish/audio") || strings.Contains(id, "cosyvoice") ||
		strings.Contains(id, "suno") || strings.Contains(id, "musicgen") {
		return []string{"audio"}
	}
	return []string{"chat"}
}

func dedupeKeepOrder(in []string) []string {
	if len(in) == 0 {
		return in
	}
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		t := strings.TrimSpace(strings.ToLower(s))
		if t == "" {
			continue
		}
		if _, ok := seen[t]; ok {
			continue
		}
		seen[t] = struct{}{}
		out = append(out, t)
	}
	return out
}
