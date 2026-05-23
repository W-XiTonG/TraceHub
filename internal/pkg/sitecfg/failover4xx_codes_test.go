package sitecfg

import "testing"

func TestParseFailover4xxCodes(t *testing.T) {
	codes, err := ParseFailover4xxCodes("401, 403,400")
	if err != nil {
		t.Fatal(err)
	}
	want := []int{400, 401, 403}
	if len(codes) != len(want) {
		t.Fatalf("got %v want %v", codes, want)
	}
	for i := range want {
		if codes[i] != want[i] {
			t.Fatalf("got %v want %v", codes, want)
		}
	}
}

func TestEffectiveFailover4xxCodes_selectiveDefault(t *testing.T) {
	got := EffectiveFailover4xxCodes(Failover4xxModeSelective, nil)
	if len(got) != len(DefaultSelectiveFailover4xxCodes) {
		t.Fatalf("expected default selective codes")
	}
}

func TestValidateRoutingFailover4xxCodesForMode_customRequiresCodes(t *testing.T) {
	if err := ValidateRoutingFailover4xxCodesForMode(Failover4xxModeCustom, nil); err == nil {
		t.Fatal("expected error")
	}
}
