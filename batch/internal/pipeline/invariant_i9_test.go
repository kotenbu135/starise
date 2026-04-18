package pipeline

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/kotenbu135/starise/batch/internal/export"
)

// I9: JSON Marshal/Unmarshal yields equal structs for all output schemas.
// Covered in detail by export/schema_test.go; this test pins it at the
// pipeline level so future format changes break here too.
func TestInvariantI9_JSONRoundTrip_Real(t *testing.T) {
	cases := []interface{}{
		export.RepoDetail{
			RepoID: "G1", Owner: "x", Name: "a", FullName: "x/a",
			Topics: []string{"a"}, StarHistory: []export.HistoryPoint{{Date: "2026-04-18", Stars: 1}},
		},
		export.Rankings{
			UpdatedAt: "X",
			Rankings: map[string][]export.RankingEntry{
				"1d_breakout": {{Rank: 1, RepoID: "G1", Owner: "x", Name: "a", FullName: "x/a"}},
			},
		},
		export.Meta{GeneratedAt: "X", TotalRepos: 1, TotalActive: 1, Periods: []string{"1d"}, RankTypes: []string{"breakout"}},
	}
	for i, in := range cases {
		b, err := json.Marshal(in)
		if err != nil {
			t.Fatalf("case %d marshal: %v", i, err)
		}
		out := reflect.New(reflect.TypeOf(in)).Interface()
		if err := json.Unmarshal(b, out); err != nil {
			t.Fatalf("case %d unmarshal: %v", i, err)
		}
		got := reflect.ValueOf(out).Elem().Interface()
		if !reflect.DeepEqual(in, got) {
			t.Errorf("case %d roundtrip mismatch:\nwant %+v\n got %+v", i, in, got)
		}
	}
}
