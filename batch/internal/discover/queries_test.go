package discover

import (
	"strings"
	"testing"
	"time"
)

func TestBuildQuerySetCount(t *testing.T) {
	now, _ := time.Parse("2006-01-02", "2026-04-18")
	qs := BuildQuerySet(now)

	// 10 star bands + 15 langs × 3 bands + 2 new-repo + 7 topics = 64
	want := 10 + 15*3 + 2 + 7
	if len(qs) != want {
		t.Errorf("len=%d, want %d", len(qs), want)
	}
}

func TestBuildQuerySetNoDuplicates(t *testing.T) {
	now, _ := time.Parse("2006-01-02", "2026-04-18")
	qs := BuildQuerySet(now)
	seen := map[string]bool{}
	for _, q := range qs {
		if seen[q] {
			t.Errorf("duplicate query: %q", q)
		}
		seen[q] = true
	}
}

func TestBuildQuerySetIncludesBaseFilters(t *testing.T) {
	now, _ := time.Parse("2006-01-02", "2026-04-18")
	qs := BuildQuerySet(now)
	for _, q := range qs {
		if !strings.Contains(q, "fork:false") {
			t.Errorf("missing fork:false in %q", q)
		}
		if !strings.Contains(q, "archived:false") {
			t.Errorf("missing archived:false in %q", q)
		}
	}
}

func TestBuildQuerySetDeterministic(t *testing.T) {
	now, _ := time.Parse("2006-01-02", "2026-04-18")
	a := BuildQuerySet(now)
	b := BuildQuerySet(now)
	if len(a) != len(b) {
		t.Fatalf("len mismatch: %d vs %d", len(a), len(b))
	}
	for i := range a {
		if a[i] != b[i] {
			t.Errorf("index %d: %q != %q", i, a[i], b[i])
		}
	}
}
