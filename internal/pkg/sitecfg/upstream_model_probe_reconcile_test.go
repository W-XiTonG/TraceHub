package sitecfg

import "testing"

func TestValidateUpstreamModelProbeReconcileScope(t *testing.T) {
	if err := ValidateUpstreamModelProbeReconcileScope("models_only"); err != nil {
		t.Fatal(err)
	}
	if err := ValidateUpstreamModelProbeReconcileScope("models_and_alias"); err != nil {
		t.Fatal(err)
	}
	if err := ValidateUpstreamModelProbeReconcileScope("invalid"); err == nil {
		t.Fatal("expected error")
	}
}

func TestValidateUpstreamModelProbeReconcileBool(t *testing.T) {
	if err := ValidateUpstreamModelProbeReconcileBool("true"); err != nil {
		t.Fatal(err)
	}
	if err := ValidateUpstreamModelProbeReconcileBool("maybe"); err == nil {
		t.Fatal("expected error")
	}
}
