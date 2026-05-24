package gateway

import (
	"encoding/json"
	"testing"

	"tracehub/internal/ent"
)

func TestUpstreamSupportsOpenAIStreamOptions(t *testing.T) {
	cases := []struct {
		name string
		ch   *ent.Channel
		want bool
	}{
		{
			name: "siliconflow url with openai type",
			ch:   &ent.Channel{ChannelType: "openai", APIBaseURL: "https://api.siliconflow.cn/v1"},
			want: false,
		},
		{
			name: "custom proxy without siliconflow in host",
			ch:   &ent.Channel{ChannelType: "openai", APIBaseURL: "https://llm-proxy.example.com/v1"},
			want: false,
		},
		{
			name: "deepseek",
			ch:   &ent.Channel{ChannelType: "deepseek", APIBaseURL: "https://api.deepseek.com/v1"},
			want: false,
		},
		{
			name: "openai official",
			ch:   &ent.Channel{ChannelType: "openai", APIBaseURL: "https://api.openai.com/v1"},
			want: true,
		},
		{
			name: "azure openai",
			ch:   &ent.Channel{ChannelType: "openai", APIBaseURL: "https://myresource.openai.azure.com/openai/deployments/gpt-4o"},
			want: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := upstreamSupportsOpenAIStreamOptions(tc.ch); got != tc.want {
				t.Fatalf("got %v want %v (base=%q)", got, tc.want, effectiveChannelBaseURL(tc.ch))
			}
		})
	}
}

func TestMaybeInjectStreamUsageJSON_siliconflowSkips(t *testing.T) {
	in := []byte(`{"model":"Qwen/Qwen3-14B","messages":[],"stream":true}`)
	ch := &ent.Channel{ChannelType: "openai", APIBaseURL: "https://api.siliconflow.cn/v1"}
	out := maybeInjectStreamUsageJSON(in, ch)
	if string(out) != string(in) {
		t.Fatalf("expected unchanged body, got %s", out)
	}
}

func TestMaybeInjectStreamUsageJSON_openaiInjects(t *testing.T) {
	in := []byte(`{"model":"gpt-4o","messages":[],"stream":true}`)
	ch := &ent.Channel{ChannelType: "openai", APIBaseURL: "https://api.openai.com/v1"}
	out := maybeInjectStreamUsageJSON(in, ch)
	var m map[string]json.RawMessage
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatal(err)
	}
	if _, ok := m["stream_options"]; !ok {
		t.Fatal("expected stream_options")
	}
}
