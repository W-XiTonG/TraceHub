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
	errNoBillModelsJSON        = errors.New("site.no_bill_models 须为 JSON 字符串数组")
	errNoBillModelsEmptyID     = errors.New("site.no_bill_models 数组元素不能为空")

	errDefaultNewUserBalanceEmpty   = errors.New("site.default_new_user_balance 不能为空")
	errDefaultNewUserBalanceInvalid = errors.New("site.default_new_user_balance 须为非负十进制整数")
	errDefaultNewUserBalanceRange   = errors.New("site.default_new_user_balance 超出允许范围")

	errCurrencyQuotaPerYuanEmpty   = errors.New("site.currency_quota_per_yuan 不能为空")
	errCurrencyQuotaPerYuanInvalid = errors.New("site.currency_quota_per_yuan 须为大于 0 的整数")
	errCurrencyQuotaPerUsdEmpty    = errors.New("site.currency_quota_per_usd 不能为空")
	errCurrencyQuotaPerUsdInvalid  = errors.New("site.currency_quota_per_usd 须为非负整数")
	errPricingCnyPerUsdEmpty       = errors.New("site.pricing_cny_per_usd 不能为空")
	errPricingCnyPerUsdInvalid     = errors.New("site.pricing_cny_per_usd 须为非负数字")

	errDefaultPricingMarkupEmpty   = errors.New("site.default_pricing_markup_percent 不能为空")
	errDefaultPricingMarkupInvalid = errors.New("site.default_pricing_markup_percent 须为有效数字")
	errDefaultPricingMarkupTooLow  = errors.New("site.default_pricing_markup_percent 须大于 -100")

	errAPIPublicBaseURLInvalid = errors.New("site.api.public_base_url 须为 http(s) URL，路径仅可为空或 /v1")
	errAPIPublicBaseURLPath    = errors.New("site.api.public_base_url 路径仅允许为空或 /v1")
	errAPIPublicBaseURLTooLong = errors.New("site.api.public_base_url 过长")

	errPublicContactTooLarge       = errors.New("site.public_contact 体积过大")
	errPublicContactJSON           = errors.New("site.public_contact 须为合法 JSON")
	errPublicContactTitleTooLong   = errors.New("site.public_contact.title 过长")
	errPublicContactTooManyQR      = errors.New("site.public_contact.qr_items 数量过多")
	errPublicContactTooManyEmails  = errors.New("site.public_contact.emails 数量过多")
	errPublicContactQRLabelTooLong = errors.New("site.public_contact.qr_items[].label 过长")
	errPublicContactQRURLRequired  = errors.New("site.public_contact.qr_items 填写说明时须填写 image_url")
	errPublicContactEmailInvalid   = errors.New("site.public_contact.emails 中存在无效邮箱（仅支持纯邮箱，不含显示名）")
	errPublicContactEmailTooLong   = errors.New("site.public_contact.emails 单项过长")

	errInvalidUpstreamProbeEnabled = errors.New("upstream_model_probe.enabled 须为 true 或 false")
	errInvalidUpstreamProbeInterval = errors.New("upstream_model_probe.interval_minutes 须为整数")
	errUpstreamProbeIntervalRange    = errors.New("upstream_model_probe.interval_minutes 须在 15–1440 之间")

	errInvalidUpstreamCapabilityFailStale = errors.New("upstream_capability.fail_stale_hours 须为整数")
	errUpstreamCapabilityFailStaleRange    = errors.New("upstream_capability.fail_stale_hours 须在 0–168 之间")
	errInvalidUpstreamCapabilityProbeEnabled = errors.New("upstream_capability_probe.enabled 须为 true 或 false")

	errRoutingStrategyInvalid        = errors.New("gateway.routing.strategy_default 须为 weighted_random、priority 或 fixed_order")
	errRoutingFailoverBool           = errors.New("gateway.routing.failover_enabled_default 须为 true 或 false")
	errRoutingFailoverAttempts       = errors.New("gateway.routing.failover_max_attempts_default 须为整数")
	errRoutingFailoverAttemptsRange  = errors.New("gateway.routing.failover_max_attempts_default 须在 1–5 之间")
	errRoutingStickyBool             = errors.New("gateway.routing.sticky_enabled_default 须为 true 或 false")
	errRoutingAutoHealthBool         = errors.New("gateway.routing.auto_health_from_probe 须为 true 或 false")
	errRoutingProbeThreshold         = errors.New("gateway.routing.probe_fail_threshold 须为整数")
	errRoutingProbeThresholdRange    = errors.New("gateway.routing.probe_fail_threshold 须在 1–20 之间")
)
