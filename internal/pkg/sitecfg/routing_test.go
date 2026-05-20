package sitecfg

import "testing"

func TestDefaultRoutingDefaults_failoverEnabled(t *testing.T) {
	d := DefaultRoutingDefaults()
	if !d.FailoverEnabled {
		t.Fatal("expected failover enabled by default")
	}
	if d.FailoverMaxAttempts != 2 {
		t.Fatalf("max attempts=%d", d.FailoverMaxAttempts)
	}
}
