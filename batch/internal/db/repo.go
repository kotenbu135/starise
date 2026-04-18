package db

import (
	"database/sql"
	"errors"
	"fmt"
)

// ErrNotFound is returned when a lookup by key yields no row.
var ErrNotFound = errors.New("not found")

// Repository models a row in the repositories table.
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
	Topics      string // JSON-encoded array
	IsArchived  bool
	IsFork      bool
	ForkCount   int
	CreatedAt   string
	UpdatedAt   string
	PushedAt    string
}

// UpsertRepository inserts or updates a repository by (owner, name) and
// returns its primary key. github_id is kept in sync on update.
func UpsertRepository(d *sql.DB, r *Repository) (int64, error) {
	const q = `
INSERT INTO repositories (
	github_id, owner, name, description, url, homepage_url,
	language, license, topics, is_archived, is_fork, fork_count,
	created_at, updated_at, pushed_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT (owner, name) DO UPDATE SET
	github_id    = excluded.github_id,
	description  = excluded.description,
	url          = excluded.url,
	homepage_url = excluded.homepage_url,
	language     = excluded.language,
	license      = excluded.license,
	topics       = excluded.topics,
	is_archived  = excluded.is_archived,
	is_fork      = excluded.is_fork,
	fork_count   = excluded.fork_count,
	created_at   = excluded.created_at,
	updated_at   = excluded.updated_at,
	pushed_at    = excluded.pushed_at
RETURNING id;
`
	topics := r.Topics
	if topics == "" {
		topics = "[]"
	}
	var id int64
	err := d.QueryRow(q,
		r.GitHubID, r.Owner, r.Name, r.Description, r.URL, r.HomepageURL,
		r.Language, r.License, topics, boolToInt(r.IsArchived), boolToInt(r.IsFork), r.ForkCount,
		r.CreatedAt, r.UpdatedAt, r.PushedAt,
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("upsert repository %s/%s: %w", r.Owner, r.Name, err)
	}
	return id, nil
}

// GetRepositoryByOwnerName returns a repository by (owner, name). ErrNotFound
// when no matching row exists.
func GetRepositoryByOwnerName(d *sql.DB, owner, name string) (*Repository, error) {
	const q = `
SELECT id, github_id, owner, name, description, url, homepage_url,
       language, license, topics, is_archived, is_fork, fork_count,
       created_at, updated_at, pushed_at
FROM repositories WHERE owner = ? AND name = ?;
`
	var r Repository
	var archived, fork int
	err := d.QueryRow(q, owner, name).Scan(
		&r.ID, &r.GitHubID, &r.Owner, &r.Name, &r.Description, &r.URL, &r.HomepageURL,
		&r.Language, &r.License, &r.Topics, &archived, &fork, &r.ForkCount,
		&r.CreatedAt, &r.UpdatedAt, &r.PushedAt,
	)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get repository %s/%s: %w", owner, name, err)
	}
	r.IsArchived = archived != 0
	r.IsFork = fork != 0
	return &r, nil
}

// ListRepositories returns all repositories ordered by id.
func ListRepositories(d *sql.DB) ([]Repository, error) {
	const q = `
SELECT id, github_id, owner, name, description, url, homepage_url,
       language, license, topics, is_archived, is_fork, fork_count,
       created_at, updated_at, pushed_at
FROM repositories ORDER BY id;
`
	rows, err := d.Query(q)
	if err != nil {
		return nil, fmt.Errorf("list repositories: %w", err)
	}
	defer rows.Close()

	var out []Repository
	for rows.Next() {
		var r Repository
		var archived, fork int
		if err := rows.Scan(
			&r.ID, &r.GitHubID, &r.Owner, &r.Name, &r.Description, &r.URL, &r.HomepageURL,
			&r.Language, &r.License, &r.Topics, &archived, &fork, &r.ForkCount,
			&r.CreatedAt, &r.UpdatedAt, &r.PushedAt,
		); err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}
		r.IsArchived = archived != 0
		r.IsFork = fork != 0
		out = append(out, r)
	}
	return out, rows.Err()
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
