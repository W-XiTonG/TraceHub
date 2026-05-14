package sitecfg

import (
	"context"
	"encoding/json"
	"errors"
	"net/mail"
	"strings"
	"unicode/utf8"

	"tracehub/internal/ent"
)

// SystemConfigKeyPublicContactJSON 官网「联系我们」区块：JSON，见 ParseAndSanitizePublicContact。
const SystemConfigKeyPublicContactJSON = "site.public_contact"

const (
	publicContactMaxBytes     = 16 * 1024
	publicContactMaxTitle     = 80
	publicContactMaxQRItems   = 10
	publicContactMaxEmails    = 10
	publicContactMaxQRLabel   = 80
	publicContactMaxEmailLen  = 120
)

// PublicContactQRItem 单张二维码（对外展示）。
type PublicContactQRItem struct {
	Label    string `json:"label"`
	ImageURL string `json:"image_url"`
}

// PublicContactData 解析并校验后的联系我们配置。
type PublicContactData struct {
	Title   string                `json:"title"`
	QRItems []PublicContactQRItem `json:"qr_items"`
	Emails  []string              `json:"emails"`
}

// StoredPublicContactJSON 数据库中的原始 JSON（trim 后空表示未配置）。
func StoredPublicContactJSON(ctx context.Context, client *ent.Client) string {
	v, ok, err := stringFromConfigRow(ctx, client, SystemConfigKeyPublicContactJSON)
	if err != nil || !ok {
		return ""
	}
	return strings.TrimSpace(v)
}

// EffectivePublicContactData 对外使用的联系我们数据；非法或缺失时返回空结构。
func EffectivePublicContactData(ctx context.Context, client *ent.Client) PublicContactData {
	raw := StoredPublicContactJSON(ctx, client)
	if raw == "" {
		return PublicContactData{QRItems: []PublicContactQRItem{}, Emails: []string{}}
	}
	d, err := ParseAndSanitizePublicContact([]byte(raw))
	if err != nil {
		return PublicContactData{QRItems: []PublicContactQRItem{}, Emails: []string{}}
	}
	return d
}

// PublicContactHasVisibleContent 是否应在官网展示联系我们区块。
func PublicContactHasVisibleContent(d PublicContactData) bool {
	if strings.TrimSpace(d.Title) != "" {
		return true
	}
	if len(d.QRItems) > 0 || len(d.Emails) > 0 {
		return true
	}
	return false
}

// ParseAndSanitizePublicContact 解析 JSON 并裁剪空白、丢弃无效条目。
func ParseAndSanitizePublicContact(raw []byte) (PublicContactData, error) {
	if len(raw) > publicContactMaxBytes {
		return PublicContactData{}, errPublicContactTooLarge
	}
	var wire struct {
		Title   string                `json:"title"`
		QRItems []PublicContactQRItem `json:"qr_items"`
		Emails  []string              `json:"emails"`
	}
	if err := json.Unmarshal(raw, &wire); err != nil {
		return PublicContactData{}, errPublicContactJSON
	}
	out := PublicContactData{
		Title:   strings.TrimSpace(wire.Title),
		QRItems: []PublicContactQRItem{},
		Emails:  []string{},
	}
	if utf8.RuneCountInString(out.Title) > publicContactMaxTitle {
		return PublicContactData{}, errPublicContactTitleTooLong
	}
	if len(wire.QRItems) > publicContactMaxQRItems {
		return PublicContactData{}, errPublicContactTooManyQR
	}
	for _, it := range wire.QRItems {
		label := strings.TrimSpace(it.Label)
		url := strings.TrimSpace(it.ImageURL)
		if label == "" && url == "" {
			continue
		}
		if utf8.RuneCountInString(label) > publicContactMaxQRLabel {
			return PublicContactData{}, errPublicContactQRLabelTooLong
		}
		if url == "" {
			return PublicContactData{}, errPublicContactQRURLRequired
		}
		if err := ValidateBrandIconURL(url); err != nil {
			return PublicContactData{}, errors.New("qr_items.image_url: " + err.Error())
		}
		out.QRItems = append(out.QRItems, PublicContactQRItem{Label: label, ImageURL: url})
	}
	if len(wire.Emails) > publicContactMaxEmails {
		return PublicContactData{}, errPublicContactTooManyEmails
	}
	for _, s0 := range wire.Emails {
		s := strings.TrimSpace(s0)
		if s == "" {
			continue
		}
		if err := validateContactEmail(s); err != nil {
			return PublicContactData{}, err
		}
		out.Emails = append(out.Emails, s)
	}
	return out, nil
}

func validateContactEmail(s string) error {
	if len(s) > publicContactMaxEmailLen {
		return errPublicContactEmailTooLong
	}
	addr, err := mail.ParseAddress(s)
	if err != nil {
		return errPublicContactEmailInvalid
	}
	if strings.TrimSpace(addr.Name) != "" {
		return errPublicContactEmailInvalid
	}
	em := strings.TrimSpace(addr.Address)
	if em == "" || em != s {
		return errPublicContactEmailInvalid
	}
	return nil
}

// ValidatePublicContactJSON 写入 system_configs 前校验；空串表示清空为默认（存 {}）。
func ValidatePublicContactJSON(s string) error {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	if len(s) > publicContactMaxBytes {
		return errPublicContactTooLarge
	}
	_, err := ParseAndSanitizePublicContact([]byte(s))
	return err
}

// CanonicalPublicContactJSON 将输入规范为紧凑 JSON，供持久化。
func CanonicalPublicContactJSON(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return `{}`, nil
	}
	d, err := ParseAndSanitizePublicContact([]byte(s))
	if err != nil {
		return "", err
	}
	b, err := json.Marshal(d)
	if err != nil {
		return "", errPublicContactJSON
	}
	return string(b), nil
}
