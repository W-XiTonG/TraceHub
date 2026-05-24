package sitecfg

import "testing"

func TestValidateUpstreamTimeoutStandardSec(t *testing.T) {
	if err := ValidateUpstreamTimeoutStandardSec("120"); err != nil {
		t.Fatal(err)
	}
	if err := ValidateUpstreamTimeoutStandardSec("9"); err == nil {
		t.Fatal("expected range error")
	}
}

func TestValidateUpstreamTimeoutVideoSec(t *testing.T) {
	if err := ValidateUpstreamTimeoutVideoSec("1800"); err != nil {
		t.Fatal(err)
	}
	if err := ValidateUpstreamTimeoutVideoSec("30"); err == nil {
		t.Fatal("expected range error")
	}
}
