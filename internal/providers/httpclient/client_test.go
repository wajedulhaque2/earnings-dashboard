package httpclient

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestTransientRetry(t *testing.T) {
	for _, status := range []int{429, 503} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			calls := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if calls < 2 {
					w.WriteHeader(status)
					return
				}
				_, _ = w.Write([]byte(`{}`))
			}))
			defer srv.Close()
			c := New("test", 0)
			_, err := c.Do(context.Background(), "GET", srv.URL, nil)
			if err != nil || calls != 2 {
				t.Fatal(err, calls)
			}
		})
	}
}

func TestPlainTextAuthenticationResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Accept") == "application/json" {
			w.WriteHeader(http.StatusNotAcceptable)
			return
		}
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("fixture-token"))
	}))
	defer srv.Close()
	body, err := New("test", 0).Do(context.Background(), "GET", srv.URL, nil)
	if err != nil || string(body) != "fixture-token" {
		t.Fatalf("plain-text endpoint rejected: %v", err)
	}
}
func TestRetryDeadlineAndSafeErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(503) }))
	defer srv.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	_, err := New("test", 0).Do(ctx, "GET", srv.URL, nil)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal(err)
	}
}
