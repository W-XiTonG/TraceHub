package channelmeta

import "testing"

func TestModelMatchesChannelType_volcengine_doubaoChat(t *testing.T) {
	if !ModelMatchesChannelType("doubao-lite-128k-240428", "volcengine") {
		t.Fatal("expected doubao chat model to match volcengine")
	}
	if !ModelMatchesChannelType("doubao-seedance-2-0-260128", "volcengine") {
		t.Fatal("expected seedance model to match volcengine")
	}
	if ModelMatchesChannelType("gpt-4o-mini", "volcengine") {
		t.Fatal("gpt should not match volcengine")
	}
}
