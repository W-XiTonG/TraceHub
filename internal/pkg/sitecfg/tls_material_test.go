package sitecfg

import "testing"

func TestEvaluateTLSMaterial_empty(t *testing.T) {
	st := evaluateTLSMaterialPaths(TLSCertKeyPaths{})
	if st.MaterialReady {
		t.Fatal("expected not ready")
	}
	if err := ValidateTLSMaterialStatus(st); err == nil {
		t.Fatal("expected error")
	}
}
