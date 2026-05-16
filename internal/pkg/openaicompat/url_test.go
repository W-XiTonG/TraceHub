package openaicompat

import "testing"

func TestNormalizeOpenAIAPIRoot_VolcengineArk(t *testing.T) {
	got, err := NormalizeOpenAIAPIRoot("https://ark.cn-beijing.volces.com/api/v3")
	if err != nil {
		t.Fatal(err)
	}
	want := "https://ark.cn-beijing.volces.com/api/v3"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
