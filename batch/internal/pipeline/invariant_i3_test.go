package pipeline

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/kotenbu135/starise/batch/internal/export"
	"github.com/kotenbu135/starise/batch/internal/github"
)

// I3: 3-day continuous simulation must preserve Day-1 history through Day-3.
// Each day re-opens a fresh DB and restores from dataDir, mirroring how the
// production CI run starts each morning.
func TestInvariantI3_ThreeDaySimulation_Real(t *testing.T) {
	dataDir := t.TempDir()
	mock := github.NewMockClient()
	mock.Add(github.RepoData{GitHubID: "G1", Owner: "x", Name: "a", StarCount: 50})

	days := []struct {
		date  string
		stars int
	}{
		{"2026-04-16", 50},
		{"2026-04-17", 75},
		{"2026-04-18", 100},
	}

	for _, day := range days {
		// Update mock to return the day's star count.
		mock.Repos["x/a"] = github.RepoData{
			GitHubID: "G1", Owner: "x", Name: "a", StarCount: day.stars,
		}

		_, err := RunSimulationDay(context.Background(), dataDir, Options{
			Client: mock, Today: day.date,
			SeedOwners: []string{"x"}, SeedNames: []string{"a"},
			TopN: 100, SkipDiscover: true,
			UpdatedAt: "X", GeneratedAt: "X",
			AllowEmptyRankings: true,
		})
		if err != nil {
			// Day 1 can fail "all slots empty" because there's no history yet —
			// that's expected and we only assert history preservation.
			t.Logf("day %s pipeline returned: %v", day.date, err)
		}
	}

	// Day 1's snapshot must still be present in the final dataDir.
	b, err := os.ReadFile(filepath.Join(dataDir, "repos", "x__a.json"))
	if err != nil {
		t.Fatal(err)
	}
	var detail export.RepoDetail
	if err := json.Unmarshal(b, &detail); err != nil {
		t.Fatal(err)
	}
	dates := map[string]int{}
	for _, p := range detail.StarHistory {
		dates[p.Date] = p.Stars
	}
	for _, day := range days {
		if got, ok := dates[day.date]; !ok {
			t.Errorf("day %s missing from final history (history=%v)", day.date, dates)
		} else if got != day.stars {
			t.Errorf("day %s stars=%d, want %d", day.date, got, day.stars)
		}
	}
}
