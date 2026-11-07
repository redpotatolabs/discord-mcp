// Package config loads settings from the environment, optionally seeded from
// an env file (default $XDG_CONFIG_HOME/discord-mcp/env).
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

// Config is the server configuration.
type Config struct {
	Token        string
	DefaultGuild string
	// AllowedGuilds restricts every call to these guilds; empty = unrestricted.
	AllowedGuilds []string
	ReadOnly      bool
	LogLevel      string
	BaseURL       string
}

// DefaultEnvFile returns the env file path used when DISCORD_MCP_ENV_FILE is unset.
func DefaultEnvFile() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "discord-mcp", "env")
}

// Load reads configuration. Real environment variables win over the env file.
func Load() (Config, error) {
	envFile := os.Getenv("DISCORD_MCP_ENV_FILE")
	if envFile == "" {
		envFile = DefaultEnvFile()
	}
	if envFile != "" {
		if err := godotenv.Load(envFile); err != nil && !errors.Is(err, os.ErrNotExist) {
			return Config{}, fmt.Errorf("load %s: %w", envFile, err)
		}
	}
	return fromEnv(os.Getenv)
}

func fromEnv(getenv func(string) string) (Config, error) {
	cfg := Config{
		Token:        strings.TrimSpace(getenv("DISCORD_TOKEN")),
		DefaultGuild: strings.TrimSpace(getenv("DISCORD_GUILD_ID")),
		LogLevel:     getenv("LOG_LEVEL"),
		BaseURL:      getenv("DISCORD_API_BASE_URL"),
	}

	if cfg.Token == "" {
		if path := getenv("DISCORD_TOKEN_FILE"); path != "" {
			b, err := os.ReadFile(filepath.Clean(path))
			if err != nil {
				return cfg, fmt.Errorf("read DISCORD_TOKEN_FILE: %w", err)
			}
			cfg.Token = strings.TrimSpace(string(b))
		}
	}
	if cfg.Token == "" {
		return cfg, errors.New("DISCORD_TOKEN (or DISCORD_TOKEN_FILE) is required")
	}

	if v := getenv("DISCORD_READ_ONLY"); v != "" {
		ro, err := strconv.ParseBool(v)
		if err != nil {
			return cfg, fmt.Errorf("DISCORD_READ_ONLY: %w", err)
		}
		cfg.ReadOnly = ro
	}

	// Allowlist: explicit list, "*" for unrestricted, or default to the
	// default guild so a single-server setup is locked to that server.
	switch allowed := strings.TrimSpace(getenv("DISCORD_ALLOWED_GUILD_IDS")); allowed {
	case "*":
	case "":
		if cfg.DefaultGuild != "" {
			cfg.AllowedGuilds = []string{cfg.DefaultGuild}
		}
	default:
		for g := range strings.SplitSeq(allowed, ",") {
			if g = strings.TrimSpace(g); g != "" {
				cfg.AllowedGuilds = append(cfg.AllowedGuilds, g)
			}
		}
		if cfg.DefaultGuild == "" && len(cfg.AllowedGuilds) == 1 {
			cfg.DefaultGuild = cfg.AllowedGuilds[0]
		}
	}

	if cfg.DefaultGuild != "" && len(cfg.AllowedGuilds) > 0 {
		found := false
		for _, g := range cfg.AllowedGuilds {
			found = found || g == cfg.DefaultGuild
		}
		if !found {
			return cfg, fmt.Errorf("DISCORD_GUILD_ID %s is not in DISCORD_ALLOWED_GUILD_IDS", cfg.DefaultGuild)
		}
	}
	return cfg, nil
}
