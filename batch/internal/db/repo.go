package db

import (
	"database/sql"
	"fmt"
	"time"
)

type Repository struct {
	ID          int64
	GitHubID    string
	Owner       string
	Name        string
	Description string
	URL         string
	HomepageURL string
	Language    string
	License     string
	Topics      string // JSON array
	IsArchived  bool
	IsFork      bool
	ForkCount   int
	CreatedAt   string
	UpdatedAt   string
	PushedAt    string
	FetchedAt   string
}

func UpsertRepository(db *sql.DB, r *Repository) (int64, error) {
	// RETURNING is required: LastInsertId() is unreliable after ON CONFLICT DO UPDATE
	// in modernc.org/sqlite — it may return a stale rowid from an earlier INSERT in
	// the same connection, causing daily_stars to be written against the wrong repo
	// (FK violation or silent data corruption).
	var id int64
	err := db.QueryRow(`
		INSERT INTO repositories (github_id, owner, name, description, url, homepage_url,
			language, license, topics, is_archived, is_fork, fork_count,
			created_at, updated_at, pushed_at, fetched_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(github_id) DO UPDATE SET
			owner=excluded.owner, name=excluded.name, description=excluded.description,
			url=excluded.url, homepage_url=excluded.homepage_url,
			language=excluded.language, license=excluded.license, topics=excluded.topics,
			is_archived=excluded.is_archived, is_fork=excluded.is_fork, fork_count=excluded.fork_count,
			created_at=excluded.created_at, updated_at=excluded.updated_at, pushed_at=excluded.pushed_at,
			fetched_at=excluded.fetched_at
		RETURNING id`,
		r.GitHubID, r.Owner, r.Name, r.Description, r.URL, r.HomepageURL,
		r.Language, r.License, r.Topics, r.IsArchived, r.IsFork, r.ForkCount,
		r.CreatedAt, r.UpdatedAt, r.PushedAt, time.Now().UTC().Format(time.RFC3339),
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("upsert repo: %w", err)
	}
	return id, nil
}

func GetReposNotFetchedToday(db *sql.DB, today string) ([]string, error) {
	rows, err := db.Query(`
		SELECT r.owner, r.name FROM repositories r
		WHERE NOT EXISTS (
			SELECT 1 FROM daily_stars ds
			WHERE ds.repo_id = r.id AND ds.recorded_date = ?
		)`, today)
	if err != nil {
		return nil, fmt.Errorf("query unfetched repos: %w", err)
	}
	defer rows.Close()

	var slugs []string
	for rows.Next() {
		var owner, name string
		if err := rows.Scan(&owner, &name); err != nil {
			return nil, fmt.Errorf("scan slug: %w", err)
		}
		slugs = append(slugs, owner+"/"+name)
	}
	return slugs, rows.Err()
}

func GetAllRepositories(db *sql.DB) ([]Repository, error) {
	rows, err := db.Query(`SELECT id, github_id, owner, name, description, url, homepage_url,
		language, license, topics, is_archived, is_fork, fork_count,
		created_at, updated_at, pushed_at, fetched_at FROM repositories`)
	if err != nil {
		return nil, fmt.Errorf("list repos: %w", err)
	}
	defer rows.Close()

	var repos []Repository
	for rows.Next() {
		var r Repository
		if err := rows.Scan(&r.ID, &r.GitHubID, &r.Owner, &r.Name, &r.Description,
			&r.URL, &r.HomepageURL, &r.Language, &r.License, &r.Topics,
			&r.IsArchived, &r.IsFork, &r.ForkCount,
			&r.CreatedAt, &r.UpdatedAt, &r.PushedAt, &r.FetchedAt); err != nil {
			return nil, fmt.Errorf("scan repo: %w", err)
		}
		repos = append(repos, r)
	}
	return repos, rows.Err()
}
