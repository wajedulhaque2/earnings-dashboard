package web

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type readinessFunc func(context.Context) error

func (f readinessFunc) Ready(ctx context.Context) error { return f(ctx) }

func TestHealth(t *testing.T) {
	for _, healthy := range []bool{true, false} {
		db := readinessFunc(func(ctx context.Context) error {
			if _, ok := ctx.Deadline(); !ok {
				t.Error("missing deadline")
			}
			if healthy {
				return nil
			}
			return errors.New("secret database credentials")
		})
		r := NewRouter(db, time.Second, slog.New(slog.NewTextHandler(io.Discard, nil)))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/health", nil))
		want := 200
		state := "ok"
		if !healthy {
			want = 503
			state = "unavailable"
		}
		if w.Code != want || w.Body.String() != `{"application":"ok","database":"`+state+`"}`+"\n" {
			t.Fatalf("health: %d %s", w.Code, w.Body.String())
		}
		if w.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("health must not be cached")
		}
	}
}

func TestHealthTimeout(t *testing.T) {
	r := NewRouter(readinessFunc(func(ctx context.Context) error { <-ctx.Done(); return ctx.Err() }), time.Millisecond, slog.Default())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/health", nil))
	if w.Code != 503 {
		t.Fatal(w.Code)
	}
}

func TestPanicDoesNotExposeSensitiveDetails(t *testing.T) {
	var logs strings.Builder
	r := NewRouter(readinessFunc(func(context.Context) error {
		panic("secret connection details")
	}), time.Second, slog.New(slog.NewJSONHandler(&logs, nil)))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/health", nil))
	if w.Code != 500 || strings.Contains(w.Body.String()+logs.String(), "secret") {
		t.Fatalf("panic was not safely handled: %d", w.Code)
	}
}

func TestPagesAndRouting(t *testing.T) {
	r := NewRouter(readinessFunc(func(context.Context) error { t.Fatal("page must not query providers or database"); return nil }), time.Second, slog.Default())
	for _, tc := range []struct {
		path     string
		code     int
		contains string
	}{{"/", 303, ""}, {"/calendar", 200, "No earnings data loaded"}, {"/static/app.css", 200, ":root"}, {"/missing", 404, ""}} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("GET", tc.path, nil))
		if w.Code != tc.code || !strings.Contains(w.Body.String(), tc.contains) {
			t.Fatalf("%s: %d %s", tc.path, w.Code, w.Body.String())
		}
		if tc.path == "/" && w.Header().Get("Location") != "/calendar" {
			t.Fatal("wrong redirect")
		}
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("POST", "/health", nil))
	if w.Code != 405 {
		t.Fatal(w.Code)
	}
}
