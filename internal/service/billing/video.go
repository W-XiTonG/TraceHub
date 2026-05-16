package billing

import (
	"strconv"
	"strings"
)

// DefaultVideoQuotaPerSecond 默认每秒额度（未配置全局计价表时使用）。
var DefaultVideoQuotaPerSecond = map[string]int64{
	"sora-2":                800,
	"sora-2-pro":            1500,
	"doubao-seedance-2-0":   900,
	"seedance":              600,
	"doubao-seedance":       500,
	"kling":                 700,
	"veo":                   900,
	"hunyuan-video":         550,
}

// VideoJobParams 创建视频任务时的计费参数。
type VideoJobParams struct {
	Seconds int
	Size    string
}

// ParseVideoSeconds 解析 OpenAI seconds 字段（字符串或数字），默认 4。
func ParseVideoSeconds(raw string) int {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 4
	}
	if n, err := strconv.Atoi(raw); err == nil && n > 0 {
		return n
	}
	return 4
}

// SizeCostMultiplier 分辨率乘数。
func SizeCostMultiplier(size string) float64 {
	size = strings.ToLower(strings.TrimSpace(size))
	if strings.Contains(size, "1792") || strings.Contains(size, "1024x1792") || strings.Contains(size, "1080") {
		return 1.8
	}
	if strings.Contains(size, "1280x720") || strings.Contains(size, "720") {
		return 1.0
	}
	return 1.2
}

func longestVideoPrefixKey(model string) string {
	keys := make([]string, 0, len(DefaultVideoQuotaPerSecond))
	for k := range DefaultVideoQuotaPerSecond {
		keys = append(keys, k)
	}
	best := ""
	for _, k := range keys {
		if strings.HasPrefix(model, k) && len(k) > len(best) {
			best = k
		}
	}
	return best
}

// LookupVideoQuotaPerSecond 每秒额度。
func LookupVideoQuotaPerSecond(model string) int64 {
	if k := longestVideoPrefixKey(model); k != "" {
		return DefaultVideoQuotaPerSecond[k]
	}
	return 500
}

// VideoJobCost 估算视频任务额度（首期预扣；in1k 表示每秒额度）。
func (b ModelBillRates) VideoJobCost(model string, params VideoJobParams) int64 {
	if !b.Billable {
		return 0
	}
	sec := params.Seconds
	if sec < 1 {
		sec = 4
	}
	perSec := b.in1k
	if perSec <= 0 {
		perSec = LookupVideoQuotaPerSecond(model)
	}
	mul := SizeCostMultiplier(params.Size)
	raw := float64(perSec*int64(sec)) * mul * b.inRate
	raw *= 1 + b.markupPercent/100.0
	raw *= b.groupMul * b.userMul
	if raw < 1 {
		return 1
	}
	return int64(raw + 0.999999)
}
