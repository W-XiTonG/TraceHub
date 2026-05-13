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
	case SystemConfigKeyOperatorDisplayName:
		return ValidateOperatorDisplayName(value)
	case SystemConfigKeyDefaultNewUserBalance:
		return ValidateDefaultNewUserBalance(value)
	case SystemConfigKeyAPIPublicBaseURL:
		return ValidateAPIPublicBaseURL(value)
	default:
		return nil
	}
}
