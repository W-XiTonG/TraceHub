package billing

// adminEngine 供管理端更新 site.no_bill_models 后失效计费缓存（router 启动时注册）。
var adminEngine *Engine

// SetAdminEngine 在 server 初始化时注册全局计费引擎实例。
func SetAdminEngine(e *Engine) {
	adminEngine = e
}

// InvalidateNoBillModelsCacheAdmin 管理端写入 site.no_bill_models 后调用。
func InvalidateNoBillModelsCacheAdmin() {
	if adminEngine != nil {
		adminEngine.InvalidateNoBillModelsCache()
	}
}

// InvalidateGlobalModelPricingCacheAdmin 管理端更新全局模型计价后调用。
func InvalidateGlobalModelPricingCacheAdmin() {
	if adminEngine != nil {
		adminEngine.InvalidateGlobalModelPricingCache()
	}
}
