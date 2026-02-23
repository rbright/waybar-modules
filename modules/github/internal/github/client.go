package github

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/rbright/waybar-github/internal/config"
)

type AuthMode string

const (
	AuthNone  AuthMode = "none"
	AuthGH    AuthMode = "gh"
	AuthToken AuthMode = "token"
)

type PullRequest struct {
	ID         string `json:"id"`
	Number     int    `json:"number"`
	Title      string `json:"title"`
	URL        string `json:"url"`
	Repository string `json:"repository"`
	IsDraft    bool   `json:"isDraft"`
	UpdatedAt  string `json:"updatedAt"`
}

type FetchResult struct {
	Count int
	Items []PullRequest
}

type graphQLResponse struct {
	Data struct {
		Search struct {
			IssueCount int `json:"issueCount"`
			Nodes      []struct {
				ID         string `json:"id"`
				Number     int    `json:"number"`
				Title      string `json:"title"`
				URL        string `json:"url"`
				IsDraft    bool   `json:"isDraft"`
				UpdatedAt  string `json:"updatedAt"`
				Repository struct {
					NameWithOwner string `json:"nameWithOwner"`
				} `json:"repository"`
			} `json:"nodes"`
		} `json:"search"`
	} `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

const searchQuery = `query WaybarGitHubPullRequests($searchQuery: String!, $limit: Int!) {
  search(type: ISSUE, query: $searchQuery, first: $limit) {
    issueCount
    nodes {
      ... on PullRequest {
        id
        number
        title
        url
        isDraft
        updatedAt
        repository {
          nameWithOwner
        }
      }
    }
  }
}`

func DetectAuth(ctx context.Context, cfg config.Runtime) AuthMode {
	if _, err := exec.LookPath("gh"); err == nil {
		cmd := exec.CommandContext(ctx, "gh", "auth", "status", "-h", cfg.Host)
		if err := cmd.Run(); err == nil {
			return AuthGH
		}
	}

	if strings.TrimSpace(cfg.Token) != "" {
		return AuthToken
	}

	return AuthNone
}

func FetchPullRequests(ctx context.Context, cfg config.Runtime, mode AuthMode) (FetchResult, error) {
	var raw []byte
	var err error

	switch mode {
	case AuthGH:
		raw, err = fetchWithGH(ctx, cfg)
	case AuthToken:
		raw, err = fetchWithToken(ctx, cfg)
	default:
		return FetchResult{}, fmt.Errorf("no supported auth mode")
	}
	if err != nil {
		return FetchResult{}, err
	}

	parsed, err := parseResponse(raw)
	if err != nil {
		return FetchResult{}, err
	}
	return parsed, nil
}

func fetchWithGH(ctx context.Context, cfg config.Runtime) ([]byte, error) {
	cmd := exec.CommandContext(
		ctx,
		"gh",
		"api",
		"graphql",
		"-f", "query="+searchQuery,
		"-F", "searchQuery="+cfg.PRQuery,
		"-F", "limit="+strconv.Itoa(cfg.MaxItems),
	)

	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("gh graphql request failed: %w", err)
	}

	return out, nil
}

func fetchWithToken(ctx context.Context, cfg config.Runtime) ([]byte, error) {
	payload := map[string]any{
		"query": searchQuery,
		"variables": map[string]any{
			"searchQuery": cfg.PRQuery,
			"limit":       cfg.MaxItems,
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal graphql payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.GraphQLURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create graphql request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("Authorization", "Bearer "+cfg.Token)

	client := &http.Client{Timeout: maxDuration(cfg.Timeout, 10*time.Second)}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("perform graphql request: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read graphql response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("github graphql status %d: %s", resp.StatusCode, strings.TrimSpace(string(responseBody)))
	}

	return responseBody, nil
}

func parseResponse(raw []byte) (FetchResult, error) {
	var response graphQLResponse
	if err := json.Unmarshal(raw, &response); err != nil {
		return FetchResult{}, fmt.Errorf("decode graphql response: %w", err)
	}

	if len(response.Errors) > 0 {
		return FetchResult{}, fmt.Errorf("graphql error: %s", strings.TrimSpace(response.Errors[0].Message))
	}

	items := make([]PullRequest, 0, len(response.Data.Search.Nodes))
	for _, node := range response.Data.Search.Nodes {
		if strings.TrimSpace(node.URL) == "" {
			continue
		}

		items = append(items, PullRequest{
			ID:         strings.TrimSpace(node.ID),
			Number:     node.Number,
			Title:      sanitize(node.Title),
			URL:        strings.TrimSpace(node.URL),
			Repository: sanitize(node.Repository.NameWithOwner),
			IsDraft:    node.IsDraft,
			UpdatedAt:  strings.TrimSpace(node.UpdatedAt),
		})
	}

	count := response.Data.Search.IssueCount
	if count <= 0 {
		count = len(items)
	}

	return FetchResult{Count: count, Items: items}, nil
}

func sanitize(value string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
}

func maxDuration(a, b time.Duration) time.Duration {
	if a > b {
		return a
	}
	return b
}
