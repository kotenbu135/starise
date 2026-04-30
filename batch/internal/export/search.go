package export

import (
	"database/sql"
	"fmt"
	"sort"

	"github.com/kotenbu135/starise/batch/internal/db"
	"github.com/kotenbu135/starise/batch/internal/translate"
)

// SearchIndexEntry is one row of the slim header-search index. Keys are
// shortened to keep the gzipped payload small (~1.5MB for 60k repos).
//
// D is description_ja with fallback to description, truncated to
// SearchDescriptionMaxRunes runes (multi-byte safe).
type SearchIndexEntry struct {
	O string `json:"o"`           // owner
	N string `json:"n"`           // name
	L string `json:"l,omitempty"` // language
	S int    `json:"s"`           // star_count, used to break ties in client-side ranking
	D string `json:"d,omitempty"` // description_ja || description, truncated
}

// SearchIndex is the data/search-index.json document. GeneratedAt is the
// only field allowed to vary between runs (matches I13 convention).
type SearchIndex struct {
	GeneratedAt string             `json:"generated_at"`
	Repos       []SearchIndexEntry `json:"repos"`
}

// SearchDescriptionMaxRunes caps each entry's description length at 80
// runes. Chosen to bound the index size while keeping enough text for
// substring matching ("react native UI library" ~= 23 chars).
const SearchDescriptionMaxRunes = 80

// BuildSearchIndex returns a deterministic search index built from the
// active repositories table. Entries are sorted by (owner, name).
//
// trCache may be nil; in that case D falls back unconditionally to
// repo.Description.
func BuildSearchIndex(d *sql.DB, generatedAt, computedDate string, trCache *translate.Cache) (SearchIndex, error) {
	repos, err := db.ListActiveRepositories(d)
	if err != nil {
		return SearchIndex{}, fmt.Errorf("list active: %w", err)
	}

	entries := make([]SearchIndexEntry, 0, len(repos))
	for _, r := range repos {
		stars, _, err := db.StarCountAtOrBefore(d, r.ID, computedDate)
		if err != nil {
			return SearchIndex{}, fmt.Errorf("star snapshot %s: %w", r.GitHubID, err)
		}
		desc := r.Description
		if trCache != nil && r.Description != "" {
			if e, ok, err := trCache.Get(r.Description); err == nil && ok && e.JA != "" {
				desc = e.JA
			}
		}
		entries = append(entries, SearchIndexEntry{
			O: r.Owner,
			N: r.Name,
			L: r.Language,
			S: stars,
			D: truncateRunes(desc, SearchDescriptionMaxRunes),
		})
	}

	// db.ListActiveRepositories already orders by id; resort by (owner, name)
	// for the deterministic, alphabetised consumer-facing view.
	sortByOwnerName(entries)

	return SearchIndex{
		GeneratedAt: generatedAt,
		Repos:       entries,
	}, nil
}

func truncateRunes(s string, max int) string {
	if max <= 0 {
		return ""
	}
	rs := []rune(s)
	if len(rs) <= max {
		return s
	}
	return string(rs[:max])
}

func sortByOwnerName(es []SearchIndexEntry) {
	sort.SliceStable(es, func(i, j int) bool {
		if es[i].O != es[j].O {
			return es[i].O < es[j].O
		}
		return es[i].N < es[j].N
	})
}
