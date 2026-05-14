package sitecfg

import "errors"

var (
	errInvalidLoginCaptchaBool = errors.New("site.login_captcha.enabled 须为 true 或 false")
	errBrandNameInvalid        = errors.New("site.brand.name 不能包含换行或控制字符，且长度不超过 64 个字符")
	errBrandNameTooLong        = errors.New("site.brand.name 长度不能超过 64 个字符")
	errBrandIconTooLong        = errors.New("site.brand.icon_url 过长")
	errBrandIconPath           = errors.New("site.brand.icon_url 相对路径不能包含 ..")
	errBrandIconURL            = errors.New("site.brand.icon_url 须为以 / 开头的路径或 http(s) URL")
	errOperatorTooLong         = errors.New("site.operator_display_name 长度不能超过 200 个字符")
	errRegisterPromoTooLong    = errors.New("site.register_promo_text 长度不能超过 1000 个字符")

	errDefaultNewUserBalanceEmpty   = errors.New("site.default_new_user_balance 不能为空")
	errDefaultNewUserBalanceInvalid = errors.New("site.default_new_user_balance 须为非负十进制整数")
	errDefaultNewUserBalanceRange   = errors.New("site.default_new_user_balance 超出允许范围")

	errAPIPublicBaseURLInvalid = errors.New("site.api.public_base_url 须为 http(s) URL，路径仅可为空或 /v1")
	errAPIPublicBaseURLPath    = errors.New("site.api.public_base_url 路径仅允许为空或 /v1")
	errAPIPublicBaseURLTooLong = errors.New("site.api.public_base_url 过长")
)
