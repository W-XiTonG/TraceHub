package pricingsync

import "encoding/json"

const (
	VersionTraceHubPricingV1 = "tracehub_pricing_v1"
	DirectionPull              = "pull"
	DirectionPush              = "push"
	DirectionBoth              = "both"

	ScopeGlobal  = "global"
	ScopeChannel = "channel"
	ScopeGroup   = "group"

	ChangeAdded   = "added"
	ChangeUpdated = "updated"
	ChangeSkipped = "skipped"
	ChangeRemoved = "removed"
)

// Payload tracehub_pricing_v1 同步载荷。
type Payload struct {
	Version     string              `json:"version"`
	GeneratedAt string              `json:"generated_at,omitempty"`
	Global      []GlobalItem        `json:"global,omitempty"`
	Channels    []ChannelBlock      `json:"channels,omitempty"`
	Groups      []GroupBlock        `json:"groups,omitempty"`
}

type GlobalItem struct {
	ModelName       string   `json:"model_name"`
	Enabled         bool     `json:"enabled"`
	QuotaInPer1k    *int64   `json:"quota_in_per_1k"`
	QuotaOutPer1k   *int64   `json:"quota_out_per_1k"`
	InputRate       float64  `json:"input_rate"`
	OutputRate      float64  `json:"output_rate"`
	MarkupPercent   *float64 `json:"markup_percent"`
	BillingMode     string   `json:"billing_mode,omitempty"`
	QuotaPerRequest *int64   `json:"quota_per_request"`
}

type ChannelBlock struct {
	Ref       string         `json:"ref,omitempty"`
	ChannelID int            `json:"channel_id,omitempty"`
	Items     []ChannelItem  `json:"items"`
}

type ChannelItem struct {
	GlobalItem
}

type GroupBlock struct {
	Ref     string      `json:"ref,omitempty"`
	GroupID int         `json:"group_id,omitempty"`
	Items   []GroupItem `json:"items"`
}

type GroupItem struct {
	ModelName  string  `json:"model_name"`
	InputRate  float64 `json:"input_rate"`
	OutputRate float64 `json:"output_rate"`
	Enabled    bool    `json:"enabled"`
	Billable   bool    `json:"billable"`
}

// DiffRow 预览/差异行。
type DiffRow struct {
	Layer      string `json:"layer"`
	ModelName  string `json:"model_name"`
	ChannelID  int    `json:"channel_id,omitempty"`
	GroupID    int    `json:"group_id,omitempty"`
	Change     string `json:"change"`
	Selected   bool   `json:"selected"`
	Detail     string `json:"detail,omitempty"`
}

// PreviewResult 预览结果。
type PreviewResult struct {
	PreviewID  string    `json:"preview_id"`
	ExpiresAt  string    `json:"expires_at"`
	Direction  string    `json:"direction"`
	Added      int       `json:"added"`
	Updated    int       `json:"updated"`
	Skipped    int       `json:"skipped"`
	Removed    int       `json:"removed"`
	Warnings   []string  `json:"warnings"`
	Rows       []DiffRow `json:"rows"`
	SampleDiff []DiffRow `json:"sample_diff"`
}

// SyncResult 执行结果。
type SyncResult struct {
	Added    int      `json:"added"`
	Updated  int      `json:"updated"`
	Skipped  int      `json:"skipped"`
	Removed  int      `json:"removed"`
	Warnings []string `json:"warnings"`
	Message  string   `json:"message"`
}

// SelectedKey 唯一标识一条同步记录。
type SelectedKey struct {
	Layer     string
	ModelName string
	ChannelID int
	GroupID   int
}

func (k SelectedKey) String() string {
	switch k.Layer {
	case ScopeChannel:
		return ScopeChannel + ":" + itoa(k.ChannelID) + ":" + k.ModelName
	case ScopeGroup:
		return ScopeGroup + ":" + itoa(k.GroupID) + ":" + k.ModelName
	default:
		return ScopeGlobal + ":" + k.ModelName
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

// ParsePayloadJSON 解析 tracehub_pricing_v1 JSON 载荷。
func ParsePayloadJSON(body []byte) (*Payload, error) {
	var p Payload
	if err := json.Unmarshal(body, &p); err != nil {
		return nil, err
	}
	if p.Version != VersionTraceHubPricingV1 {
		return nil, ErrVersionMismatch
	}
	return &p, nil
}
