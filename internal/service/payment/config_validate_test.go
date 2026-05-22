package payment

import "testing"

func TestValidateRuntimeConfigOnlineDisabled(t *testing.T) {
	cfg := RuntimeConfig{
		OnlineEnabled: false,
		EasyPayEnabled: true,
		EasyPayPID:     "",
		EasyPayKey:     "",
	}
	if err := ValidateRuntimeConfig(cfg); err != nil {
		t.Fatalf("expected no error when online disabled: %v", err)
	}
}

func TestMergeUpdateOnlineDisabled(t *testing.T) {
	cur := RuntimeConfig{OnlineEnabled: true, EasyPayEnabled: true, EasyPayPID: "1"}
	merged := MergeUpdate(cur, map[string]any{"online_enabled": false})
	if merged.OnlineEnabled {
		t.Fatal("online_enabled should be false")
	}
}
