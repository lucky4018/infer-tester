package client

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"infer-tester/internal/config"
)

type RawResp struct {
	Status int
	Body   []byte
}

type SSEEvent struct {
	Data string
	Done bool
}

type Client struct {
	base    string
	apiKey  string
	Timeout time.Duration
	http    *http.Client
}

func New(cfg *config.Config) *Client {
	timeout := time.Duration(cfg.Server.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 120 * time.Second
	}
	return &Client{
		base:    strings.TrimRight(cfg.Server.BaseURL, "/"),
		apiKey:  cfg.Server.APIKey,
		Timeout: timeout,
		http:    &http.Client{Timeout: timeout},
	}
}

func (c *Client) Do(ctx context.Context, method, path string, body any) (*RawResp, error) {
	var r io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal request: %w", err)
		}
		r = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.base+path, r)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return &RawResp{Status: resp.StatusCode, Body: data}, nil
}

func (c *Client) Get(ctx context.Context, path string) (*RawResp, error) {
	return c.Do(ctx, http.MethodGet, path, nil)
}

func (c *Client) Post(ctx context.Context, path string, body any) (*RawResp, error) {
	return c.Do(ctx, http.MethodPost, path, body)
}

func (c *Client) JsonPost(ctx context.Context, path string, reqBody any, out any) (int, error) {
	r, err := c.Post(ctx, path, reqBody)
	if err != nil {
		return 0, err
	}
	if out != nil {
		if err := json.Unmarshal(r.Body, out); err != nil {
			return r.Status, fmt.Errorf("decode response (status=%d): %w", r.Status, err)
		}
	}
	return r.Status, nil
}

func (c *Client) JsonGet(ctx context.Context, path string, out any) error {
	r, err := c.Get(ctx, path)
	if err != nil {
		return err
	}
	return json.Unmarshal(r.Body, out)
}

// ResolveModelName queries /v1/models and returns the ID of the first available model.
func (c *Client) ResolveModelName(ctx context.Context) (string, error) {
	var r struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := c.JsonGet(ctx, "/v1/models", &r); err != nil {
		return "", fmt.Errorf("query /v1/models: %w", err)
	}
	if len(r.Data) == 0 {
		return "", fmt.Errorf("/v1/models returned empty list")
	}
	return r.Data[0].ID, nil
}

// StreamTTFT sends a streaming request and returns time-to-first-token, total duration, and chunk count.
func (c *Client) StreamTTFT(ctx context.Context, path string, body any) (ttft, total time.Duration, chunks int, err error) {
	b, err := json.Marshal(body)
	if err != nil {
		return 0, 0, 0, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+path, bytes.NewReader(b))
	if err != nil {
		return 0, 0, 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	t0 := time.Now()
	resp, err := c.http.Do(req)
	if err != nil {
		return 0, 0, 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		return 0, 0, 0, fmt.Errorf("stream: status=%d body=%s", resp.StatusCode, raw)
	}

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "[DONE]" {
			break
		}
		if ttft == 0 {
			ttft = time.Since(t0)
		}
		chunks++
	}
	return ttft, time.Since(t0), chunks, scanner.Err()
}

// Stream sends a streaming request and collects all SSE events.
func (c *Client) Stream(ctx context.Context, path string, body any) ([]SSEEvent, time.Duration, error) {
	b, err := json.Marshal(body)
	if err != nil {
		return nil, 0, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+path, bytes.NewReader(b))
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	t0 := time.Now()
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, 0, fmt.Errorf("stream: status=%d body=%s", resp.StatusCode, body)
	}

	var events []SSEEvent
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "[DONE]" {
			events = append(events, SSEEvent{Done: true})
			break
		}
		events = append(events, SSEEvent{Data: data})
	}
	if err := scanner.Err(); err != nil {
		return events, time.Since(t0), err
	}
	return events, time.Since(t0), nil
}
