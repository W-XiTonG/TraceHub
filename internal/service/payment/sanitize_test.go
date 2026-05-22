package payment

import "testing"

func TestSanitizePostgresTextStripsNUL(t *testing.T) {
	in := "https://pay.example.com/\x00pay"
	got := sanitizePostgresText(in, 2000)
	if stringsContainsNUL(got) {
		t.Fatalf("still has nul: %q", got)
	}
	if got != "https://pay.example.com/pay" {
		t.Fatalf("got %q", got)
	}
}

func stringsContainsNUL(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] == 0 {
			return true
		}
	}
	return false
}
