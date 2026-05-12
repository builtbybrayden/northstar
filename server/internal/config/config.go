package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	ListenAddr  string
	DBPath      string
	LogLevel    string
	PairingTTL  time.Duration
	AdminToken  string // Optional: gates /api/pair/initiate. Empty = open (warns on boot).

	Pillars struct {
		Finance bool
		Goals   bool
		Health  bool
		AI      bool
	}

	Finance struct {
		SidecarURL    string
		SidecarSecret string
		SyncInterval  time.Duration
		Enabled       bool
	}

	Health struct {
		SidecarURL    string
		SidecarSecret string
		SyncInterval  time.Duration
		Enabled       bool
	}

	AI struct {
		Mode     string // anthropic | mock
		APIKey   string
		Model    string
	}
}

func Load() Config {
	c := Config{
		ListenAddr: env("NORTHSTAR_LISTEN_ADDR", ":8080"),
		DBPath:     env("NORTHSTAR_DB_PATH", "./data/northstar.db"),
		LogLevel:   env("NORTHSTAR_LOG_LEVEL", "info"),
		PairingTTL: time.Duration(envInt("NORTHSTAR_PAIRING_TTL", 600)) * time.Second,
		AdminToken: env("NORTHSTAR_ADMIN_TOKEN", ""),
	}
	c.Pillars.Finance = envInt("NORTHSTAR_PILLAR_FINANCE", 1) == 1
	c.Pillars.Goals = envInt("NORTHSTAR_PILLAR_GOALS", 1) == 1
	c.Pillars.Health = envInt("NORTHSTAR_PILLAR_HEALTH", 1) == 1
	c.Pillars.AI = envInt("NORTHSTAR_PILLAR_AI", 1) == 1

	c.Finance.SidecarURL = env("NORTHSTAR_FINANCE_SIDECAR_URL", "http://127.0.0.1:9090")
	c.Finance.SidecarSecret = env("NORTHSTAR_FINANCE_SIDECAR_SECRET", "")
	c.Finance.SyncInterval = time.Duration(envInt("NORTHSTAR_FINANCE_SYNC_SECONDS", 900)) * time.Second
	c.Finance.Enabled = c.Pillars.Finance && envInt("NORTHSTAR_FINANCE_SYNC_ENABLED", 1) == 1

	c.Health.SidecarURL = env("NORTHSTAR_HEALTH_SIDECAR_URL", "http://127.0.0.1:9091")
	c.Health.SidecarSecret = env("NORTHSTAR_HEALTH_SIDECAR_SECRET", "")
	c.Health.SyncInterval = time.Duration(envInt("NORTHSTAR_HEALTH_SYNC_SECONDS", 900)) * time.Second
	c.Health.Enabled = c.Pillars.Health && envInt("NORTHSTAR_HEALTH_SYNC_ENABLED", 1) == 1

	c.AI.Mode = env("NORTHSTAR_AI_MODE", "mock")     // mock by default — no API key needed
	c.AI.APIKey = env("NORTHSTAR_CLAUDE_API_KEY", "")
	c.AI.Model = env("NORTHSTAR_AI_MODEL", "claude-sonnet-4-6")
	return c
}

func env(k, def string) string {
	if v, ok := os.LookupEnv(k); ok && v != "" {
		return v
	}
	return def
}

func envInt(k string, def int) int {
	if v, ok := os.LookupEnv(k); ok && v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}
