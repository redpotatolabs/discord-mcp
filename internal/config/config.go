// Package config loads settings from the environment, optionally seeded from
// a .env file next to the binary or in $XDG_CONFIG_HOME/discord-mcp/.
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
	// DMPrefix is an opening line send_dm puts before every DM; empty = none.
	DMPrefix string
}

// EnvFileCandidates returns the env files tried, in order, when
// DISCORD_MCP_ENV_FILE is unset: .env next to the (symlink-resolved)
// executable — i.e. the repo when running the repo-built binary — then
// $XDG_CONFIG_HOME/discord-mcp/.env.
func EnvFileCandidates() []string {
	var out []string
	if exe, err := os.Executable(); err == nil {
		if resolved, err := filepath.EvalSymlinks(exe); err == nil {
			exe = resolved
		}
		out = append(out, filepath.Join(filepath.Dir(exe), ".env"))
	}
	if dir, err := os.UserConfigDir(); err == nil {
		out = append(out, filepath.Join(dir, "discord-mcp", ".env"))
	}
	return out
}

// Load reads configuration. Real environment variables win over the env file.
// Only the first env file found is loaded.
func Load() (Config, error) {
	candidates := EnvFileCandidates()
	if f := os.Getenv("DISCORD_MCP_ENV_FILE"); f != "" {
		candidates = []string{f}
	}
	for _, f := range candidates {
		err := godotenv.Load(f)
		if err == nil {
			break
		}
		if !errors.Is(err, os.ErrNotExist) {
			return Config{}, fmt.Errorf("load %s: %w", f, err)
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
		DMPrefix:     strings.TrimRight(getenv("DISCORD_DM_PREFIX"), "\r\n"),
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
