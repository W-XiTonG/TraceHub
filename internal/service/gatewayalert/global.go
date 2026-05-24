package gatewayalert

import (
	"sync/atomic"
)

var globalNotifier atomic.Pointer[Notifier]

// SetGlobal 在 server 初始化时注册全局告警实例。
func SetGlobal(n *Notifier) {
	globalNotifier.Store(n)
}

// Global 返回全局告警实例（可能为 nil）。
func Global() *Notifier {
	return globalNotifier.Load()
}
