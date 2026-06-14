// Package transport handles outbound mTLS HTTP communication from agent to server.
package transport

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// Client is an outbound mTLS HTTP client.
type Client struct {
	http      *http.Client
	baseURL   string
	agentID   string
}

// Config holds TLS material for the transport.
type Config struct {
	BaseURL        string
	AgentID        string
	ClientCertFile string
	ClientKeyFile  string
	CACertFile     string
}

func New(cfg Config) (*Client, error) {
	tlsCfg, err := buildTLS(cfg)
	if err != nil {
		return nil, err
	}
	return &Client{
		http: &http.Client{
			Timeout:   30 * time.Second,
			Transport: &http.Transport{TLSClientConfig: tlsCfg},
		},
		baseURL: cfg.BaseURL,
		agentID: cfg.AgentID,
	}, nil
}

// NewPlain creates a client without mTLS, for Phase 1 testing.
func NewPlain(baseURL, agentID string) *Client {
	return &Client{
		http:    &http.Client{Timeout: 30 * time.Second},
		baseURL: baseURL,
		agentID: agentID,
	}
}

// Post sends a JSON-encoded request with gzip compression and retry backoff.
func (c *Client) Post(ctx context.Context, path string, req, resp interface{}) error {
	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	var gz bytes.Buffer
	w := gzip.NewWriter(&gz)
	if _, err := w.Write(body); err != nil {
		return err
	}
	w.Close()

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

		lastErr = c.doPost(ctx, path, gz.Bytes(), resp)
		if lastErr == nil {
			return nil
		}
		// Don't retry on 4xx
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
	if c.agentID != "" {
		r.Header.Set("X-Agent-ID", c.agentID)
	}

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

// HTTPError wraps a non-2xx response.
type HTTPError struct {
	Code int
	Body string
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("HTTP %d: %s", e.Code, e.Body)
}

func buildTLS(cfg Config) (*tls.Config, error) {
	cert, err := tls.LoadX509KeyPair(cfg.ClientCertFile, cfg.ClientKeyFile)
	if err != nil {
		return nil, fmt.Errorf("load client cert: %w", err)
	}

	caData, err := os.ReadFile(cfg.CACertFile)
	if err != nil {
		return nil, fmt.Errorf("read CA cert: %w", err)
	}
	pool := x509.NewCertPool()
	pool.AppendCertsFromPEM(caData)

	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		RootCAs:      pool,
		MinVersion:   tls.VersionTLS13,
	}, nil
}
