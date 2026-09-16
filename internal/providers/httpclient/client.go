// Package httpclient enforces bounded, cancellable requests and credential-safe errors.
package httpclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"
)

type StatusError struct{ Code int }

func (e *StatusError) Error() string { return fmt.Sprintf("provider HTTP %d", e.Code) }

type Client struct {
	HTTP      *http.Client
	UserAgent string
	Interval  time.Duration
	mu        sync.Mutex
	next      time.Time
}

func New(agent string, interval time.Duration) *Client {
	return &Client{HTTP: &http.Client{Timeout: 20 * time.Second}, UserAgent: agent, Interval: interval}
}
func wait(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
func (c *Client) limit(ctx context.Context) error {
	c.mu.Lock()
	now := time.Now()
	at := c.next
	if at.Before(now) {
		at = now
	}
	c.next = at.Add(c.Interval)
	c.mu.Unlock()
	return wait(ctx, time.Until(at))
}
func (c *Client) Do(ctx context.Context, method, url string, body []byte) ([]byte, error) {
	for attempt := 0; attempt < 3; attempt++ {
		if err := c.limit(ctx); err != nil {
			return nil, err
		}
		req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(body))
		if err != nil {
			return nil, errors.New("invalid provider request")
		}
		req.Header.Set("User-Agent", c.UserAgent)
		req.Header.Set("Accept", "application/json")
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		res, err := c.HTTP.Do(req)
		if err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			return nil, errors.New("provider transport failure")
		}
		data, readErr := io.ReadAll(io.LimitReader(res.Body, 32<<20))
		res.Body.Close()
		if res.StatusCode >= 200 && res.StatusCode < 300 {
			if readErr != nil {
				return nil, errors.New("provider response read failure")
			}
			return data, nil
		}
		if attempt == 2 || (res.StatusCode != 429 && res.StatusCode < 500) {
			return nil, &StatusError{res.StatusCode}
		}
		delay := time.Duration(1<<attempt) * time.Second
		if seconds, e := strconv.Atoi(res.Header.Get("Retry-After")); e == nil && seconds > 0 {
			if seconds > 30 {
				return nil, &StatusError{res.StatusCode}
			}
			delay = time.Duration(seconds) * time.Second
		}
		if err := wait(ctx, delay); err != nil {
			return nil, err
		}
	}
	return nil, errors.New("provider unavailable")
}
func (c *Client) JSON(ctx context.Context, method, url string, body any, out any) error {
	var payload []byte
	var err error
	if body != nil {
		payload, err = json.Marshal(body)
		if err != nil {
			return errors.New("invalid provider request")
		}
	}
	data, err := c.Do(ctx, method, url, payload)
	if err != nil {
		return err
	}
	if err = json.Unmarshal(data, out); err != nil {
		return errors.New("invalid provider JSON")
	}
	return nil
}
