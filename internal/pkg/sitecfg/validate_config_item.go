package sitecfg

import (
	"strconv"
	"strings"
)

// ValidateSystemConfigPut 管理端写入 system_configs 前的白名单校验；未知 key 放行。
func ValidateSystemConfigPut(key, value string) error {
	k := strings.TrimSpace(key)
	switch k {
	case SystemConfigKeyLoginCaptchaEnabled:
		if _, err := strconv.ParseBool(strings.TrimSpace(value)); err != nil {
			return errInvalidLoginCaptchaBool
		}
		return nil
	case SystemConfigKeyBrandName:
		return ValidateBrandName(value)
	case SystemConfigKeyBrandIconURL:
		return ValidateBrandIconURL(value)
	case SystemConfigKeyBrandFaviconURL:
		return ValidateBrandIconURL(value)
	case SystemConfigKeyBrandOGImageURL:
		return ValidateBrandIconURL(value)
	case SystemConfigKeyBrandContactCardURL:
		return ValidateBrandIconURL(value)
	case SystemConfigKeyOperatorDisplayName:
		return ValidateOperatorDisplayName(value)
	case SystemConfigKeyDefaultNewUserBalance:
		return ValidateDefaultNewUserBalance(value)
	case SystemConfigKeyAPIPublicBaseURL:
		return ValidateAPIPublicBaseURL(value)
	case SystemConfigKeyRegisterPromoText:
		return ValidateRegisterPromoText(value)
	case SystemConfigKeyNoBillModels:
		return ValidateNoBillModelsJSON(value)
	case SystemConfigKeyPublicContactJSON:
		return ValidatePublicContactJSON(value)
	case SystemConfigKeyUpstreamModelProbeEnabled:
		return ValidateUpstreamModelProbeEnabled(value)
	case SystemConfigKeyUpstreamModelProbeIntervalMinutes:
		return ValidateUpstreamModelProbeIntervalMinutes(value)
	case SystemConfigKeyUpstreamCapabilityFailStaleHours:
		return ValidateUpstreamCapabilityFailStaleHours(value)
	case SystemConfigKeyUpstreamCapabilityProbeEnabled:
		return ValidateUpstreamCapabilityProbeEnabled(value)
	case SystemConfigKeyRoutingStrategyDefault:
		return ValidateRoutingStrategy(value)
	case SystemConfigKeyRoutingFailoverEnabledDefault:
		return ValidateRoutingFailoverEnabled(value)
	case SystemConfigKeyRoutingFailoverMaxAttempts:
		return ValidateRoutingFailoverMaxAttempts(value)
	case SystemConfigKeyRoutingStickyEnabledDefault:
		return ValidateRoutingStickyEnabled(value)
	case SystemConfigKeyRoutingAutoHealthFromProbe:
		return ValidateRoutingAutoHealthFromProbe(value)
	case SystemConfigKeyRoutingProbeFailThreshold:
		return ValidateRoutingProbeFailThreshold(value)
	case SystemConfigKeyGatewayIPBlacklist:
		return ValidateGatewayIPBlacklistJSON(value)
	case SystemConfigKeyCurrencyQuotaPerYuan:
		return ValidateCurrencyQuotaPerYuan(value)
	case SystemConfigKeyCurrencyQuotaPerUsd:
		return ValidateCurrencyQuotaPerUsd(value)
	case SystemConfigKeyPricingCnyPerUsd:
		return ValidatePricingCnyPerUsd(value)
	case SystemConfigKeyDefaultPricingMarkupPercent:
		return ValidateDefaultPricingMarkupPercent(value)
	default:
		return nil
	}
}
