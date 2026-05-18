package upstreamprobe

import "testing"

func BenchmarkNotifyCapabilityOutcome(b *testing.B) {
	runtimeOn = true
	notifyCh = make(chan notifyMsg, notifyBuf)
	go func() {
		for range notifyCh {
		}
	}()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		NotifyCapabilityOutcome(1, "grok-imagine-video", CapabilityVideo, 400, []byte(`{"code":"x"}`), CapSourceRuntime)
	}
}
