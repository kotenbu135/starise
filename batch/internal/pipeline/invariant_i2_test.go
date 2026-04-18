package pipeline

import (
	"sort"
	"testing"

	"github.com/kotenbu135/starise/batch/internal/db"
	"github.com/kotenbu135/starise/batch/internal/export"
	"github.com/kotenbu135/starise/batch/internal/restore"
)

// I2: DB → export → fresh DB ← restore must round-trip every column listed
// in the issue (id auto-increment ignored).
func TestInvariantI2_ExportRestoreRoundTrip_Real(t *testing.T) {
	src := openMem(t)
	id1, _ := db.UpsertRepository(src, db.Repository{
		GitHubID: "G1", Owner: "x", Name: "a",
		Description: "desc", URL: "https://github.com/x/a", HomepageURL: "https://x.example",
		Language: "Go", License: "MIT", Topics: []string{"a", "b"},
		IsArchived: false, IsFork: false, ForkCount: 5,
		CreatedAt: "2024-01-01T00:00:00Z",
		UpdatedAt: "2026-04-18T00:00:00Z",
		PushedAt:  "2026-04-18T00:00:00Z",
	})
	db.UpsertDailyStar(src, id1, "2026-04-17", 50)
	db.UpsertDailyStar(src, id1, "2026-04-18", 100)
	id2, _ := db.UpsertRepository(src, db.Repository{
		GitHubID: "G2", Owner: "x", Name: "b", Language: "Rust", IsArchived: true,
	})
	db.UpsertDailyStar(src, id2, "2026-04-18", 500)
	id3, _ := db.UpsertRepository(src, db.Repository{GitHubID: "G3", Owner: "x", Name: "del"})
	db.SoftDeleteByGitHubID(src, "G3", "2026-04-18")
	db.UpsertDailyStar(src, id3, "2026-04-15", 10)

	dir := t.TempDir()
	if _, err := export.Export(src, export.Options{
		OutDir: dir, UpdatedAt: "X", GeneratedAt: "X", ComputedDate: "2026-04-18", TopN: 100,
	}); err != nil {
		t.Fatal(err)
	}

	dst := openMem(t)
	if _, err := restore.FromDir(dst, dir); err != nil {
		t.Fatal(err)
	}

	allSrc, _ := db.ListAllRepositories(src)
	allDst, _ := db.ListAllRepositories(dst)
	if len(allSrc) != len(allDst) {
		t.Fatalf("repo count: src=%d dst=%d", len(allSrc), len(allDst))
	}
	bySrc := indexByGitHubID(allSrc)
	for _, dr := range allDst {
		sr, ok := bySrc[dr.GitHubID]
		if !ok {
			t.Errorf("dst has unknown github_id %s", dr.GitHubID)
			continue
		}
		if !equalRepoModuloID(sr, dr) {
			t.Errorf("repo %s differs:\nsrc=%+v\ndst=%+v", dr.GitHubID, sr, dr)
		}
		// star history equality
		srcHist, _ := db.ListStarHistory(src, sr.ID)
		dstHist, _ := db.ListStarHistory(dst, dr.ID)
		if len(srcHist) != len(dstHist) {
			t.Errorf("repo %s history len: src=%d dst=%d", dr.GitHubID, len(srcHist), len(dstHist))
			continue
		}
		for i := range srcHist {
			if srcHist[i].RecordedDate != dstHist[i].RecordedDate ||
				srcHist[i].StarCount != dstHist[i].StarCount {
				t.Errorf("repo %s history idx %d differs", dr.GitHubID, i)
			}
		}
	}
}

func indexByGitHubID(rs []db.Repository) map[string]db.Repository {
	out := make(map[string]db.Repository, len(rs))
	for _, r := range rs {
		out[r.GitHubID] = r
	}
	return out
}

func equalRepoModuloID(a, b db.Repository) bool {
	at := append([]string(nil), a.Topics...)
	bt := append([]string(nil), b.Topics...)
	sort.Strings(at)
	sort.Strings(bt)
	if len(at) != len(bt) {
		return false
	}
	for i := range at {
		if at[i] != bt[i] {
			return false
		}
	}
	return a.GitHubID == b.GitHubID &&
		a.Owner == b.Owner && a.Name == b.Name &&
		a.Description == b.Description &&
		a.URL == b.URL && a.HomepageURL == b.HomepageURL &&
		a.Language == b.Language && a.License == b.License &&
		a.IsArchived == b.IsArchived && a.IsFork == b.IsFork &&
		a.ForkCount == b.ForkCount &&
		a.CreatedAt == b.CreatedAt && a.UpdatedAt == b.UpdatedAt &&
		a.PushedAt == b.PushedAt && a.DeletedAt == b.DeletedAt
}
