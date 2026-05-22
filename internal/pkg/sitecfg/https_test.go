package sitecfg

import "testing"

func TestValidateHTTPSFields(t *testing.T) {
	if err := ValidateHTTPSForceRedirect("true"); err != nil {
		t.Fatal(err)
	}
	if err := ValidateHTTPSHSTSMaxAge("31536000"); err != nil {
		t.Fatal(err)
	}
	if err := ValidateHTTPSHSTSMaxAge("63072001"); err == nil {
		t.Fatal("expected range error")
	}
	if err := ValidateTLSEnabled("false"); err != nil {
		t.Fatal(err)
	}
}

func TestFileHTTPSPolicy_defaults(t *testing.T) {
	p := fileHTTPSPolicy(nil)
	if p.HSTSMaxAgeSeconds != defaultHSTSMaxAgeSeconds {
		t.Fatalf("got %d", p.HSTSMaxAgeSeconds)
	}
}
