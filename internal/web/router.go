package web

import (
	"bytes"
	"context"
	"encoding/json"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"time"

	assets "earnings-dashboard/web"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type Readiness interface{ Ready(context.Context) error }

func NewRouter(db Readiness, timeout time.Duration, logger *slog.Logger, apps ...*App) http.Handler {
	page := template.Must(template.ParseFS(assets.Files, "templates/base.html"))
	static, err := fs.Sub(assets.Files, "static")
	if err != nil {
		panic(err)
	}
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if recover() != nil {
					logger.Error("request panic", "request_id", middleware.GetReqID(r.Context()))
					http.Error(w, "Internal server error", http.StatusInternalServerError)
				}
			}()
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("Content-Security-Policy", "default-src 'self'; img-src 'self' https:; style-src 'self' 'unsafe-inline'; frame-ancestors 'none'; base-uri 'none'; form-action 'self'")
			next.ServeHTTP(w, r)
		})
	})
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), timeout)
		defer cancel()
		status, database := http.StatusOK, "ok"
		if err := db.Ready(ctx); err != nil {
			status, database = http.StatusServiceUnavailable, "unavailable"
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(struct {
			Application string `json:"application"`
			Database    string `json:"database"`
		}{"ok", database})
	})
	r.Get("/", func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, "/calendar", http.StatusSeeOther) })
	if len(apps) > 0 && apps[0] != nil {
		apps[0].register(r)
	} else {
		r.Get("/calendar", func(w http.ResponseWriter, r *http.Request) {
			var body bytes.Buffer
			if err := page.Execute(&body, nil); err != nil {
				http.Error(w, "Unable to render page", http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write(body.Bytes())
		})
	}
	r.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.FS(static))))
	return r
}
