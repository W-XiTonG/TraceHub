package usagelogpublic

import "testing"

func TestUserFacing_redactsInternal(t *testing.T) {
	raw := WrapInternal("没有可用渠道支持模型 gpt-4o")
	got := UserFacing(&raw, ptr(403))
	if got == nil || *got != "请求被拒绝" {
		t.Fatalf("got %v want 请求被拒绝", got)
	}
}

func TestAdminFacing_showsDetail(t *testing.T) {
	raw := WrapInternal("没有可用渠道支持模型 gpt-4o")
	got := AdminFacing(&raw)
	if got == nil || *got != "没有可用渠道支持模型 gpt-4o" {
		t.Fatalf("got %v", got)
	}
}

func TestUserFacing_allowsUpstreamStatusText(t *testing.T) {
	msg := "Unauthorized"
	got := UserFacing(&msg, ptr(401))
	if got == nil || *got != msg {
		t.Fatalf("got %v want %s", got, msg)
	}
}

func ptr(n int) *int { return &n }

func TestSanitizeGatewayErrorMessage_selectChannel(t *testing.T) {
	client, internal := SanitizeGatewayErrorMessage(403, "没有可用渠道支持模型 gpt-5.")
	if client != "请求被拒绝" || internal != "没有可用渠道支持模型 gpt-5." {
		t.Fatalf("client=%q internal=%q", client, internal)
	}
}

func TestSanitizeGatewayErrorMessage_userSafePassThrough(t *testing.T) {
	client, internal := SanitizeGatewayErrorMessage(401, "无效的 API Key")
	if client != "无效的 API Key" || internal != "" {
		t.Fatalf("client=%q internal=%q", client, internal)
	}
}
