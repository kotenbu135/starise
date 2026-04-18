package fetch

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/kotenbu135/starise/batch/internal/db"
	"github.com/kotenbu135/starise/batch/internal/github"
)

func TestLoadSeedsSkipsCommentsAndBlanks(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "seeds.txt")
	os.WriteFile(path, []byte("# header\n\nACME/Widget\n  \nfoo/bar\n"), 0o644)
	owners, names, err := LoadSeeds(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(owners) != 2 || owners[0] != "acme" || names[0] != "widget" {
		t.Errorf("got owners=%v names=%v", owners, names)
	}
}

func TestLoadSeedsRejectsBadLine(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.txt")
	os.WriteFile(path, []byte("noslash\n"), 0o644)
	if _, _, err := LoadSeeds(path); err == nil {
		t.Errorf("expected error")
	}
}

func TestRunFetchesAndStoresSnapshot(t *testing.T) {
	d, err := db.Open("")
	if err != nil {
		t.Fatalf("db open: %v", err)
	}
	defer d.Close()
	c := github.NewMockClient()
	c.Add(github.RepoData{GitHubID: "G1", Owner: "x", Name: "a", StarCount: 100})
	c.Add(github.RepoData{GitHubID: "G2", Owner: "x", Name: "b", StarCount: 200})

	res, err := Run(context.Background(), d, c, []string{"x", "x"}, []string{"a", "b"}, "2026-04-18")
	if err != nil {
		t.Fatal(err)
	}
	if res.Fetched != 2 {
		t.Errorf("fetched=%d, want 2", res.Fetched)
	}
	all, _ := db.ListActiveRepositories(d)
	if len(all) != 2 {
		t.Errorf("repos=%d", len(all))
	}
}

func TestRunCountsMissing(t *testing.T) {
	d, err := db.Open("")
	if err != nil {
		t.Fatalf("db open: %v", err)
	}
	defer d.Close()
	c := github.NewMockClient() // empty
	res, _ := Run(context.Background(), d, c, []string{"ghost"}, []string{"town"}, "2026-04-18")
	if res.Missing != 1 || res.Fetched != 0 {
		t.Errorf("got %+v", res)
	}
}
