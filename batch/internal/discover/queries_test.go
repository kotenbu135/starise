package discover

import (
	"strings"
	"testing"
	"time"
)

func TestBuildQuerySetCount(t *testing.T) {
	now, _ := time.Parse("2006-01-02", "2026-04-18")
	qs := BuildQuerySet(now)

	// 10 star bands + 15 langs × 3 bands + 2 new-repo + 7 topics = 64 (v1)
	// + 4 low-star tiers (breakout candidates)
	// + 11 new langs × 3 bands = 33
	// + 15 topics (blockchain/web3/gamedev/devops/security/nlp/cv/etc.)
	// + 10 framework topics (react/vue/django/rails/etc.)
	// + 5 recent-activity splits
	// = 131 total
	want := 10 + 15*3 + 2 + 7 + 4 + 11*3 + 15 + 10 + 5
	if len(qs) != want {
		t.Errorf("len=%d, want %d", len(qs), want)
	}
}

func TestBuildQuerySetIncludesLowStarBreakoutTiers(t *testing.T) {
	// The ranking model needs a snapshot with 1 <= start < 100 to surface
	// breakout candidates. The original preset started at stars:>=100, so
	// no breakout bucket ever populated. These low-star tiers seed the
	// breakout axis so small-but-rising repos can eventually rank.
	now, _ := time.Parse("2006-01-02", "2026-04-18")
	qs := BuildQuerySet(now)
	joined := strings.Join(qs, "\n")
	for _, frag := range []string{
		"stars:50..99", "stars:30..49", "stars:10..29", "stars:5..9",
	} {
		if !strings.Contains(joined, frag) {
			t.Errorf("missing low-star tier %q", frag)
		}
	}
}

func TestBuildQuerySetIncludesExtendedLanguages(t *testing.T) {
	// Coverage expansion: the 2026-04-21 run discovered ~36k repos using
	// 15 languages. Adding niche/declining-but-active languages broadens
	// the discovery net at negligible budget cost (actual ~1 pt/query).
	now, _ := time.Parse("2006-01-02", "2026-04-18")
	qs := BuildQuerySet(now)
	joined := strings.Join(qs, "\n")
	for _, lang := range []string{
		"Lua", "Haskell", "Zig", "OCaml", "Elm",
		"Nim", "Crystal", "Clojure", "R", "Shell",
	} {
		if !strings.Contains(joined, "language:"+lang) {
			t.Errorf("missing language %q", lang)
		}
	}
	// F# name contains "#" — URL-encoded in the query as-is.
	if !strings.Contains(joined, "language:F#") {
		t.Errorf("missing language F#")
	}
}

func TestBuildQuerySetIncludesExtendedTopics(t *testing.T) {
	now, _ := time.Parse("2006-01-02", "2026-04-18")
	qs := BuildQuerySet(now)
	joined := strings.Join(qs, "\n")
	// Original 7 topics cover AI/ML. These 15 topics widen coverage into
	// the other major OSS domains.
	for _, topic := range []string{
		"blockchain", "web3", "gamedev", "devops", "security",
		"nlp", "computer-vision", "robotics", "iot", "database",
		"deep-learning", "data-science", "backend", "frontend", "cli",
	} {
		if !strings.Contains(joined, "topic:"+topic) {
			t.Errorf("missing topic %q", topic)
		}
	}
}

func TestBuildQuerySetIncludesFrameworkTopics(t *testing.T) {
	now, _ := time.Parse("2006-01-02", "2026-04-18")
	qs := BuildQuerySet(now)
	joined := strings.Join(qs, "\n")
	for _, topic := range []string{
		"react", "vue", "svelte", "nextjs", "fastapi",
		"django", "rails", "laravel", "flutter", "tensorflow",
	} {
		if !strings.Contains(joined, "topic:"+topic) {
			t.Errorf("missing framework topic %q", topic)
		}
	}
}

func TestBuildQuerySetIncludesRecentActivitySplits(t *testing.T) {
	// Recent-activity filters surface fresh trending repos that may be
	// buried under older heavyweights in the broad star-band queries.
	now, _ := time.Parse("2006-01-02", "2026-04-18")
	qs := BuildQuerySet(now)
	// last-week and last-month are derived from `now` — check the actual
	// dates rather than literal strings.
	lastWeek := now.AddDate(0, 0, -7).Format("2006-01-02")
	lastMonth := now.AddDate(0, 0, -30).Format("2006-01-02")

	joined := strings.Join(qs, "\n")
	if !strings.Contains(joined, "pushed:>"+lastWeek) {
		t.Errorf("missing pushed:>%s (last-week) split", lastWeek)
	}
	if !strings.Contains(joined, "pushed:>"+lastMonth) {
		t.Errorf("missing pushed:>%s (last-month) split", lastMonth)
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
