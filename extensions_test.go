package pathrules

import "testing"

func TestParseExtensions(t *testing.T) {
	t.Parallel()

	got := ParseExtensions([]string{
		"rvmat",
		".PAA",
		"*.OGG",
		" ..cfg  ",
		"",
		"   ",
	})

	want := []Rule{
		{Action: ActionInclude, Pattern: "*.rvmat"},
		{Action: ActionInclude, Pattern: "*.paa"},
		{Action: ActionInclude, Pattern: "*.ogg"},
		{Action: ActionInclude, Pattern: "*.cfg"},
	}

	if len(got) != len(want) {
		t.Fatalf("len(got)=%d, want %d", len(got), len(want))
	}

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("rule[%d]=%+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestParseExtensions_Empty(t *testing.T) {
	t.Parallel()

	got := ParseExtensions(nil)
	if len(got) != 0 {
		t.Fatalf("len(got)=%d, want 0", len(got))
	}
}
