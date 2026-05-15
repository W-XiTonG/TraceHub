package sitecfg

import "testing"

func TestValidateGatewayIPBlacklistJSON(t *testing.T) {
	if err := ValidateGatewayIPBlacklistJSON(""); err != nil {
		t.Fatal(err)
	}
	if err := ValidateGatewayIPBlacklistJSON("[]"); err != nil {
		t.Fatal(err)
	}
	if err := ValidateGatewayIPBlacklistJSON(`["203.0.113.1","2001:db8::1","10.0.0.0/8"]`); err != nil {
		t.Fatal(err)
	}
	if err := ValidateGatewayIPBlacklistJSON(`["not-an-ip"]`); err == nil {
		t.Fatal("expected error")
	}
	if err := ValidateGatewayIPBlacklistJSON(`["10.0.0.0/33"]`); err == nil {
		t.Fatal("expected error")
	}
	if err := ValidateGatewayIPBlacklistJSON(`["  "]`); err == nil {
		t.Fatal("expected error")
	}
	if err := ValidateGatewayIPBlacklistJSON("not json"); err == nil {
		t.Fatal("expected error")
	}
}
