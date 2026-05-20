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

func TestResolveModelsListBaseURL_VolcengineStripsChatSuffix(t *testing.T) {
	raw := "https://ark.cn-beijing.volces.com/api/v3/chat/completions"
	base, err := ResolveModelsListBaseURL("volcengine", raw)
	if err != nil {
		t.Fatal(err)
	}
	want := "https://ark.cn-beijing.volces.com/api/v3"
	if base != want {
		t.Fatalf("got %q want %q", base, want)
	}
}

func TestResolveModelsListBaseURL_EmptyUsesChannelDefault(t *testing.T) {
	base, err := ResolveModelsListBaseURL("volcengine", "")
	if err != nil {
		t.Fatal(err)
	}
	if base != "https://ark.cn-beijing.volces.com/api/v3" {
		t.Fatalf("got %q", base)
	}
}

func TestParseModelIDsFromResponseBody_ArkNameFallback(t *testing.T) {
	body := []byte(`{"data":[{"name":"doubao-seedance-2-0-260128"}]}`)
	ids, err := ParseModelIDsFromResponseBody(body)
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 1 || ids[0] != "doubao-seedance-2-0-260128" {
		t.Fatalf("got %#v", ids)
	}
}
