package linear

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/rbright/waybar-linear/internal/config"
)

type Notification struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Subtitle string `json:"subtitle"`
	URL      string `json:"url"`
}

type FetchResult struct {
	URLKey      string
	UnreadCount int
	UnreadIDs   []string
	Items       []Notification
}

type graphQLResponse struct {
	Data struct {
		Notifications struct {
			Nodes []struct {
				ID             string     `json:"id"`
				Title          string     `json:"title"`
				Subtitle       string     `json:"subtitle"`
				URL            string     `json:"url"`
				ReadAt         *time.Time `json:"readAt"`
				SnoozedUntilAt *time.Time `json:"snoozedUntilAt"`
			} `json:"nodes"`
		} `json:"notifications"`
		Organization struct {
			URLKey string `json:"urlKey"`
		} `json:"organization"`
	} `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

type mutationResponse struct {
	Data struct {
		NotificationUpdate struct {
			Success bool `json:"success"`
		} `json:"notificationUpdate"`
	} `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

const notificationsQuery = `query WaybarLinearNotifications {
  notifications {
    nodes {
      id
      title
      subtitle
      url
      readAt
      snoozedUntilAt
    }
  }
  organization {
    urlKey
  }
}`

const markReadMutation = `mutation MarkLinearNotificationRead($id: String!, $readAt: DateTime!) {
  notificationUpdate(id: $id, input: { readAt: $readAt }) {
    success
  }
}`

func FetchNotifications(ctx context.Context, cfg config.Runtime) (FetchResult, error) {
	raw, err := doGraphQL(ctx, cfg, notificationsQuery, map[string]any{})
	if err != nil {
		return FetchResult{}, err
	}

	var response graphQLResponse
	if err := json.Unmarshal(raw, &response); err != nil {
		return FetchResult{}, fmt.Errorf("decode notifications response: %w", err)
	}
	if len(response.Errors) > 0 {
		return FetchResult{}, fmt.Errorf("linear graphql error: %s", strings.TrimSpace(response.Errors[0].Message))
	}

	now := time.Now().UTC()
	items := make([]Notification, 0, cfg.MaxItems)
	ids := make([]string, 0)
	count := 0

	for _, node := range response.Data.Notifications.Nodes {
		if node.ReadAt != nil {
			continue
		}
		if node.SnoozedUntilAt != nil && node.SnoozedUntilAt.After(now) {
			continue
		}
		if strings.TrimSpace(node.ID) == "" || strings.TrimSpace(node.URL) == "" {
			continue
		}

		count++
		ids = append(ids, strings.TrimSpace(node.ID))

		if len(items) < cfg.MaxItems {
			items = append(items, Notification{
				ID:       strings.TrimSpace(node.ID),
				Title:    sanitize(node.Title),
				Subtitle: sanitize(node.Subtitle),
				URL:      strings.TrimSpace(node.URL),
			})
		}
	}

	return FetchResult{
		URLKey:      strings.TrimSpace(response.Data.Organization.URLKey),
		UnreadCount: count,
		UnreadIDs:   ids,
		Items:       items,
	}, nil
}

func MarkRead(ctx context.Context, cfg config.Runtime, notificationID string) error {
	id := strings.TrimSpace(notificationID)
	if id == "" {
		return fmt.Errorf("notification id is required")
	}

	readAt := time.Now().UTC().Format(time.RFC3339)
	raw, err := doGraphQL(ctx, cfg, markReadMutation, map[string]any{
		"id":     id,
		"readAt": readAt,
	})
	if err != nil {
		return err
	}

	var response mutationResponse
	if err := json.Unmarshal(raw, &response); err != nil {
		return fmt.Errorf("decode mark-read response: %w", err)
	}
	if len(response.Errors) > 0 {
		return fmt.Errorf("linear graphql error: %s", strings.TrimSpace(response.Errors[0].Message))
	}
	if !response.Data.NotificationUpdate.Success {
		return fmt.Errorf("linear notificationUpdate returned success=false")
	}
	return nil
}

func doGraphQL(ctx context.Context, cfg config.Runtime, query string, variables map[string]any) ([]byte, error) {
	payload := map[string]any{
		"query":     query,
		"variables": variables,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal graphql payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.APIURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create graphql request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", cfg.APIKey)

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
		return nil, fmt.Errorf("linear graphql status %d: %s", resp.StatusCode, strings.TrimSpace(string(responseBody)))
	}

	return responseBody, nil
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
