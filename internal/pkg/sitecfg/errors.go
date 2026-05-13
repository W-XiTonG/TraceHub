package sitecfg

import "errors"

var (
	errInvalidLoginCaptchaBool = errors.New("site.login_captcha.enabled 须为 true 或 false")
	errBrandNameInvalid        = errors.New("site.brand.name 不能包含换行或控制字符，且长度不超过 64 个字符")
	errBrandNameTooLong = errors.New("site.brand.name 长度不能超过 64 个字符")
	errBrandIconTooLong = errors.New("site.brand.icon_url 过长")
	errBrandIconPath    = errors.New("site.brand.icon_url 相对路径不能包含 ..")
	errBrandIconURL     = errors.New("site.brand.icon_url 须为以 / 开头的路径或 http(s) URL")
	errOperatorTooLong  = errors.New("site.operator_display_name 长度不能超过 200 个字符")
)
