package github

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"time"
)

const maxRetries = 3

const endpoint = "https://api.github.com/graphql"

type Client struct {
	token      string
	httpClient *http.Client
}

func NewClient(token string) *Client {
	return &Client{
		token: token,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

type graphQLRequest struct {
	Query     string         `json:"query"`
	Variables map[string]any `json:"variables,omitempty"`
}

type graphQLResponse struct {
	Data   json.RawMessage `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

type RateLimit struct {
	Remaining int    `json:"remaining"`
	ResetAt   string `json:"resetAt"`
}

type RepoData struct {
	ID             string `json:"id"`
	DatabaseID     int    `json:"databaseId"`
	Name           string `json:"name"`
	NameWithOwner  string `json:"nameWithOwner"`
	Owner          struct {
		Login string `json:"login"`
	} `json:"owner"`
	Description    *string `json:"description"`
	URL            string  `json:"url"`
	HomepageURL    *string `json:"homepageUrl"`
	StargazerCount int     `json:"stargazerCount"`
	ForkCount      int     `json:"forkCount"`
	PrimaryLanguage *struct {
		Name string `json:"name"`
	} `json:"primaryLanguage"`
	RepositoryTopics struct {
		Nodes []struct {
			Topic struct {
				Name string `json:"name"`
			} `json:"topic"`
		} `json:"nodes"`
	} `json:"repositoryTopics"`
	LicenseInfo *struct {
		SpdxID string `json:"spdxId"`
		Name   string `json:"name"`
	} `json:"licenseInfo"`
	IsArchived bool   `json:"isArchived"`
	IsFork     bool   `json:"isFork"`
	CreatedAt  string `json:"createdAt"`
	UpdatedAt  string `json:"updatedAt"`
	PushedAt   string `json:"pushedAt"`
}

type FetchResult struct {
	Repo      RepoData
	RateLimit RateLimit
}

const repoQuery = `
query ($owner: String!, $name: String!) {
  repository(owner: $owner, name: $name) {
    id
    databaseId
    name
    nameWithOwner
    owner { login }
    description
    url
    homepageUrl
    stargazerCount
    forkCount
    primaryLanguage { name }
    repositoryTopics(first: 20) {
      nodes { topic { name } }
    }
    licenseInfo { spdxId name }
    isArchived
    isFork
    createdAt
    updatedAt
    pushedAt
  }
  rateLimit { remaining resetAt }
}
`

func (c *Client) FetchRepo(owner, name string) (*FetchResult, error) {
	vars := map[string]any{"owner": owner, "name": name}
	body, err := c.do(repoQuery, vars)
	if err != nil {
		return nil, err
	}

	var result struct {
		Repository RepoData  `json:"repository"`
		RateLimit  RateLimit `json:"rateLimit"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}
	return &FetchResult{Repo: result.Repository, RateLimit: result.RateLimit}, nil
}

func (c *Client) do(query string, vars map[string]any) (json.RawMessage, error) {
	reqBody, err := json.Marshal(graphQLRequest{Query: query, Variables: vars})
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			wait := time.Duration(1<<(attempt-1)) * time.Second
			jitter := time.Duration(rand.Intn(500)) * time.Millisecond
			log.Printf("Retry %d/%d after %v", attempt, maxRetries-1, wait+jitter)
			time.Sleep(wait + jitter)
		}

		req, err := http.NewRequest("POST", endpoint, bytes.NewReader(reqBody))
		if err != nil {
			return nil, fmt.Errorf("new request: %w", err)
		}
		req.Header.Set("Authorization", "bearer "+c.token)
		req.Header.Set("Content-Type", "application/json")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("http do: %w", err)
			continue // network error → retry
		}

		respBody, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			lastErr = fmt.Errorf("read body: %w", err)
			continue
		}

		if resp.StatusCode == 502 || resp.StatusCode == 503 {
			lastErr = fmt.Errorf("server error (status %d)", resp.StatusCode)
			continue // transient server error → retry
		}
		if resp.StatusCode == 403 || resp.StatusCode == 429 {
			return nil, fmt.Errorf("rate limited (status %d): %s", resp.StatusCode, respBody)
		}
		if resp.StatusCode != 200 {
			return nil, fmt.Errorf("http %d: %s", resp.StatusCode, respBody)
		}

		var gqlResp graphQLResponse
		if err := json.Unmarshal(respBody, &gqlResp); err != nil {
			return nil, fmt.Errorf("unmarshal response: %w", err)
		}
		if len(gqlResp.Errors) > 0 {
			return nil, fmt.Errorf("graphql error: %s", gqlResp.Errors[0].Message)
		}
		return gqlResp.Data, nil
	}

	return nil, fmt.Errorf("max retries exceeded: %w", lastErr)
}

type SearchResult struct {
	Repos     []RepoData
	Total     int
	HasNext   bool
	EndCursor string
	RateLimit RateLimit
}

const searchReposQuery = `
query ($query: String!, $first: Int!, $after: String) {
  search(query: $query, type: REPOSITORY, first: $first, after: $after) {
    repositoryCount
    pageInfo {
      hasNextPage
      endCursor
    }
    nodes {
      ... on Repository {
        id
        databaseId
        name
        nameWithOwner
        owner { login }
        description
        url
        homepageUrl
        stargazerCount
        forkCount
        primaryLanguage { name }
        repositoryTopics(first: 20) {
          nodes { topic { name } }
        }
        licenseInfo { spdxId name }
        isArchived
        isFork
        createdAt
        updatedAt
        pushedAt
      }
    }
  }
  rateLimit { remaining resetAt }
}
`

func (c *Client) SearchRepos(query string, perPage int, after string) (*SearchResult, error) {
	vars := map[string]any{
		"query": query,
		"first": perPage,
	}
	if after != "" {
		vars["after"] = after
	}

	body, err := c.do(searchReposQuery, vars)
	if err != nil {
		return nil, err
	}

	var result struct {
		Search struct {
			RepositoryCount int `json:"repositoryCount"`
			PageInfo        struct {
				HasNextPage bool   `json:"hasNextPage"`
				EndCursor   string `json:"endCursor"`
			} `json:"pageInfo"`
			Nodes []json.RawMessage `json:"nodes"`
		} `json:"search"`
		RateLimit RateLimit `json:"rateLimit"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("unmarshal search: %w", err)
	}

	var repos []RepoData
	for _, raw := range result.Search.Nodes {
		if string(raw) == "null" {
			continue
		}
		var r RepoData
		if err := json.Unmarshal(raw, &r); err != nil {
			continue
		}
		if r.ID == "" {
			continue
		}
		repos = append(repos, r)
	}

	return &SearchResult{
		Repos:     repos,
		Total:     result.Search.RepositoryCount,
		HasNext:   result.Search.PageInfo.HasNextPage,
		EndCursor: result.Search.PageInfo.EndCursor,
		RateLimit: result.RateLimit,
	}, nil
}

// FetchReposBatch fetches up to 20 repos in a single GraphQL request using aliases.
func (c *Client) FetchReposBatch(slugs []string) (*BatchResult, error) {
	if len(slugs) == 0 {
		return &BatchResult{Repos: make(map[string]RepoData)}, nil
	}
	if len(slugs) > 20 {
		slugs = slugs[:20]
	}

	vars := make(map[string]any)
	var q bytes.Buffer

	// Variable declarations
	q.WriteString("query(")
	first := true
	for i, slug := range slugs {
		if splitSlug(slug) == nil {
			continue
		}
		if !first {
			q.WriteString(", ")
		}
		fmt.Fprintf(&q, "$owner%d: String!, $name%d: String!", i, i)
		first = false
	}
	q.WriteString(") {")

	// Aliased repository fields
	for i, slug := range slugs {
		parts := splitSlug(slug)
		if parts == nil {
			continue
		}
		vars[fmt.Sprintf("owner%d", i)] = parts[0]
		vars[fmt.Sprintf("name%d", i)] = parts[1]
		fmt.Fprintf(&q, "\n  repo%d: repository(owner: $owner%d, name: $name%d) { ...RepoFields }", i, i, i)
	}

	q.WriteString(`
  rateLimit { remaining resetAt }
}
fragment RepoFields on Repository {
  id databaseId name nameWithOwner
  owner { login } description url homepageUrl
  stargazerCount forkCount
  primaryLanguage { name }
  repositoryTopics(first: 20) { nodes { topic { name } } }
  licenseInfo { spdxId name }
  isArchived isFork createdAt updatedAt pushedAt
}`)

	body, err := c.do(q.String(), vars)
	if err != nil {
		return nil, err
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("unmarshal batch: %w", err)
	}

	result := &BatchResult{Repos: make(map[string]RepoData)}
	for i, slug := range slugs {
		key := fmt.Sprintf("repo%d", i)
		data, ok := raw[key]
		if !ok || string(data) == "null" {
			continue
		}
		var repo RepoData
		if err := json.Unmarshal(data, &repo); err != nil {
			log.Printf("WARN: unmarshal batch repo %s: %v", slug, err)
			continue
		}
		result.Repos[slug] = repo
	}

	if rlData, ok := raw["rateLimit"]; ok {
		json.Unmarshal(rlData, &result.RateLimit)
	}

	return result, nil
}

type BatchResult struct {
	Repos     map[string]RepoData
	RateLimit RateLimit
}

func splitSlug(slug string) []string {
	for i, c := range slug {
		if c == '/' {
			if i > 0 && i < len(slug)-1 {
				return []string{slug[:i], slug[i+1:]}
			}
			return nil
		}
	}
	return nil
}

func (c *Client) CheckRateLimit(rl RateLimit) {
	if rl.Remaining < 100 {
		resetAt, err := time.Parse(time.RFC3339, rl.ResetAt)
		if err != nil {
			log.Printf("WARN: rate limit low (%d remaining), can't parse resetAt", rl.Remaining)
			time.Sleep(60 * time.Second)
			return
		}
		wait := time.Until(resetAt) + time.Second
		if wait > 0 {
			log.Printf("Rate limit low (%d remaining), waiting %v until reset", rl.Remaining, wait)
			time.Sleep(wait)
		}
	}
}
