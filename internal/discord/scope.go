package discord

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
)

// Scope restricts requests to a set of guilds. Paths are classified by their
// first segment; channel, webhook and invite IDs are resolved to their owning
// guild (and cached) before the call is allowed.
type Scope struct {
	client  *Client
	allowed map[string]bool // nil = every guild the bot is in

	mu    sync.Mutex
	owner map[string]string // "channels/123" -> guild ID ("" = not in a guild)
}

// NewScope returns a Scope; an empty allowlist permits everything.
func NewScope(c *Client, allowed []string) *Scope {
	s := &Scope{client: c, owner: map[string]string{}}
	if len(allowed) > 0 {
		s.allowed = make(map[string]bool, len(allowed))
		for _, g := range allowed {
			s.allowed[g] = true
		}
	}
	return s
}

// Unrestricted reports whether every guild is allowed.
func (s *Scope) Unrestricted() bool { return s.allowed == nil }

// GuildAllowed reports whether a guild ID is in scope.
func (s *Scope) GuildAllowed(id string) bool { return s.allowed == nil || s.allowed[id] }

// Check returns an error if the request would touch something outside scope.
func (s *Scope) Check(ctx context.Context, method, path string) error {
	if s.allowed == nil {
		return nil
	}
	p, _, _ := strings.Cut(path, "?")
	segs := strings.Split(strings.Trim(p, "/"), "/")

	switch segs[0] {
	case "guilds":
		if len(segs) < 2 || segs[1] == "" {
			return fmt.Errorf("%s %s is not allowed while a guild allowlist is configured", method, p)
		}
		return s.guild(segs[1])
	case "channels":
		return s.resolve(ctx, segs, "channels", func(raw json.RawMessage) (string, error) {
			var ch struct {
				GuildID string `json:"guild_id"`
			}
			err := json.Unmarshal(raw, &ch)
			return ch.GuildID, err
		})
	case "webhooks":
		return s.resolve(ctx, segs, "webhooks", func(raw json.RawMessage) (string, error) {
			var wh struct {
				GuildID string `json:"guild_id"`
			}
			err := json.Unmarshal(raw, &wh)
			return wh.GuildID, err
		})
	case "invites":
		return s.resolve(ctx, segs, "invites", func(raw json.RawMessage) (string, error) {
			var inv struct {
				Guild struct {
					ID string `json:"id"`
				} `json:"guild"`
			}
			err := json.Unmarshal(raw, &inv)
			return inv.Guild.ID, err
		})
	case "users":
		// /users/@me/guilds/{guild_id}[/member] targets a specific guild.
		if len(segs) >= 4 && segs[1] == "@me" && segs[2] == "guilds" {
			return s.guild(segs[3])
		}
	}

	// Anything else (users, applications, voice regions, sticker packs, ...)
	// is not guild-owned: allow reads only.
	if method != http.MethodGet {
		return fmt.Errorf("%s %s is not guild-scoped and is blocked while a guild allowlist is configured", method, p)
	}
	return nil
}

func (s *Scope) guild(id string) error {
	if s.allowed[id] {
		return nil
	}
	return fmt.Errorf("guild %s is not in DISCORD_ALLOWED_GUILD_IDS", id)
}

func (s *Scope) resolve(ctx context.Context, segs []string, kind string, guildOf func(json.RawMessage) (string, error)) error {
	if len(segs) < 2 || segs[1] == "" {
		return fmt.Errorf("/%s requires an ID while a guild allowlist is configured", kind)
	}
	key := kind + "/" + segs[1]

	s.mu.Lock()
	gid, ok := s.owner[key]
	s.mu.Unlock()

	if !ok {
		// segs[1] is already path-escaped (it came from a built path).
		raw, err := s.client.Do(ctx, Request{Method: http.MethodGet, Path: "/" + kind + "/" + segs[1]})
		if err != nil {
			return fmt.Errorf("resolve owning guild of %s: %w", key, err)
		}
		gid, err = guildOf(raw)
		if err != nil {
			return fmt.Errorf("resolve owning guild of %s: %w", key, err)
		}
		s.mu.Lock()
		s.owner[key] = gid
		s.mu.Unlock()
	}
	if gid == "" {
		return fmt.Errorf("%s does not belong to a guild (DMs are blocked while a guild allowlist is configured)", key)
	}
	return s.guild(gid)
}
