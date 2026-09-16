package config

import (
	"strings"
	"testing"
	"time"
)

func TestDefaults(t *testing.T) {
	c, err := load(func(string) (string, bool) { return "", false })
	if err != nil {
		t.Fatal(err)
	}
	if c.Address() != ":8080" || c.DatabaseTimeout != 5*time.Second || c.MarketLocation.String() != "America/New_York" {
		t.Fatalf("unexpected defaults: %#v", c)
	}
	winter := time.Date(2026, 1, 1, 12, 0, 0, 0, c.MarketLocation)
	summer := time.Date(2026, 7, 1, 12, 0, 0, 0, c.MarketLocation)
	_, w := winter.Zone()
	_, s := summer.Zone()
	if w != -5*3600 || s != -4*3600 {
		t.Fatal("market timezone must observe DST")
	}
}

func TestConfigurationValidation(t *testing.T) {
	for _, tc := range []struct {
		name   string
		values map[string]string
		want   string
	}{
		{"bad port", map[string]string{"PORT": "abc"}, "PORT"},
		{"zero port", map[string]string{"PORT": "0"}, "PORT"},
		{"large port", map[string]string{"PORT": "65536"}, "PORT"},
		{"empty database", map[string]string{"DATABASE_URL": ""}, "DATABASE_URL"},
		{"production database", map[string]string{"APP_ENV": "production"}, "DATABASE_URL"},
		{"environment", map[string]string{"APP_ENV": "typo"}, "APP_ENV"},
		{"timeout", map[string]string{"DATABASE_TIMEOUT": "-1s"}, "DATABASE_TIMEOUT"},
		{"shutdown", map[string]string{"SHUTDOWN_TIMEOUT": "2m"}, "SHUTDOWN_TIMEOUT"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := load(func(k string) (string, bool) { v, ok := tc.values[k]; return v, ok })
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("expected %s error, got %v", tc.want, err)
			}
		})
	}
}

func TestOverrides(t *testing.T) {
	values := map[string]string{"APP_ENV": "production", "PORT": "9090", "DATABASE_URL": "postgres://localhost/custom", "DATABASE_TIMEOUT": "2s", "SHUTDOWN_TIMEOUT": "3s"}
	c, err := load(func(k string) (string, bool) { v, ok := values[k]; return v, ok })
	if err != nil {
		t.Fatal(err)
	}
	if c.Address() != ":9090" || c.DatabaseTimeout != 2*time.Second || c.ShutdownTimeout != 3*time.Second || c.DatabaseURL != values["DATABASE_URL"] {
		t.Fatal("overrides not honored")
	}
}
