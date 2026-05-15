package modelkind

import (
	"reflect"
	"testing"
)

func TestLabelsForModel_adminLabels(t *testing.T) {
	got := LabelsForModel("gpt-4o", nil, []string{"vision", "beta"})
	want := []string{"vision", "beta"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("admin tags skip infer chat = %v want %v", got, want)
	}
	got2 := LabelsForModel("gpt-4o", nil, []string{"vision"})
	want2 := []string{"vision"}
	if !reflect.DeepEqual(got2, want2) {
		t.Fatalf("admin only custom = %v want %v", got2, want2)
	}
}

func TestNormalizeAdminLabels(t *testing.T) {
	got, err := NormalizeAdminLabels([]string{" Foo ", "foo", "bar-1"})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, []string{"foo", "bar-1"}) {
		t.Fatalf("got %v", got)
	}
	if _, err := NormalizeAdminLabels([]string{"bad space"}); err == nil {
		t.Fatal("expected err for space inside tag")
	}
	_, err = NormalizeAdminLabels([]string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k", "l", "m", "n", "o", "p", "q"})
	if err == nil {
		t.Fatal("expected too many")
	}
}
