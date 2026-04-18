package github

import (
	"testing"
	"time"
)

func TestSleepUntilResetWaitsWhenLow(t *testing.T) {
	now, _ := time.Parse(time.RFC3339, "2026-04-18T15:00:00Z")
	info := RateLimitInfo{Remaining: 50, ResetAt: "2026-04-18T15:30:00Z"}
	d := SleepUntilReset(info, 100, now)
	if d <= 0 {
		t.Errorf("expected positive sleep, got %v", d)
	}
}

func TestSleepUntilResetSkipsWhenAboveThreshold(t *testing.T) {
	now, _ := time.Parse(time.RFC3339, "2026-04-18T15:00:00Z")
	info := RateLimitInfo{Remaining: 500, ResetAt: "2026-04-18T15:30:00Z"}
	if d := SleepUntilReset(info, 100, now); d != 0 {
		t.Errorf("expected 0, got %v", d)
	}
}

func TestSleepUntilResetSkipsWhenResetPassed(t *testing.T) {
	now, _ := time.Parse(time.RFC3339, "2026-04-18T16:00:00Z")
	info := RateLimitInfo{Remaining: 50, ResetAt: "2026-04-18T15:30:00Z"}
	if d := SleepUntilReset(info, 100, now); d != 0 {
		t.Errorf("expected 0, got %v", d)
	}
}

func TestIsNotFound(t *testing.T) {
	cases := map[string]bool{
		"Could not resolve to a Repository": true,
		"NOT_FOUND":                         true,
		"unrelated error":                   false,
	}
	for msg, want := range cases {
		got := isNotFound(&stringErr{msg})
		if got != want {
			t.Errorf("%q: got %v, want %v", msg, got, want)
		}
	}
}

type stringErr struct{ s string }

func (e *stringErr) Error() string { return e.s }
