package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
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
	Topics      []string
	IsArchived  bool
	IsFork      bool
	ForkCount   int
	CreatedAt   string
	UpdatedAt   string
	PushedAt    string
	DeletedAt   string
}

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}

// UpsertRepository inserts or updates by github_id and returns the row ID.
// The deleted_at column is preserved (not overwritten) to keep soft deletes.
func UpsertRepository(d *sql.DB, r Repository) (int64, error) {
	topicsJSON, err := json.Marshal(r.Topics)
	if err != nil {
		return 0, fmt.Errorf("marshal topics: %w", err)
	}
	if r.Topics == nil {
		topicsJSON = []byte("[]")
	}

	const q = `
INSERT INTO repositories (
    github_id, owner, name, description, url, homepage_url, language, license,
    topics, is_archived, is_fork, fork_count, created_at, updated_at, pushed_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(github_id) DO UPDATE SET
    owner=excluded.owner,
    name=excluded.name,
    description=excluded.description,
    url=excluded.url,
    homepage_url=excluded.homepage_url,
    language=excluded.language,
    license=excluded.license,
    topics=excluded.topics,
    is_archived=excluded.is_archived,
    is_fork=excluded.is_fork,
    fork_count=excluded.fork_count,
    created_at=excluded.created_at,
    updated_at=excluded.updated_at,
    pushed_at=excluded.pushed_at
RETURNING id`
	var id int64
	err = d.QueryRow(q,
		r.GitHubID, r.Owner, r.Name, r.Description, r.URL, r.HomepageURL,
		r.Language, r.License, string(topicsJSON), b2i(r.IsArchived), b2i(r.IsFork),
		r.ForkCount, r.CreatedAt, r.UpdatedAt, r.PushedAt,
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("upsert repository: %w", err)
	}
	return id, nil
}

func SoftDeleteByGitHubID(d *sql.DB, githubID, date string) error {
	_, err := d.Exec("UPDATE repositories SET deleted_at=? WHERE github_id=?", date, githubID)
	return err
}

func HardDeleteByGitHubID(d *sql.DB, githubID string) error {
	_, err := d.Exec("DELETE FROM repositories WHERE github_id=?", githubID)
	return err
}

func GetRepositoryByGitHubID(d *sql.DB, githubID string) (Repository, error) {
	const q = `SELECT id, github_id, owner, name, description, url, homepage_url, language, license,
        topics, is_archived, is_fork, fork_count, created_at, updated_at, pushed_at, deleted_at
        FROM repositories WHERE github_id=?`
	return scanRepository(d.QueryRow(q, githubID))
}

func GetRepositoryByOwnerName(d *sql.DB, owner, name string) (Repository, error) {
	const q = `SELECT id, github_id, owner, name, description, url, homepage_url, language, license,
        topics, is_archived, is_fork, fork_count, created_at, updated_at, pushed_at, deleted_at
        FROM repositories WHERE owner=? AND name=?`
	return scanRepository(d.QueryRow(q, owner, name))
}

func ListActiveRepositories(d *sql.DB) ([]Repository, error) {
	return queryRepositories(d, `SELECT id, github_id, owner, name, description, url, homepage_url, language, license,
        topics, is_archived, is_fork, fork_count, created_at, updated_at, pushed_at, deleted_at
        FROM repositories WHERE deleted_at='' AND is_archived=0 ORDER BY id`)
}

func ListNonDeletedRepositories(d *sql.DB) ([]Repository, error) {
	return queryRepositories(d, `SELECT id, github_id, owner, name, description, url, homepage_url, language, license,
        topics, is_archived, is_fork, fork_count, created_at, updated_at, pushed_at, deleted_at
        FROM repositories WHERE deleted_at='' ORDER BY id`)
}

func ListAllRepositories(d *sql.DB) ([]Repository, error) {
	return queryRepositories(d, `SELECT id, github_id, owner, name, description, url, homepage_url, language, license,
        topics, is_archived, is_fork, fork_count, created_at, updated_at, pushed_at, deleted_at
        FROM repositories ORDER BY id`)
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanRepository(row rowScanner) (Repository, error) {
	var r Repository
	var topicsJSON string
	var arch, fork int
	if err := row.Scan(&r.ID, &r.GitHubID, &r.Owner, &r.Name, &r.Description,
		&r.URL, &r.HomepageURL, &r.Language, &r.License,
		&topicsJSON, &arch, &fork, &r.ForkCount,
		&r.CreatedAt, &r.UpdatedAt, &r.PushedAt, &r.DeletedAt); err != nil {
		return Repository{}, err
	}
	r.IsArchived = arch != 0
	r.IsFork = fork != 0
	if topicsJSON == "" {
		topicsJSON = "[]"
	}
	if err := json.Unmarshal([]byte(topicsJSON), &r.Topics); err != nil {
		return Repository{}, fmt.Errorf("unmarshal topics: %w", err)
	}
	return r, nil
}

func queryRepositories(d *sql.DB, q string, args ...any) ([]Repository, error) {
	rows, err := d.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Repository
	for rows.Next() {
		r, err := scanRepository(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
