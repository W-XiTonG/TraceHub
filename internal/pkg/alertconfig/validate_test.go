package alertconfig

import "testing"

func TestNormalizeAndValidate(t *testing.T) {
	_, err := NormalizeAndValidate(Stored{Emails: []string{"not-an-email"}})
	if err == nil {
		t.Fatal("expected error")
	}
	_, err = NormalizeAndValidate(Stored{
		Emails:          []string{"  A@b.com ", "a@b.com"},
		DingTalkWebhook: "http://x.com",
	})
	if err == nil {
		t.Fatal("expected https error")
	}
	out, err := NormalizeAndValidate(Stored{
		Emails:          []string{"ok@example.com"},
		DingTalkWebhook: "https://oapi.dingtalk.com/robot/send?access_token=abc",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Emails) != 1 || out.Emails[0] != "ok@example.com" {
		t.Fatalf("emails: %+v", out.Emails)
	}
}
