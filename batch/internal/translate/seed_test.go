package translate

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func TestReadDescriptionsFromRepoDir_Basic(t *testing.T) {
	dir := t.TempDir()
	mustWriteRepoJSON(t, dir, "x__a.json", map[string]any{
		"owner": "x", "name": "a", "description": "thing one",
	})
	mustWriteRepoJSON(t, dir, "x__b.json", map[string]any{
		"owner": "x", "name": "b", "description": "thing two",
	})

	got, err := ReadDescriptionsFromRepoDir(dir)
	if err != nil {
		t.Fatalf("ReadDescriptionsFromRepoDir: %v", err)
	}
	sort.Strings(got)
	want := []string{"thing one", "thing two"}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d", len(got), len(want))
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestReadDescriptionsFromRepoDir_DedupsAndSkipsBlanks(t *testing.T) {
	dir := t.TempDir()
	mustWriteRepoJSON(t, dir, "x__a.json", map[string]any{"description": "same"})
	mustWriteRepoJSON(t, dir, "x__b.json", map[string]any{"description": "same"})
	mustWriteRepoJSON(t, dir, "x__c.json", map[string]any{"description": ""})
	mustWriteRepoJSON(t, dir, "x__d.json", map[string]any{"description": "  "})

	got, err := ReadDescriptionsFromRepoDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != "same" {
		t.Fatalf("got %v, want [\"same\"]", got)
	}
}

func TestReadDescriptionsFromRepoDir_IgnoresNonJSON(t *testing.T) {
	dir := t.TempDir()
	mustWriteRepoJSON(t, dir, "ok.json", map[string]any{"description": "hello"})
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# nope"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := ReadDescriptionsFromRepoDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("got %v, want only the .json file's description", got)
	}
}

func TestReadDescriptionsFromRepoDir_DirNotFound(t *testing.T) {
	_, err := ReadDescriptionsFromRepoDir("/no/such/path/whatsoever")
	if err == nil {
		t.Fatal("expected error for missing dir")
	}
}

func mustWriteRepoJSON(t *testing.T, dir, name string, payload map[string]any) {
	t.Helper()
	b, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), b, 0o644); err != nil {
		t.Fatal(err)
	}
}
