package ipcheck

import "testing"

func TestBlocked(t *testing.T) {
	if Blocked("192.168.1.1", nil) {
		t.Fatal("empty blacklist should not block")
	}
	if Blocked("192.168.1.1", []string{}) {
		t.Fatal("empty blacklist should not block")
	}
	if !Blocked("192.168.1.1", []string{"192.168.1.1"}) {
		t.Fatal("exact match should block")
	}
	if Blocked("192.168.1.2", []string{"192.168.1.1"}) {
		t.Fatal("non-matching IP should not block")
	}
	if !Blocked("192.168.1.10", []string{"192.168.1.0/24"}) {
		t.Fatal("CIDR match should block")
	}
	if Blocked("10.0.0.1", []string{"192.168.1.0/24"}) {
		t.Fatal("outside CIDR should not block")
	}
	if Blocked("not-an-ip", []string{"192.168.1.1"}) {
		t.Fatal("unparsable client IP should not block")
	}
}
