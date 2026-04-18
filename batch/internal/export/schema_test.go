package export

import (
	"encoding/json"
	"testing"
)

// TestRankingsJSONRoundTrip ensures the schema encodes/decodes losslessly.
func TestRankingsJSONRoundTrip(t *testing.T) {
	in := Rankings{
		UpdatedAt: "2026-04-18T00:00:00Z",
		Rankings: map[string][]RankingEntry{
			"7d": {
				{Rank: 1, RepoID: "g1", Owner: "o", Name: "r", FullName: "o/r",
					StartStars: 100, EndStars: 150, StarDelta: 50, GrowthPct: 50.0},
			},
			"30d": {},
		},
	}
	b, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var out Rankings
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.UpdatedAt != in.UpdatedAt {
		t.Errorf("updated_at mismatch")
	}
	if len(out.Rankings["7d"]) != 1 {
		t.Errorf("7d entries lost")
	}
	if out.Rankings["7d"][0].GrowthPct != 50.0 {
		t.Errorf("growth_pct lost")
	}
}

func TestRankingsJSONFieldNames(t *testing.T) {
	in := Rankings{
		UpdatedAt: "2026-04-18T00:00:00Z",
		Rankings: map[string][]RankingEntry{"7d": {{Rank: 1, RepoID: "g", Owner: "o", Name: "r", FullName: "o/r"}}},
	}
	b, _ := json.Marshal(in)

	// Decode into generic map to inspect keys.
	var raw map[string]any
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatal(err)
	}
	if _, ok := raw["updated_at"]; !ok {
		t.Errorf("expected snake_case updated_at, got keys %v", keys(raw))
	}

	entries := raw["rankings"].(map[string]any)["7d"].([]any)
	first := entries[0].(map[string]any)
	for _, k := range []string{"rank", "repo_id", "owner", "name", "full_name", "growth_pct", "star_delta"} {
		if _, ok := first[k]; !ok {
			t.Errorf("entry missing field %q: %v", k, keys(first))
		}
	}
}

func TestRepoDetailRoundTrip(t *testing.T) {
	in := RepoDetail{
		RepoID:       "g1",
		Owner:        "o",
		Name:         "r",
		FullName:     "o/r",
		Description:  "desc",
		URL:          "https://example",
		HomepageURL:  "https://hp",
		Language:     "Go",
		License:      "MIT",
		Topics:       []string{"ai", "go"},
		StarCount:    1000,
		ForkCount:    50,
		StarHistory: []StarPoint{
			{Date: "2026-04-17", Stars: 900},
			{Date: "2026-04-18", Stars: 1000},
		},
	}
	b, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out RepoDetail
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(out.StarHistory) != 2 || out.StarHistory[1].Stars != 1000 {
		t.Errorf("history lost: %+v", out.StarHistory)
	}
	if len(out.Topics) != 2 {
		t.Errorf("topics lost: %v", out.Topics)
	}
}

func TestMetaJSONRoundTrip(t *testing.T) {
	in := Meta{
		GeneratedAt: "2026-04-18T00:00:00Z",
		TotalRepos:  1234,
		Periods:     []string{"1d", "7d", "30d"},
	}
	b, _ := json.Marshal(in)
	var out Meta
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.TotalRepos != 1234 || len(out.Periods) != 3 {
		t.Errorf("meta lost: %+v", out)
	}
}

func keys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
