package availmodels

import "strings"

// ownedByRule 按顺序匹配；先匹配到的生效。
type ownedByRule struct {
	owner    string
	contains []string
	prefixes []string
}

// tierPathPrefixes 路径首段常为套餐/线路，跳过后再解析组织名。
var tierPathPrefixes = map[string]struct{}{
	"pro": {}, "free": {}, "vip": {}, "standard": {}, "plus": {}, "max": {},
}

// hfOrgSlug 将路径中的组织段映射为 owned_by（小写键）。
var hfOrgSlug = map[string]string{
	"openai":            "openai",
	"anthropic":         "anthropic",
	"google":            "google",
	"meta":              "meta",
	"meta-llama":        "meta",
	"mistralai":         "mistral",
	"mistral":           "mistral",
	"deepseek-ai":       "deepseek",
	"deepseek":          "deepseek",
	"qwen":              "alibaba",
	"alibaba":           "alibaba",
	"alibaba-cloud":     "alibaba",
	"baai":              "baai",
	"moonshotai":        "moonshot",
	"moonshot":          "moonshot",
	"01-ai":             "lingyi",
	"zhipuai":           "zhipu",
	"thudm":             "zhipu",
	"chatglm":           "zhipu",
	"tencent":           "tencent",
	"bytedance":         "bytedance",
	"volcengine":        "bytedance",
	"nvidia":            "nvidia",
	"microsoft":         "microsoft",
	"stabilityai":       "stability",
	"stability-ai":      "stability",
	"black-forest-labs": "black-forest-labs",
	"cohere":            "cohere",
	"cohereforai":       "cohere",
	"togethercomputer":  "together",
	"baidu":             "baidu",
	"minimax":           "minimax",
	"stepfun-ai":        "stepfun",
	"xai-org":           "xai",
	"voyageai":          "voyage",
	"jinaai":            "jina",
	"nomic-ai":          "nomic",
	"sentence-transformers": "huggingface",
	"intfloat": "intfloat",
}

var ownedByRules = []ownedByRule{
	{owner: "openai", prefixes: []string{
		"gpt-", "gpt/", "chatgpt", "o1-", "o1/", "o3-", "o3/", "o4-", "o4/",
		"text-embedding", "tts-", "whisper", "sora", "dall-e", "dalle", "davinci",
		"omni-moderation", "gpt-image", "gpt-4", "gpt-3",
	}, contains: []string{"openai/", "/openai", "openai-"}},
	{owner: "anthropic", prefixes: []string{"claude"}, contains: []string{"anthropic/", "/anthropic"}},
	{owner: "google", prefixes: []string{"gemini", "palm", "imagen"}, contains: []string{
		"google/", "/google", "gemini-", "veo-", "veo/",
	}},
	{owner: "deepseek", prefixes: []string{"deepseek"}, contains: []string{"deepseek-ai/", "deepseek/"}},
	{owner: "alibaba", prefixes: []string{"qwen"}, contains: []string{
		"qwen/", "qwen2", "qwen3", "tongyi", "dashscope", "alibaba/", "aliyun",
	}},
	{owner: "bytedance", prefixes: []string{"doubao", "seedance", "seed-"}, contains: []string{
		"doubao-", "seedance", "skylark", "volcengine", "bytedance/",
	}},
	{owner: "tencent", prefixes: []string{"hunyuan"}, contains: []string{"tencent/", "hunyuan"}},
	{owner: "zhipu", prefixes: []string{"glm-", "glm/", "chatglm"}, contains: []string{"zhipu", "thudm/", "chatglm"}},
	{owner: "moonshot", prefixes: []string{"moonshot", "kimi"}, contains: []string{"moonshotai/", "kimi-"}},
	{owner: "baai", prefixes: []string{"bge-", "bge/", "bge."}, contains: []string{"baai/", "/baai/", "bge-m", "bge-rerank"}},
	{owner: "meta", prefixes: []string{"llama", "meta-llama"}, contains: []string{"meta-llama/", "llama-", "/llama"}},
	{owner: "mistral", prefixes: []string{"mistral", "mixtral", "codestral", "pixtral"}, contains: []string{"mistralai/", "mistral/"}},
	{owner: "xai", prefixes: []string{"grok"}, contains: []string{"xai/", "x-ai"}},
	{owner: "baidu", prefixes: []string{"ernie"}, contains: []string{"baidu/", "ernie-"}},
	{owner: "cohere", prefixes: []string{"command", "embed-english", "embed-multilingual"}, contains: []string{"cohere", "cohere."}},
	{owner: "minimax", prefixes: []string{"abab", "minimax"}, contains: []string{"minimax/"}},
	{owner: "stepfun", prefixes: []string{"step-1", "step1", "step-2"}, contains: []string{"stepfun"}},
	{owner: "lingyi", prefixes: []string{"yi-"}, contains: []string{"01-ai/", "yi-large", "yi-medium", "yi-light"}},
	{owner: "stability", contains: []string{"stable-diffusion", "sdxl", "stabilityai"}},
	{owner: "black-forest-labs", prefixes: []string{"flux"}, contains: []string{"black-forest-labs", "flux-"}},
	{owner: "kling", prefixes: []string{"kling"}, contains: []string{"kling-", "kling/"}},
	{owner: "voyage", prefixes: []string{"voyage"}, contains: []string{"voyage-"}},
	{owner: "jina", prefixes: []string{"jina"}, contains: []string{"jina-", "jina/"}},
	{owner: "nomic", prefixes: []string{"nomic"}, contains: []string{"nomic-"}},
	{owner: "elevenlabs", contains: []string{"elevenlabs", "eleven_multilingual", "eleven_turbo"}},
	{owner: "perplexity", prefixes: []string{"pplx", "sonar"}, contains: []string{"perplexity"}},
	{owner: "amazon", prefixes: []string{"amazon.titan", "titan-embed"}, contains: []string{"amazon.titan", "titan-text", "titan-embed"}},
	{owner: "nvidia", prefixes: []string{"nvidia/"}, contains: []string{"nvidia/", "nemotron"}},
	{owner: "microsoft", prefixes: []string{"phi-"}, contains: []string{"microsoft/", "phi-"}},
	{owner: "huggingface", contains: []string{"sentence-transformers/", "hf.co/"}},
	{owner: "together", contains: []string{"togethercomputer/"}},
	{owner: "openrouter", contains: []string{"openrouter/"}},
	{owner: "siliconflow", contains: []string{"siliconflow"}},
	{owner: "groq", prefixes: []string{"groq/"}, contains: []string{"groq-"}},
	{owner: "fireworks", contains: []string{"fireworks/", "accounts/fireworks"}},
	{owner: "replicate", contains: []string{"replicate/"}},
}

func inferOwnedByFromPath(id string) string {
	if !strings.Contains(id, "/") {
		return ""
	}
	parts := strings.Split(id, "/")
	for _, raw := range parts {
		p := strings.TrimSpace(strings.ToLower(raw))
		if p == "" {
			continue
		}
		if _, skip := tierPathPrefixes[p]; skip {
			continue
		}
		if owner, ok := hfOrgSlug[p]; ok && owner != "" {
			return owner
		}
		// 未在映射表中的路径段：若像组织名（无日期后缀数字集群）可原样展示
		if looksLikeOrgSlug(p) {
			return p
		}
	}
	return ""
}

func looksLikeOrgSlug(s string) bool {
	if len(s) < 2 || len(s) > 48 {
		return false
	}
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			continue
		}
		return false
	}
	// 排除纯模型名片段
	switch s {
	case "instruct", "chat", "base", "preview", "latest":
		return false
	}
	return true
}

func matchOwnedByRule(id string, rule ownedByRule) bool {
	for _, p := range rule.prefixes {
		if strings.HasPrefix(id, p) {
			return true
		}
	}
	for _, sub := range rule.contains {
		if strings.Contains(id, sub) {
			return true
		}
	}
	return false
}

// inferOwnedBy 根据模型 ID 推断 owned_by（OpenAI 兼容语义）；目录未填 owned_by 时使用。
func inferOwnedBy(id string) string {
	id = strings.TrimSpace(id)
	if id == "" {
		return "tracehub"
	}
	lower := strings.ToLower(id)

	if owner := inferOwnedByFromPath(lower); owner != "" {
		return owner
	}

	for _, rule := range ownedByRules {
		if matchOwnedByRule(lower, rule) {
			return rule.owner
		}
	}

	// 兼容旧逻辑：子串 gpt（放规则之后，避免误伤已匹配的厂商）
	if strings.HasPrefix(lower, "gpt") || strings.Contains(lower, "gpt-") || strings.Contains(lower, "/gpt") {
		return "openai"
	}

	return "tracehub"
}
