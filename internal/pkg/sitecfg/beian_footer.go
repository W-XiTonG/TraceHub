package sitecfg

import (
	"context"
	"strings"
	"unicode"
	"unicode/utf8"

	"tracehub/internal/ent"
	"tracehub/internal/ent/systemconfig"
	"tracehub/internal/pkg/config"
)

const (
	// SystemConfigKeyBeianFooterText 全站页脚备案等多行文案（\n 分行）。
	SystemConfigKeyBeianFooterText = "site.beian_footer_text"
	beianFooterMaxRunes = 500
)

// EffectiveBeianFooterText 对外展示用的原始多行文本：库中非空则优先，否则 YAML。
func EffectiveBeianFooterText(ctx context.Context, client *ent.Client, cfg *config.Config) string {
	fallback := ""
	if cfg != nil {
		fallback = strings.TrimSpace(cfg.Site.BeianFooterText)
	}
	v, ok, err := stringFromConfigRow(ctx, client, SystemConfigKeyBeianFooterText)
	if err != nil || !ok {
		return fallback
	}
	s := strings.TrimSpace(v)
	if s == "" {
		return fallback
	}
	return v
}

// EffectiveBeianFooterLines 解析后的非空行；无内容时返回 nil 切片（长度为 0）。
func EffectiveBeianFooterLines(ctx context.Context, client *ent.Client, cfg *config.Config) []string {
	return ParseBeianFooterLines(EffectiveBeianFooterText(ctx, client, cfg))
}

// ParseBeianFooterLines 将多行文本拆为展示行（trim、丢弃空行）。
func ParseBeianFooterLines(raw string) []string {
	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	raw = strings.ReplaceAll(raw, "\r", "\n")
	lines := strings.Split(raw, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		s := strings.TrimSpace(line)
		if s == "" {
			continue
		}
		out = append(out, s)
	}
	return out
}

// BeianFooterHasVisibleContent 是否应在页脚展示备案区块。
func BeianFooterHasVisibleContent(lines []string) bool {
	return len(lines) > 0
}

// BeianFooterTextDBMeta 是否已在库中写入该键，以及原始值（供管理端展示）。
func BeianFooterTextDBMeta(ctx context.Context, client *ent.Client) (hasRow bool, raw string) {
	row, err := client.SystemConfig.Query().
		Where(systemconfig.ConfigKeyEQ(SystemConfigKeyBeianFooterText)).
		Only(ctx)
	if err != nil {
		return false, ""
	}
	return true, row.ConfigValue
}

// ValidateBeianFooterText 管理端写入前校验；允许空串表示清空。
func ValidateBeianFooterText(s string) error {
	if utf8.RuneCountInString(s) > beianFooterMaxRunes {
		return errBeianFooterTooLong
	}
	for _, r := range s {
		if r == '\n' || r == '\t' {
			continue
		}
		if unicode.IsControl(r) {
			return errBeianFooterInvalid
		}
	}
	return nil
}
