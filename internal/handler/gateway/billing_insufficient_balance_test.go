package gateway

import (
	"strings"
	"testing"
)

func TestBalancePrecheckInsufficientMessage_includesFigures(t *testing.T) {
	s := balancePrecheckInsufficientMessage(100, 500, 50, 2048)
	for _, sub := range []string{"100", "500", "400", "50", "2048", "还差"} {
		if !strings.Contains(s, sub) {
			t.Fatalf("want substring %q in %q", sub, s)
		}
	}
	if strings.Contains(s, "403") {
		t.Fatalf("precheck message should not steer users to 403: %q", s)
	}
}

func TestBalancePrecheckInsufficientMessage_zeroCompletionPhrase(t *testing.T) {
	s := balancePrecheckInsufficientMessage(1, 2, 3, 0)
	if !strings.Contains(s, "0 token") {
		t.Fatalf("expected zero-completion phrase, got %q", s)
	}
	if !strings.Contains(s, "还差") {
		t.Fatalf("expected shortfall phrase, got %q", s)
	}
}

func TestInsufficientBalancePostDebitMessage_shortfall(t *testing.T) {
	s := insufficientBalancePostDebitMessage(10, 100)
	if !strings.Contains(s, "还差") || !strings.Contains(s, "90") {
		t.Fatalf("got %q", s)
	}
}
