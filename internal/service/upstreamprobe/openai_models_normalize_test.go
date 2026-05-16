package upstreamprobe

import "testing"

func TestNormalizeOpenAICompatBaseURL_VolcengineArk(t *testing.T) {
	base, err := NormalizeOpenAICompatBaseURL("https://ark.cn-beijing.volces.com/api/v3")
	if err != nil {
		t.Fatal(err)
	}
	want := "https://ark.cn-beijing.volces.com/api/v3"
	if base != want {
		t.Fatalf("got %q want %q", base, want)
	}
}
