package config

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"time"
	_ "time/tzdata"
)

type Config struct {
	Environment     string
	Port            string
	DatabaseURL     string
	DatabaseTimeout time.Duration
	ShutdownTimeout time.Duration
	MarketLocation  *time.Location
}

func Load() (Config, error) { return load(os.LookupEnv) }

func load(lookup func(string) (string, bool)) (Config, error) {
	get := func(key, fallback string) string {
		if value, ok := lookup(key); ok {
			return value
		}
		return fallback
	}
	c := Config{Environment: get("APP_ENV", "development"), Port: get("PORT", "8080"), DatabaseURL: get("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/earnings_dashboard?sslmode=disable")}
	if c.Environment != "development" && c.Environment != "test" && c.Environment != "production" {
		return Config{}, fmt.Errorf("APP_ENV must be development, test, or production")
	}
	port, err := strconv.Atoi(c.Port)
	if err != nil || port < 1 || port > 65535 {
		return Config{}, fmt.Errorf("PORT must be between 1 and 65535")
	}
	if c.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL must not be empty")
	}
	if c.Environment == "production" {
		if _, ok := lookup("DATABASE_URL"); !ok {
			return Config{}, fmt.Errorf("DATABASE_URL is required in production")
		}
	}
	for _, item := range []struct {
		key, fallback string
		target        *time.Duration
	}{{"DATABASE_TIMEOUT", "5s", &c.DatabaseTimeout}, {"SHUTDOWN_TIMEOUT", "10s", &c.ShutdownTimeout}} {
		d, err := time.ParseDuration(get(item.key, item.fallback))
		if err != nil || d <= 0 || d > time.Minute {
			return Config{}, fmt.Errorf("%s must be a positive duration up to 1m", item.key)
		}
		*item.target = d
	}
	c.MarketLocation, err = time.LoadLocation("America/New_York")
	if err != nil {
		return Config{}, fmt.Errorf("load market timezone: %w", err)
	}
	return c, nil
}

func (c Config) Address() string { return net.JoinHostPort("", c.Port) }
