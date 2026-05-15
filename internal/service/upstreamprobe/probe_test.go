package upstreamprobe

import "testing"

func TestModelInUpstreamList(t *testing.T) {
	up := map[string]struct{}{"gpt-4o": {}, "deepseek-chat": {}}
	alias := map[string]string{"my-gpt": "gpt-4o", "alias-only": "missing"}

	if !modelInUpstreamList("gpt-4o", up, alias) {
		t.Fatal("direct id")
	}
	if !modelInUpstreamList("my-gpt", up, alias) {
		t.Fatal("alias to listed upstream")
	}
	if modelInUpstreamList("alias-only", up, alias) {
		t.Fatal("alias target missing")
	}
	if modelInUpstreamList("not-there", up, alias) {
		t.Fatal("unknown")
	}
}

func TestParseModelIDsFromResponseBody_OpenAI(t *testing.T) {
	body := []byte(`{"object":"list","data":[{"id":" a ","object":"model"},{"id":"b","object":"model"}]}`)
	ids, err := ParseModelIDsFromResponseBody(body)
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 2 || ids[0] != "a" || ids[1] != "b" {
		t.Fatalf("got %#v", ids)
	}
}

func TestParseModelIDsFromResponseBody_Generic(t *testing.T) {
	body := []byte(`{"models":[{"id":"x1"},{"id":"x2"}]}`)
	ids, err := ParseModelIDsFromResponseBody(body)
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 2 {
		t.Fatalf("got %#v", ids)
	}
}
