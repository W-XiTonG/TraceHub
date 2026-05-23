package pricingsync

import "errors"

var (
	errInvalidURL      = errors.New("无效的 URL")
	errHTTPSRequired   = errors.New("仅允许 HTTPS")
	errSSRFBlocked     = errors.New("目标地址被 SSRF 策略拒绝")
	errVersionMismatch = errors.New("不支持的 pricing payload 版本")
	errTargetDisabled  = errors.New("同步目标已禁用")
	errFeatureDisabled = errors.New("价格同步功能未开启")
	errPreviewExpired  = errors.New("preview_id 无效或已过期")
	errPreviewMismatch = errors.New("preview 与请求不匹配")
	errSyncInProgress  = errors.New("该目标正在同步中")
	errCooldown        = errors.New("同步冷却中，请稍后再试")
	errTooManyRows     = errors.New("同步行数超过上限")
	errDirection       = errors.New("direction 无效")

	ErrInvalidURL      = errInvalidURL
	ErrHTTPSRequired   = errHTTPSRequired
	ErrSSRFBlocked     = errSSRFBlocked
	ErrVersionMismatch = errVersionMismatch
	ErrTargetDisabled  = errTargetDisabled
	ErrFeatureDisabled = errFeatureDisabled
	ErrPreviewExpired  = errPreviewExpired
	ErrPreviewMismatch = errPreviewMismatch
	ErrSyncInProgress  = errSyncInProgress
	ErrCooldown        = errCooldown
	ErrTooManyRows     = errTooManyRows
	ErrDirection       = errDirection
)
