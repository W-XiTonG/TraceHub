package announcementaudience

import (
	"testing"

	"tracehub/internal/ent"
)

func TestIsGlobal(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		a    *ent.Announcement
		want bool
	}{
		{"nil", nil, true},
		{"empty", &ent.Announcement{}, true},
		{"single legacy", &ent.Announcement{AudienceUserID: ptrInt(3)}, false},
		{"multi", &ent.Announcement{AudienceUserIds: []int{1, 2}}, false},
		{"empty slice", &ent.Announcement{AudienceUserIds: []int{}}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsGlobal(tc.a); got != tc.want {
				t.Fatalf("IsGlobal() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestNormalizeIDs(t *testing.T) {
	t.Parallel()
	got := NormalizeIDs([]int{3, 1, 3, 0, -1, 2})
	want := []int{1, 2, 3}
	if len(got) != len(want) {
		t.Fatalf("NormalizeIDs len = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("NormalizeIDs[%d] = %d, want %d", i, got[i], want[i])
		}
	}
	if NormalizeIDs(nil) != nil {
		t.Fatal("NormalizeIDs(nil) should be nil")
	}
}

func ptrInt(v int) *int { return &v }
