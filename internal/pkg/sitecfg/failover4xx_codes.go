package sitecfg

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

const SystemConfigKeyRoutingFailover4xxCodesDefault = "gateway.routing.failover_4xx_codes_default"

// DefaultSelectiveFailover4xxCodes 选择性模式未配置 codes 时的默认参与状态码。
var DefaultSelectiveFailover4xxCodes = []int{400, 401, 403, 408}

// AllowedFailover4xxCodes 管理端可勾选的状态码（不含 429，429 始终按全局规则转移）。
var AllowedFailover4xxCodes = []int{400, 401, 403, 404, 408, 413, 422}

// ParseFailover4xxCodes 解析逗号分隔或 JSON 数组，如 "401,403" 或 "[401,403]"。
func ParseFailover4xxCodes(value string) ([]int, error) {
	s := strings.TrimSpace(value)
	if s == "" {
		return nil, nil
	}
	if strings.HasPrefix(s, "[") {
		var raw []int
		if err := json.Unmarshal([]byte(s), &raw); err != nil {
			return nil, err
		}
		return NormalizeFailover4xxCodeSlice(raw), nil
	}
	parts := strings.FieldsFunc(s, func(r rune) bool {
		return r == ',' || r == ' ' || r == ';'
	})
	var codes []int
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		n, err := strconv.Atoi(p)
		if err != nil {
			return nil, fmt.Errorf("invalid status code %q", p)
		}
		codes = append(codes, n)
	}
	return NormalizeFailover4xxCodeSlice(codes), nil
}

// FormatFailover4xxCodes 持久化为逗号分隔字符串。
func FormatFailover4xxCodes(codes []int) string {
	norm := NormalizeFailover4xxCodeSlice(codes)
	if len(norm) == 0 {
		return ""
	}
	parts := make([]string, len(norm))
	for i, c := range norm {
		parts[i] = strconv.Itoa(c)
	}
	return strings.Join(parts, ",")
}

// NormalizeFailover4xxCodeSlice 去重、排序，仅保留允许列表中的码。
func NormalizeFailover4xxCodeSlice(codes []int) []int {
	if len(codes) == 0 {
		return nil
	}
	allowed := make(map[int]struct{}, len(AllowedFailover4xxCodes))
	for _, c := range AllowedFailover4xxCodes {
		allowed[c] = struct{}{}
	}
	seen := make(map[int]struct{}, len(codes))
	var out []int
	for _, c := range codes {
		if _, ok := allowed[c]; !ok {
			continue
		}
		if _, dup := seen[c]; dup {
			continue
		}
		seen[c] = struct{}{}
		out = append(out, c)
	}
	sort.Ints(out)
	return out
}

func ValidateRoutingFailover4xxCodes(value string) error {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	_, err := ParseFailover4xxCodes(value)
	return err
}

func ValidateRoutingFailover4xxCodesForMode(mode string, codes []int) error {
	mode = strings.TrimSpace(mode)
	if mode == Failover4xxModeCustom && len(NormalizeFailover4xxCodeSlice(codes)) == 0 {
		return errRoutingFailover4xxCodesRequired
	}
	return nil
}

// Failover4xxCodeSet 将切片转为查找集。
func Failover4xxCodeSet(codes []int) map[int]struct{} {
	norm := NormalizeFailover4xxCodeSlice(codes)
	m := make(map[int]struct{}, len(norm))
	for _, c := range norm {
		m[c] = struct{}{}
	}
	return m
}

// EffectiveFailover4xxCodes 合并配置与模式默认。
func EffectiveFailover4xxCodes(mode string, configured []int) []int {
	norm := NormalizeFailover4xxCodeSlice(configured)
	if len(norm) > 0 {
		return norm
	}
	if strings.TrimSpace(mode) == Failover4xxModeSelective {
		return append([]int(nil), DefaultSelectiveFailover4xxCodes...)
	}
	return nil
}
