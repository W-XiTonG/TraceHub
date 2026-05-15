package billing

import "testing"

func TestModelCountHundredPlusBadge(t *testing.T) {
	tests := []struct {
		n    int
		want string
	}{
		{0, ""},
		{299, ""},
		{300, "300+"},
		{350, "300+"},
		{399, "300+"},
		{400, "400+"},
		{1200, "1200+"},
	}
	for _, tt := range tests {
		if got := ModelCountHundredPlusBadge(tt.n); got != tt.want {
			t.Fatalf("ModelCountHundredPlusBadge(%d) = %q, want %q", tt.n, got, tt.want)
		}
	}
}
