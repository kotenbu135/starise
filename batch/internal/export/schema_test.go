package export

import (
	"encoding/json"
	"reflect"
	"testing"
)

// I9: Marshal -> Unmarshal yields equal structs for each output schema.
func TestRepoDetailRoundTrip(t *testing.T) {
	d := RepoDetail{
		RepoID: "G1", Owner: "acme", Name: "widget", FullName: "acme/widget",
		Description: "x", URL: "https://github.com/acme/widget",
		Language: "Go", License: "MIT", Topics: []string{"a", "b"},
		StarCount: 100, ForkCount: 10,
		IsArchived: false, IsFork: false,
		CreatedAt: "2024-01-01T00:00:00Z",
		UpdatedAt: "2026-04-18T12:00:00Z",
		PushedAt:  "2026-04-18T12:00:00Z",
		StarHistory: []HistoryPoint{{Date: "2026-04-17", Stars: 90}},
	}
	b, err := json.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	var got RepoDetail
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(d, got) {
		t.Errorf("mismatch:\nwant %+v\n got %+v", d, got)
	}
}

func TestRankingsRoundTrip(t *testing.T) {
	r := Rankings{
		UpdatedAt: "2026-04-18T15:00:00Z",
		Rankings: map[string][]RankingEntry{
			"1d_breakout": {{Rank: 1, RepoID: "G1", Owner: "x", Name: "a", FullName: "x/a", StarDelta: 100, GrowthPct: 1900}},
		},
	}
	b, _ := json.Marshal(r)
	var got Rankings
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(r, got) {
		t.Errorf("mismatch")
	}
}

func TestMetaRoundTrip(t *testing.T) {
	m := Meta{GeneratedAt: "x", TotalRepos: 10, TotalActive: 5, Periods: []string{"1d"}, RankTypes: []string{"breakout"}}
	b, _ := json.Marshal(m)
	var got Meta
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(m, got) {
		t.Errorf("mismatch")
	}
}

func TestAllRankingKeys(t *testing.T) {
	keys := AllRankingKeys()
	if len(keys) != 6 {
		t.Errorf("got %d, want 6", len(keys))
	}
}
