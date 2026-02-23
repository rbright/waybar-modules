package httpx

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type StatusError struct {
	StatusCode int
	Body       string
}

func (e *StatusError) Error() string {
	if strings.TrimSpace(e.Body) == "" {
		return fmt.Sprintf("http %d", e.StatusCode)
	}
	return fmt.Sprintf("http %d: %s", e.StatusCode, truncate(e.Body, 220))
}

func IsStatus(err error, statusCode int) bool {
	var statusErr *StatusError
	if !AsStatusError(err, &statusErr) {
		return false
	}
	return statusErr.StatusCode == statusCode
}

func Is5xx(err error) bool {
	var statusErr *StatusError
	if !AsStatusError(err, &statusErr) {
		return false
	}
	return statusErr.StatusCode >= 500 && statusErr.StatusCode <= 599
}

func AsStatusError(err error, target **StatusError) bool {
	return errors.As(err, target)
}

func DoJSON[T any](
	ctx context.Context,
	method string,
	url string,
	headers map[string]string,
	body []byte,
	timeout time.Duration,
) (T, error) {
	var zero T

	requestBody := io.Reader(nil)
	if len(body) > 0 {
		requestBody = bytes.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, requestBody)
	if err != nil {
		return zero, fmt.Errorf("create request: %w", err)
	}

	for key, value := range headers {
		req.Header.Set(key, value)
	}

	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	if err != nil {
		return zero, fmt.Errorf("request failed: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	payload, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return zero, fmt.Errorf("read response: %w", readErr)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return zero, &StatusError{StatusCode: resp.StatusCode, Body: strings.TrimSpace(string(payload))}
	}

	var decoded T
	if err := json.Unmarshal(payload, &decoded); err != nil {
		return zero, fmt.Errorf("decode json response: %w", err)
	}

	return decoded, nil
}

func EncodeJSON(v any) ([]byte, error) {
	payload, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("encode json body: %w", err)
	}
	return payload, nil
}

func truncate(value string, max int) string {
	if max <= 0 || len(value) <= max {
		return value
	}
	return value[:max] + "…"
}
