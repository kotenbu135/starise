// Package fetch retrieves seed repositories one-by-one and writes their
// today snapshot to the DB.
package fetch

import (
	"bufio"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/kotenbu135/starise/batch/internal/db"
	"github.com/kotenbu135/starise/batch/internal/github"
)

// Result summarizes one fetch invocation.
type Result struct {
	Fetched int
	Missing int
	Errors  int
}

// LoadSeeds reads "owner/name" lines from path. Lines starting with '#' or
// whitespace-only are skipped. Returned pairs are lowercase.
func LoadSeeds(path string) ([]string, []string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()
	var owners, names []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "/", 2)
		if len(parts) != 2 {
			return nil, nil, fmt.Errorf("invalid seed line %q (want owner/name)", line)
		}
		owners = append(owners, strings.ToLower(parts[0]))
		names = append(names, strings.ToLower(parts[1]))
	}
	return owners, names, sc.Err()
}

// Run fetches each seed via the client, upserts the repo, and writes today's
// star snapshot. ErrNotFound on a seed is silently counted (the repo never
// existed in our DB anyway).
func Run(ctx context.Context, d *sql.DB, c github.Client, owners, names []string, today string) (Result, error) {
	if len(owners) != len(names) {
		return Result{}, fmt.Errorf("owners/names length mismatch (%d vs %d)", len(owners), len(names))
	}
	var res Result
	for i := range owners {
		data, _, err := c.FetchRepo(ctx, owners[i], names[i])
		if err != nil {
			if errors.Is(err, github.ErrNotFound) {
				res.Missing++
				continue
			}
			res.Errors++
			continue
		}
		id, err := db.UpsertRepository(d, repoFromData(data))
		if err != nil {
			res.Errors++
			continue
		}
		if err := db.UpsertDailyStar(d, id, today, data.StarCount); err != nil {
			res.Errors++
			continue
		}
		res.Fetched++
	}
	return res, nil
}

func repoFromData(d github.RepoData) db.Repository {
	return db.Repository{
		GitHubID: d.GitHubID, Owner: d.Owner, Name: d.Name,
		Description: d.Description, URL: d.URL, HomepageURL: d.HomepageURL,
		Language: d.Language, License: d.License, Topics: d.Topics,
		IsArchived: d.IsArchived, IsFork: d.IsFork, ForkCount: d.ForkCount,
		CreatedAt: d.CreatedAt, UpdatedAt: d.UpdatedAt, PushedAt: d.PushedAt,
	}
}
