package database

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestInvalidURLDoesNotLeakCredentials(t *testing.T) {
	_, err := Open(context.Background(), "postgres://user:super-secret@host:bad/db", time.Second)
	if err == nil || strings.Contains(err.Error(), "super-secret") {
		t.Fatalf("expected sanitized configuration error: %v", err)
	}
}

func TestPoolConfigurationWithoutDatabase(t *testing.T) {
	pool, err := Open(context.Background(), "postgres://localhost/test?sslmode=disable", 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	cfg := pool.Config()
	if cfg.ConnConfig.ConnectTimeout != 2*time.Second || cfg.ConnConfig.RuntimeParams["timezone"] != "UTC" || cfg.MaxConns != 10 {
		t.Fatal("unexpected pool configuration")
	}
}
