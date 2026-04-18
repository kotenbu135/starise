// Package github abstracts the GitHub GraphQL API behind an interface so that
// the batch commands remain fully testable without hitting the real service.
//
// Tests MUST use the Mock implementation in this package. Hitting the real
// GitHub API from a test is forbidden by project TDD policy.
package github

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/shurcooL/githubv4"
	"golang.org/x/oauth2"
)

// ErrNotFound is returned when a repo is not found (mock parity with real API 404s).
var ErrNotFound = errors.New("github: not found")

// Owner is the minimal identity of a repo owner.
type Owner struct {
	Login string
}

// LicenseInfo is the license subset we consume.
type LicenseInfo struct {
	Name string
}

// Language is one entry in the repository's language list.
type Language struct {
	Name string
}

// Topic is one topic node.
type Topic struct {
	Name string
}

// RepoData is the normalized repository payload returned by the client.
type RepoData struct {
	ID             string
	Owner          Owner
	Name           string
	Description    *string
	URL            string
	HomepageURL    *string
	StargazerCount int
	ForkCount      int
	IsArchived     bool
	IsFork         bool
	PrimaryLang    *Language
	LicenseInfo    *LicenseInfo
	Topics         []Topic
	CreatedAt      string
	UpdatedAt      string
	PushedAt       string
}

// RateLimitInfo mirrors GraphQL rate limit payload.
type RateLimitInfo struct {
	Limit     int
	Remaining int
	Cost      int
	ResetAt   time.Time
}

// FetchRepoResult is a single-repo GraphQL result + rate limit state.
type FetchRepoResult struct {
	Repo      RepoData
	RateLimit RateLimitInfo
}

// SearchResult is a page of the search API.
type SearchResult struct {
	Total     int
	Repos     []RepoData
	HasNext   bool
	EndCursor string
	RateLimit RateLimitInfo
}

// Client is the narrow interface consumed by cmd/ packages. Production code
// uses *APIClient; tests use *Mock.
type Client interface {
	FetchRepo(owner, name string) (*FetchRepoResult, error)
	SearchRepos(query string, first int, after string) (*SearchResult, error)
}

// APIClient is the real GraphQL implementation.
type APIClient struct {
	gql *githubv4.Client
}

// NewAPIClient returns a Client backed by the public GitHub GraphQL endpoint.
func NewAPIClient(token string) *APIClient {
	src := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	httpClient := oauth2.NewClient(context.Background(), src)
	httpClient.Timeout = 30 * time.Second
	return &APIClient{gql: githubv4.NewClient(httpClient)}
}

// NewAPIClientWithHTTP allows tests and callers to inject a custom HTTP client
// (e.g. for retry wrappers). Not used by the default code path.
func NewAPIClientWithHTTP(h *http.Client) *APIClient {
	return &APIClient{gql: githubv4.NewClient(h)}
}

type repoQuery struct {
	Repository struct {
		ID             githubv4.String
		Name           githubv4.String
		Description    *githubv4.String
		URL            githubv4.String
		HomepageURL    *githubv4.String
		StargazerCount githubv4.Int
		ForkCount      githubv4.Int
		IsArchived     githubv4.Boolean
		IsFork         githubv4.Boolean
		CreatedAt      githubv4.DateTime
		UpdatedAt      githubv4.DateTime
		PushedAt       githubv4.DateTime
		Owner          struct {
			Login githubv4.String
		}
		PrimaryLanguage *struct {
			Name githubv4.String
		}
		LicenseInfo *struct {
			Name githubv4.String
		}
		RepositoryTopics struct {
			Nodes []struct {
				Topic struct {
					Name githubv4.String
				}
			}
		} `graphql:"repositoryTopics(first: 20)"`
	} `graphql:"repository(owner: $owner, name: $name)"`
	RateLimit rateLimitFragment
}

type rateLimitFragment struct {
	Limit     githubv4.Int
	Remaining githubv4.Int
	Cost      githubv4.Int
	ResetAt   githubv4.DateTime
}

// FetchRepo retrieves a single repository by owner/name.
func (c *APIClient) FetchRepo(owner, name string) (*FetchRepoResult, error) {
	var q repoQuery
	vars := map[string]any{
		"owner": githubv4.String(owner),
		"name":  githubv4.String(name),
	}
	if err := c.gql.Query(context.Background(), &q, vars); err != nil {
		return nil, fmt.Errorf("fetch %s/%s: %w", owner, name, err)
	}
	return &FetchRepoResult{
		Repo:      repoFromQuery(q),
		RateLimit: rateLimitFrom(q.RateLimit),
	}, nil
}

type searchQuery struct {
	Search struct {
		RepositoryCount githubv4.Int
		PageInfo        struct {
			HasNextPage githubv4.Boolean
			EndCursor   githubv4.String
		}
		Nodes []struct {
			Repository struct {
				ID             githubv4.String
				Name           githubv4.String
				Description    *githubv4.String
				URL            githubv4.String
				HomepageURL    *githubv4.String
				StargazerCount githubv4.Int
				ForkCount      githubv4.Int
				IsArchived     githubv4.Boolean
				IsFork         githubv4.Boolean
				CreatedAt      githubv4.DateTime
				UpdatedAt      githubv4.DateTime
				PushedAt       githubv4.DateTime
				Owner          struct {
					Login githubv4.String
				}
				PrimaryLanguage *struct {
					Name githubv4.String
				}
				LicenseInfo *struct {
					Name githubv4.String
				}
				RepositoryTopics struct {
					Nodes []struct {
						Topic struct {
							Name githubv4.String
						}
					}
				} `graphql:"repositoryTopics(first: 20)"`
			} `graphql:"... on Repository"`
		}
	} `graphql:"search(query: $q, type: REPOSITORY, first: $first, after: $after)"`
	RateLimit rateLimitFragment
}

// SearchRepos executes a GitHub search query and returns one page of results.
// Pass after="" for the first page.
func (c *APIClient) SearchRepos(query string, first int, after string) (*SearchResult, error) {
	var q searchQuery
	var cursor *githubv4.String
	if after != "" {
		c := githubv4.String(after)
		cursor = &c
	}
	vars := map[string]any{
		"q":     githubv4.String(query),
		"first": githubv4.Int(first),
		"after": cursor,
	}
	if err := c.gql.Query(context.Background(), &q, vars); err != nil {
		return nil, fmt.Errorf("search %q: %w", query, err)
	}

	res := &SearchResult{
		Total:     int(q.Search.RepositoryCount),
		HasNext:   bool(q.Search.PageInfo.HasNextPage),
		EndCursor: string(q.Search.PageInfo.EndCursor),
		RateLimit: rateLimitFrom(q.RateLimit),
	}
	for _, n := range q.Search.Nodes {
		rd := RepoData{
			ID:             string(n.Repository.ID),
			Owner:          Owner{Login: string(n.Repository.Owner.Login)},
			Name:           string(n.Repository.Name),
			URL:            string(n.Repository.URL),
			StargazerCount: int(n.Repository.StargazerCount),
			ForkCount:      int(n.Repository.ForkCount),
			IsArchived:     bool(n.Repository.IsArchived),
			IsFork:         bool(n.Repository.IsFork),
			CreatedAt:      n.Repository.CreatedAt.Format(time.RFC3339),
			UpdatedAt:      n.Repository.UpdatedAt.Format(time.RFC3339),
			PushedAt:       n.Repository.PushedAt.Format(time.RFC3339),
		}
		if n.Repository.Description != nil {
			s := string(*n.Repository.Description)
			rd.Description = &s
		}
		if n.Repository.HomepageURL != nil {
			s := string(*n.Repository.HomepageURL)
			rd.HomepageURL = &s
		}
		if n.Repository.PrimaryLanguage != nil {
			rd.PrimaryLang = &Language{Name: string(n.Repository.PrimaryLanguage.Name)}
		}
		if n.Repository.LicenseInfo != nil {
			rd.LicenseInfo = &LicenseInfo{Name: string(n.Repository.LicenseInfo.Name)}
		}
		for _, tn := range n.Repository.RepositoryTopics.Nodes {
			rd.Topics = append(rd.Topics, Topic{Name: string(tn.Topic.Name)})
		}
		res.Repos = append(res.Repos, rd)
	}
	return res, nil
}

func repoFromQuery(q repoQuery) RepoData {
	r := q.Repository
	rd := RepoData{
		ID:             string(r.ID),
		Owner:          Owner{Login: string(r.Owner.Login)},
		Name:           string(r.Name),
		URL:            string(r.URL),
		StargazerCount: int(r.StargazerCount),
		ForkCount:      int(r.ForkCount),
		IsArchived:     bool(r.IsArchived),
		IsFork:         bool(r.IsFork),
		CreatedAt:      r.CreatedAt.Format(time.RFC3339),
		UpdatedAt:      r.UpdatedAt.Format(time.RFC3339),
		PushedAt:       r.PushedAt.Format(time.RFC3339),
	}
	if r.Description != nil {
		s := string(*r.Description)
		rd.Description = &s
	}
	if r.HomepageURL != nil {
		s := string(*r.HomepageURL)
		rd.HomepageURL = &s
	}
	if r.PrimaryLanguage != nil {
		rd.PrimaryLang = &Language{Name: string(r.PrimaryLanguage.Name)}
	}
	if r.LicenseInfo != nil {
		rd.LicenseInfo = &LicenseInfo{Name: string(r.LicenseInfo.Name)}
	}
	for _, tn := range r.RepositoryTopics.Nodes {
		rd.Topics = append(rd.Topics, Topic{Name: string(tn.Topic.Name)})
	}
	return rd
}

func rateLimitFrom(rl rateLimitFragment) RateLimitInfo {
	return RateLimitInfo{
		Limit:     int(rl.Limit),
		Remaining: int(rl.Remaining),
		Cost:      int(rl.Cost),
		ResetAt:   rl.ResetAt.Time,
	}
}
