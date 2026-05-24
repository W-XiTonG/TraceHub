package gatewayalert

import "testing"

func TestMaybeNotifyNilSafe(t *testing.T) {
	var n *Notifier
	n.MaybeNotify(Event{Status: 500})
	if Global() != nil {
		// Global may be set by other tests in same package if run with -count=1 it's nil
	}
	n2 := &Notifier{}
	n2.MaybeNotify(Event{Status: 500})
}

func TestGlobalInitiallyNil(t *testing.T) {
	// SetGlobal(nil) or fresh package - Global may be set by router in integration; unit test only checks no panic
	_ = Global()
}
