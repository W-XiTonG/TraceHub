package sitecfg

import "testing"

func TestNormalizeAPIPublicBaseURL(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in   string
		want string
	}{
		{"", ""},
		{"  ", ""},
		{"https://api.example.com", "https://api.example.com/v1"},
		{"https://api.example.com/", "https://api.example.com/v1"},
		{"https://api.example.com/v1", "https://api.example.com/v1"},
		{"https://api.example.com/v1/", "https://api.example.com/v1"},
		{"http://127.0.0.1:8080", "http://127.0.0.1:8080/v1"},
	}
	for _, tc := range cases {
		got, err := NormalizeAPIPublicBaseURL(tc.in)
		if err != nil {
			t.Fatalf("in %q: %v", tc.in, err)
		}
		if got != tc.want {
			t.Fatalf("in %q: got %q want %q", tc.in, got, tc.want)
		}
	}
	if _, err := NormalizeAPIPublicBaseURL("https://x/bad"); err == nil {
		t.Fatal("expected error for bad path")
	}
}
