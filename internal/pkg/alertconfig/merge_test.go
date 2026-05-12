package alertconfig

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestApplySecretPatch(t *testing.T) {
	base := Stored{DingTalkWebhook: "https://oapi.dingtalk.com/robot/send?access_token=x", DingTalkSecret: "old"}
	raw := []byte(`{"dingtalk":"SEC_NEW"}`)
	out, err := ApplySecretPatch(base, raw)
	if err != nil {
		t.Fatal(err)
	}
	if out.DingTalkSecret != "SEC_NEW" || out.DingTalkWebhook != base.DingTalkWebhook {
		t.Fatalf("%+v", out)
	}
	out2, err := ApplySecretPatch(out, []byte(`{"dingtalk":""}`))
	if err != nil {
		t.Fatal(err)
	}
	if out2.DingTalkSecret != "" {
		t.Fatal("clear failed")
	}
}

func TestPublicViewNoSecrets(t *testing.T) {
	s := Stored{DingTalkSecret: "SEC", DingTalkWebhook: "https://x"}
	b, _ := json.Marshal(PublicView(s))
	if string(b) == "" {
		t.Fatal("empty")
	}
	if strings.Contains(string(b), "SEC") {
		t.Fatalf("secret leaked: %s", b)
	}
}
