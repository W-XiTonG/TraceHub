package pricingsync

import (
	"strings"

	"tracehub/internal/ent"
)

// FilterOpts 同步过滤选项。
type FilterOpts struct {
	Scopes         []string
	ChannelFilter  []int
	GroupFilter    []int
	IncludeMode    string
	Allowlist      []string
	Denylist       []string
	Patterns       []string
	SelectedModels map[string]struct{}
	SelectedKeys   map[string]struct{}
}

func filterOptsFromTarget(t *ent.ModelPricingSyncTarget, selectedModels, selectedKeys []string) FilterOpts {
	opts := FilterOpts{
		Scopes:        append([]string(nil), t.Scopes...),
		ChannelFilter: append([]int(nil), t.ChannelFilter...),
		GroupFilter:   append([]int(nil), t.GroupFilter...),
		IncludeMode:   string(t.ModelIncludeMode),
		Allowlist:     append([]string(nil), t.ModelAllowlist...),
		Denylist:      append([]string(nil), t.ModelDenylist...),
		Patterns:      append([]string(nil), t.ModelIncludePatterns...),
	}
	if len(selectedModels) > 0 {
		opts.SelectedModels = make(map[string]struct{}, len(selectedModels))
		for _, m := range selectedModels {
			m = strings.TrimSpace(m)
			if m != "" {
				opts.SelectedModels[m] = struct{}{}
			}
		}
	}
	if len(selectedKeys) > 0 {
		opts.SelectedKeys = make(map[string]struct{}, len(selectedKeys))
		for _, k := range selectedKeys {
			k = strings.TrimSpace(k)
			if k != "" {
				opts.SelectedKeys[k] = struct{}{}
			}
		}
	}
	return opts
}

func scopeEnabled(scopes []string, scope string) bool {
	if len(scopes) == 0 {
		return true
	}
	for _, s := range scopes {
		if s == scope {
			return true
		}
	}
	return false
}

func idInFilter(id int, filter []int) bool {
	if len(filter) == 0 {
		return true
	}
	for _, f := range filter {
		if f == id {
			return true
		}
	}
	return false
}

func modelAllowed(name string, opts FilterOpts) bool {
	name = strings.TrimSpace(name)
	if name == "" {
		return false
	}
	mode := opts.IncludeMode
	if mode == "" {
		mode = "all"
	}
	switch mode {
	case "allowlist":
		if len(opts.Allowlist) == 0 && len(opts.Patterns) == 0 {
			return false
		}
		for _, a := range opts.Allowlist {
			if a == name {
				return true
			}
		}
		for _, p := range opts.Patterns {
			if simpleGlob(p, name) {
				return true
			}
		}
		return false
	case "denylist":
		for _, d := range opts.Denylist {
			if d == name {
				return false
			}
		}
		for _, p := range opts.Patterns {
			if simpleGlob(p, name) {
				return false
			}
		}
		return true
	default:
		for _, d := range opts.Denylist {
			if d == name {
				return false
			}
		}
		return true
	}
}

func keySelected(key SelectedKey, opts FilterOpts) bool {
	if len(opts.SelectedKeys) > 0 {
		_, ok := opts.SelectedKeys[key.String()]
		return ok
	}
	if len(opts.SelectedModels) > 0 {
		_, ok := opts.SelectedModels[key.ModelName]
		return ok
	}
	return true
}

func rowPasses(key SelectedKey, opts FilterOpts) bool {
	if !scopeEnabled(opts.Scopes, key.Layer) {
		return false
	}
	if key.Layer == ScopeChannel && !idInFilter(key.ChannelID, opts.ChannelFilter) {
		return false
	}
	if key.Layer == ScopeGroup && !idInFilter(key.GroupID, opts.GroupFilter) {
		return false
	}
	if !modelAllowed(key.ModelName, opts) {
		return false
	}
	return keySelected(key, opts)
}

// simpleGlob 仅支持 * 后缀/前缀通配。
func simpleGlob(pattern, name string) bool {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return false
	}
	if strings.HasPrefix(pattern, "*") && strings.HasSuffix(pattern, "*") {
		mid := strings.Trim(pattern, "*")
		return mid != "" && strings.Contains(name, mid)
	}
	if strings.HasPrefix(pattern, "*") {
		suffix := strings.TrimPrefix(pattern, "*")
		return suffix != "" && strings.HasSuffix(name, suffix)
	}
	if strings.HasSuffix(pattern, "*") {
		prefix := strings.TrimSuffix(pattern, "*")
		return prefix != "" && strings.HasPrefix(name, prefix)
	}
	return pattern == name
}
