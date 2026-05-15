package sitecfg

import (
	"context"
	"encoding/json"
	"strings"

	"tracehub/internal/ent"
	"tracehub/internal/ent/systemconfig"
	"tracehub/internal/pkg/config"
)

// SystemConfigKeyNoBillModels 全局不计费模型列表（JSON 字符串数组）；可被 system_configs 覆盖 YAML。
const SystemConfigKeyNoBillModels = "site.no_bill_models"

// EffectiveNoBillModels 返回生效的全局不计费模型名列表（去重、trim）；DB JSON 非空优先，否则 YAML。
func EffectiveNoBillModels(ctx context.Context, client *ent.Client, cfg *config.Config) []string {
	fallback := normalizeModelIDList(cfg.Site.NoBillModels)
	row, err := client.SystemConfig.Query().Where(systemconfig.ConfigKeyEQ(SystemConfigKeyNoBillModels)).Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return fallback
		}
		return fallback
	}
	parsed := parseNoBillModelsJSON(row.ConfigValue)
	if len(parsed) > 0 {
		return parsed
	}
	return fallback
}

// NoBillModelsDBMeta 管理端：是否库覆盖及原始值。
func NoBillModelsDBMeta(ctx context.Context, client *ent.Client) (hasRow bool, raw string) {
	row, err := client.SystemConfig.Query().
		Where(systemconfig.ConfigKeyEQ(SystemConfigKeyNoBillModels)).
		Only(ctx)
	if err != nil {
		return false, ""
	}
	return true, row.ConfigValue
}

// ValidateNoBillModelsJSON 管理端写入 site.no_bill_models 前校验（JSON 数组，元素为 model id）。
func ValidateNoBillModelsJSON(value string) error {
	v := strings.TrimSpace(value)
	if v == "" {
		return nil
	}
	var arr []string
	if err := json.Unmarshal([]byte(v), &arr); err != nil {
		return errNoBillModelsJSON
	}
	for _, id := range arr {
		if strings.TrimSpace(id) == "" {
			return errNoBillModelsEmptyID
		}
	}
	return nil
}

func parseNoBillModelsJSON(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var arr []string
	if err := json.Unmarshal([]byte(raw), &arr); err != nil {
		return nil
	}
	return normalizeModelIDList(arr)
}

func normalizeModelIDList(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, id := range in {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

// NoBillModelsToJSON 将模型 ID 列表序列化为管理端存储格式。
func NoBillModelsToJSON(models []string) string {
	norm := normalizeModelIDList(models)
	if len(norm) == 0 {
		return "[]"
	}
	b, _ := json.Marshal(norm)
	return string(b)
}
