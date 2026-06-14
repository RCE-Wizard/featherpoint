// Package transport handles outbound mTLS HTTP communication from agent to server.
package transport

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client is an outbound mTLS HTTP client.
type Client struct {
	http    *http.Client
	baseURL string
	agentID string
}

// Config holds connection parameters for the transport.
type Config struct {
	BaseURL   string
	AgentID   string
	TLSConfig *tls.Config // nil = plain HTTP (dev only)
}

func New(cfg Config) (*Client, error) {
	transport := &http.Transport{}
	if cfg.TLSConfig != nil {
		transport.TLSClientConfig = cfg.TLSConfig
	}
	return &Client{
		http:    &http.Client{Timeout: 30 * time.Second, Transport: transport},
		baseURL: cfg.BaseURL,
		agentID: cfg.AgentID,
	}, nil
}

// NewPlain creates an unauthenticated client for Phase 1 / local testing.
func NewPlain(baseURL, agentID string) *Client {
	return &Client{
		http:    &http.Client{Timeout: 30 * time.Second},
		baseURL: baseURL,
		agentID: agentID,
	}
}

// Post sends a gzip-compressed JSON request with exponential retry backoff.
func (c *Client) Post(ctx context.Context, path string, req, resp interface{}) error {
	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	var gz bytes.Buffer
	w := gzip.NewWriter(&gz)
	_, _ = w.Write(body)
	w.Close()
	compressed := gz.Bytes()

	var lastErr error
	for attempt := 0; attempt < 5; attempt++ {
		if attempt > 0 {
			wait := time.Duration(1<<uint(attempt-1)) * time.Second
			if wait > 30*time.Second {
				wait = 30 * time.Second
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(wait):
			}
		}
		lastErr = c.doPost(ctx, path, compressed, resp)
		if lastErr == nil {
			return nil
		}
		if httpErr, ok := lastErr.(*HTTPError); ok && httpErr.Code < 500 {
			return lastErr
		}
	}
	return fmt.Errorf("after 5 attempts: %w", lastErr)
}

func (c *Client) doPost(ctx context.Context, path string, body []byte, resp interface{}) error {
	r, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Content-Encoding", "gzip")

	res, err := c.http.Do(r)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	raw, _ := io.ReadAll(res.Body)
	if res.StatusCode >= 300 {
		return &HTTPError{Code: res.StatusCode, Body: string(raw)}
	}
	return json.Unmarshal(raw, resp)
}

// HTTPError wraps a non-2xx HTTP response.
type HTTPError struct {
	Code int
	Body string
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("HTTP %d: %s", e.Code, e.Body)
}
