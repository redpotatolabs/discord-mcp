// Command discord-mcp is a stdio MCP server for managing Discord guilds
// through a bot account.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"github.com/mark3labs/mcp-go/server"

	"github.com/redpotatolabs/discord-mcp/internal/config"
	"github.com/redpotatolabs/discord-mcp/internal/discord"
	"github.com/redpotatolabs/discord-mcp/internal/tools"
	"github.com/redpotatolabs/discord-mcp/internal/watch"
)

var version = "dev"

const instructions = `Manage Discord servers (guilds) through a bot account.

- Call server_overview first: it returns roles, the channel tree and the bot's highest role position.
- The bot can only manage roles and members whose highest role is below the bot's highest role.
- IDs are strings. Permission fields are decimal bitfield strings; convert names with the permissions tool.
- Destructive actions (delete, ban, kick, prune, bulk delete) cannot be undone. Confirm intent before calling them, and pass a reason so the audit log explains the change.
- discord_request reaches any REST route not covered by a dedicated tool.`

func main() {
	if len(os.Args) > 1 && (os.Args[1] == "--version" || os.Args[1] == "-v") {
		fmt.Println(version)
		return
	}
	if err := run(); err != nil {
		log.Fatalf("discord-mcp: %v", err)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	setupLogging(cfg.LogLevel)

	client, err := discord.New(cfg.Token, cfg.BaseURL)
	if err != nil {
		return err
	}
	// Fail fast on a bad token rather than on the first tool call.
	me, err := client.Do(context.Background(), discord.Request{Method: http.MethodGet, Path: "/users/@me"})
	if err != nil {
		return fmt.Errorf("verify bot token: %w", err)
	}
	var bot struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(me, &bot); err != nil || bot.ID == "" {
		return fmt.Errorf("verify bot token: unexpected /users/@me response")
	}

	deps := tools.Deps{
		Client:       client,
		Scope:        discord.NewScope(client, cfg.AllowedGuilds),
		DefaultGuild: cfg.DefaultGuild,
		DMPrefix:     cfg.DMPrefix,
	}

	opts := []server.ServerOption{server.WithToolCapabilities(false), server.WithRecovery()}
	if len(cfg.WatchChannels) > 0 {
		// Watched channels must pass the same guild allowlist as every tool.
		for _, id := range cfg.WatchChannels {
			if err := deps.Scope.Check(context.Background(), http.MethodGet, "/channels/"+id); err != nil {
				return fmt.Errorf("DISCORD_WATCH_CHANNEL_IDS %s: %w", id, err)
			}
		}
		opts = append(opts,
			server.WithExperimental(map[string]any{watch.Capability: map[string]any{}}),
			server.WithInstructions(instructions+watch.Instructions))
	} else {
		opts = append(opts, server.WithInstructions(instructions))
	}
	s := server.NewMCPServer("discord-mcp", version, opts...)
	n := tools.Register(s, deps, cfg.ReadOnly)

	if len(cfg.WatchChannels) > 0 {
		w := watch.New(bot.ID, cfg.WatchChannels, cfg.TrustedUsers, s.SendNotificationToAllClients)
		// A gateway failure disables watching but keeps the tools available.
		if gw, err := w.Open(cfg.Token); err != nil {
			slog.Error("channel watching disabled", "err", err)
		} else {
			defer func() { _ = gw.Close() }()
			slog.Info("watching channels", "channels", strings.Join(cfg.WatchChannels, ","), "trusted", strings.Join(cfg.TrustedUsers, ","))
		}
	}

	scope := "all guilds"
	if len(cfg.AllowedGuilds) > 0 {
		scope = strings.Join(cfg.AllowedGuilds, ",")
	}
	slog.Info("discord-mcp ready", "version", version, "tools", n, "read_only", cfg.ReadOnly,
		"default_guild", cfg.DefaultGuild, "allowed_guilds", scope)

	return server.ServeStdio(s)
}

// setupLogging sends logs to stderr: stdout carries the MCP protocol.
func setupLogging(level string) {
	var l slog.Level
	switch strings.ToLower(level) {
	case "debug":
		l = slog.LevelDebug
	case "warn":
		l = slog.LevelWarn
	case "error":
		l = slog.LevelError
	default:
		l = slog.LevelInfo
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: l})))
}
