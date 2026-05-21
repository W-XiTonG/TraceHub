package klingvideo

import (
	"strings"
)

// 官方文档业务模块前缀（相对 v1/ 之后的路径，如 videos/text2video）。
var defaultAllowPrefixes = []string{
	"videos/",
	"images/",
	"kolors/",
	"audio/",
	"digital-human/",
	"digital_human/",
	"general/",
	"account/",
}

// billableCreatePrefixes POST 创建类任务预扣额度。
var billableCreatePrefixes = []string{
	"videos/",
	"images/",
	"kolors/",
	"audio/",
	"digital-human/",
	"digital_human/",
}

// AllowlistConfig 站点扩展白名单/拒绝列表。
type AllowlistConfig struct {
	ExtraAllowPrefixes []string
	DenyPrefixes       []string
}

func normalizeRest(rest string) string {
	rest = strings.Trim(strings.TrimSpace(rest), "/")
	if strings.Contains(rest, "..") {
		return ""
	}
	return rest
}

func pathMatchesAny(rest string, prefixes []string) bool {
	if rest == "" {
		return false
	}
	for _, p := range prefixes {
		p = strings.Trim(strings.TrimSpace(p), "/")
		if p == "" {
			continue
		}
		if rest == p || strings.HasPrefix(rest, p+"/") {
			return true
		}
	}
	return false
}

func allAllowPrefixes(cfg AllowlistConfig) []string {
	out := append([]string(nil), defaultAllowPrefixes...)
	for _, p := range cfg.ExtraAllowPrefixes {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, strings.Trim(p, "/"))
		}
	}
	return out
}

// PassthroughAllowed 是否允许透传该相对路径（klingPath，不含 /v1/kling 前缀）。
func PassthroughAllowed(rest string, cfg AllowlistConfig) bool {
	rest = normalizeRest(rest)
	if rest == "" {
		return false
	}
	for _, d := range cfg.DenyPrefixes {
		if pathMatchesAny(rest, []string{strings.Trim(strings.TrimSpace(d), "/")}) {
			return false
		}
	}
	return pathMatchesAny(rest, allAllowPrefixes(cfg))
}

// IsBillableCreate POST 且为创建类任务路径。
func IsBillableCreate(method, rest string) bool {
	if strings.ToUpper(strings.TrimSpace(method)) != "POST" {
		return false
	}
	rest = normalizeRest(rest)
	return pathMatchesAny(rest, billableCreatePrefixes)
}

// IsTaskQuery GET 单任务查询（用于响应规范化与结算）。
func IsTaskQuery(method, rest string) bool {
	if strings.ToUpper(strings.TrimSpace(method)) != "GET" {
		return false
	}
	rest = normalizeRest(rest)
	if rest == "" {
		return false
	}
	// 列表接口末尾无 task id；单任务为 module/action/{id}
	parts := strings.Split(rest, "/")
	if len(parts) < 3 {
		return false
	}
	last := parts[len(parts)-1]
	if last == "" || last == "list" {
		return false
	}
	// account、general 查询类
	if strings.HasPrefix(rest, "account/") {
		return false
	}
	return pathMatchesAny(rest, []string{
		"videos/", "images/", "kolors/", "audio/", "digital-human/", "digital_human/", "general/",
	})
}

// UpstreamRelativePath 拼上游路径 v1/{rest}。
func UpstreamRelativePath(rest string) string {
	rest = normalizeRest(rest)
	if rest == "" {
		return "v1"
	}
	return "v1/" + rest
}

// CreateKindFromRest 从创建路径推断任务查询类型（存入 Redis meta）。
func CreateKindFromRest(rest string) CreateKind {
	rest = normalizeRest(rest)
	switch {
	case strings.HasPrefix(rest, "videos/image2video"):
		return CreateImage2Video
	case strings.HasPrefix(rest, "videos/text2video"):
		return CreateText2Video
	case strings.HasPrefix(rest, "videos/extend"):
		return CreateKind("extend")
	case strings.HasPrefix(rest, "videos/lip-sync"), strings.HasPrefix(rest, "videos/lip_sync"):
		return CreateKind("lip_sync")
	case strings.HasPrefix(rest, "videos/effects"):
		return CreateKind("effects")
	case strings.HasPrefix(rest, "videos/"):
		return CreateKind(strings.TrimPrefix(rest, "videos/"))
	default:
		return CreateKind(rest)
	}
}
