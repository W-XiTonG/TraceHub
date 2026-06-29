package sitecfg

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"unicode/utf8"

	"tracehub/internal/ent"
)

// SystemConfigKeyRechargeShopsJSON 充值外部商铺链接配置（JSON）。
const SystemConfigKeyRechargeShopsJSON = "site.recharge_shops"

const (
	rechargeShopsMaxBytes      = 16 * 1024
	rechargeShopsMaxTitle      = 80
	rechargeShopsMaxHint       = 500
	rechargeShopsMaxItems      = 20
	rechargeShopsMaxLabel      = 80
	rechargeShopsMaxDescription = 200
	rechargeShopsMaxURLLen     = 512
)

// RechargeShopItem 单条外部充值商铺。
type RechargeShopItem struct {
	Label       string `json:"label"`
	URL         string `json:"url"`
	Description string `json:"description"`
	Enabled     bool   `json:"enabled"`
	Sort        int    `json:"sort"`
	ShowQR      bool   `json:"show_qr"`
	QRImageURL  string `json:"qr_image_url"`
}

// RechargeShopsData 解析并校验后的充值商铺配置（管理端完整列表）。
type RechargeShopsData struct {
	Title string             `json:"title"`
	Hint  string             `json:"hint"`
	Shops []RechargeShopItem `json:"shops"`
}

// StoredRechargeShopsJSON 数据库中的原始 JSON（trim 后空表示未配置）。
func StoredRechargeShopsJSON(ctx context.Context, client *ent.Client) string {
	v, ok, err := stringFromConfigRow(ctx, client, SystemConfigKeyRechargeShopsJSON)
	if err != nil || !ok {
		return ""
	}
	return strings.TrimSpace(v)
}

// EffectiveRechargeShopsData 对外使用的充值商铺数据；仅含已启用且合法的条目，按 sort 升序。
func EffectiveRechargeShopsData(ctx context.Context, client *ent.Client) RechargeShopsData {
	raw := StoredRechargeShopsJSON(ctx, client)
	if raw == "" {
		return RechargeShopsData{Shops: []RechargeShopItem{}}
	}
	d, err := ParseAndSanitizeRechargeShops([]byte(raw))
	if err != nil {
		return RechargeShopsData{Shops: []RechargeShopItem{}}
	}
	return VisibleRechargeShops(d)
}

// AdminRechargeShopsData 管理端读取的完整配置（含 disabled 条目）。
func AdminRechargeShopsData(ctx context.Context, client *ent.Client) RechargeShopsData {
	raw := StoredRechargeShopsJSON(ctx, client)
	if raw == "" {
		return RechargeShopsData{Shops: []RechargeShopItem{}}
	}
	d, err := ParseAndSanitizeRechargeShops([]byte(raw))
	if err != nil {
		return RechargeShopsData{Shops: []RechargeShopItem{}}
	}
	return d
}

// VisibleRechargeShops 过滤为已启用条目并按 sort 升序。
func VisibleRechargeShops(d RechargeShopsData) RechargeShopsData {
	out := RechargeShopsData{
		Title: strings.TrimSpace(d.Title),
		Hint:  strings.TrimSpace(d.Hint),
		Shops: []RechargeShopItem{},
	}
	for _, it := range d.Shops {
		if it.Enabled && strings.TrimSpace(it.URL) != "" && strings.TrimSpace(it.Label) != "" {
			out.Shops = append(out.Shops, it)
		}
	}
	sort.SliceStable(out.Shops, func(i, j int) bool {
		if out.Shops[i].Sort != out.Shops[j].Sort {
			return out.Shops[i].Sort < out.Shops[j].Sort
		}
		return out.Shops[i].Label < out.Shops[j].Label
	})
	return out
}

// RechargeShopsHasVisibleContent 是否存在至少一条已启用的商铺。
func RechargeShopsHasVisibleContent(d RechargeShopsData) bool {
	return len(VisibleRechargeShops(d).Shops) > 0
}

// ParseAndSanitizeRechargeShops 解析 JSON 并裁剪空白。
func ParseAndSanitizeRechargeShops(raw []byte) (RechargeShopsData, error) {
	if len(raw) > rechargeShopsMaxBytes {
		return RechargeShopsData{}, errRechargeShopsTooLarge
	}
	var wire struct {
		Title string             `json:"title"`
		Hint  string             `json:"hint"`
		Shops []RechargeShopItem `json:"shops"`
	}
	if err := json.Unmarshal(raw, &wire); err != nil {
		return RechargeShopsData{}, errRechargeShopsJSON
	}
	out := RechargeShopsData{
		Title: strings.TrimSpace(wire.Title),
		Hint:  strings.TrimSpace(wire.Hint),
		Shops: []RechargeShopItem{},
	}
	if utf8.RuneCountInString(out.Title) > rechargeShopsMaxTitle {
		return RechargeShopsData{}, errRechargeShopsTitleTooLong
	}
	if utf8.RuneCountInString(out.Hint) > rechargeShopsMaxHint {
		return RechargeShopsData{}, errRechargeShopsHintTooLong
	}
	if len(wire.Shops) > rechargeShopsMaxItems {
		return RechargeShopsData{}, errRechargeShopsTooMany
	}
	for _, it := range wire.Shops {
		label := strings.TrimSpace(it.Label)
		url := strings.TrimSpace(it.URL)
		desc := strings.TrimSpace(it.Description)
		if label == "" && url == "" && desc == "" {
			continue
		}
		if label == "" {
			return RechargeShopsData{}, errRechargeShopsLabelRequired
		}
		if url == "" {
			return RechargeShopsData{}, errRechargeShopsURLRequired
		}
		if utf8.RuneCountInString(label) > rechargeShopsMaxLabel {
			return RechargeShopsData{}, errRechargeShopsLabelTooLong
		}
		if utf8.RuneCountInString(desc) > rechargeShopsMaxDescription {
			return RechargeShopsData{}, errRechargeShopsDescTooLong
		}
		if len(url) > rechargeShopsMaxURLLen {
			return RechargeShopsData{}, errRechargeShopsURLTooLong
		}
		if err := validateHTTPURL(url); err != nil {
			return RechargeShopsData{}, errors.New("shops.url: " + err.Error())
		}
		qrURL := strings.TrimSpace(it.QRImageURL)
		if qrURL != "" {
			if len(qrURL) > rechargeShopsMaxURLLen {
				return RechargeShopsData{}, errRechargeShopsQRURLTooLong
			}
			if err := ValidateBrandIconURL(qrURL); err != nil {
				return RechargeShopsData{}, errors.New("shops.qr_image_url: " + err.Error())
			}
		}
		out.Shops = append(out.Shops, RechargeShopItem{
			Label:       label,
			URL:         url,
			Description: desc,
			Enabled:     it.Enabled,
			Sort:        it.Sort,
			ShowQR:      it.ShowQR,
			QRImageURL:  qrURL,
		})
	}
	return out, nil
}

// ValidateRechargeShopsJSON 写入 system_configs 前校验；空串表示清空。
func ValidateRechargeShopsJSON(s string) error {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	if len(s) > rechargeShopsMaxBytes {
		return errRechargeShopsTooLarge
	}
	_, err := ParseAndSanitizeRechargeShops([]byte(s))
	return err
}

// CanonicalRechargeShopsJSON 将输入规范为紧凑 JSON，供持久化。
func CanonicalRechargeShopsJSON(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return `{}`, nil
	}
	d, err := ParseAndSanitizeRechargeShops([]byte(s))
	if err != nil {
		return "", err
	}
	b, err := json.Marshal(d)
	if err != nil {
		return "", errRechargeShopsJSON
	}
	return string(b), nil
}

// MarshalRechargeShopsData 将结构体序列化为 JSON 字符串。
func MarshalRechargeShopsData(d RechargeShopsData) (string, error) {
	b, err := json.Marshal(d)
	if err != nil {
		return "", errRechargeShopsJSON
	}
	return string(b), nil
}
