package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/redpotatolabs/discord-mcp/internal/discord"
)

// Register adds every tool to the server. In readOnly mode only GET tools are
// exposed and the raw request tool is limited to GET.
func Register(s *server.MCPServer, d Deps, readOnly bool) int {
	n := 0
	for _, e := range Catalog {
		if readOnly && !e.ReadOnly() {
			continue
		}
		s.AddTool(e.Tool(d.DefaultGuild != ""), e.Handler(d))
		n++
	}

	ov := &overview{d: d}
	s.AddTool(ov.tool(d.DefaultGuild != ""), ov.handle)
	pt, ph := permissionsTool()
	s.AddTool(pt, ph)
	s.AddTool(rawTool(readOnly), rawHandler(d, readOnly))
	n += 3
	if !readOnly {
		s.AddTool(dmTool(d.DMPrefix), dmHandler(d))
		n++
	}
	return n
}

// ---- server_overview ----

type overview struct {
	d     Deps
	once  sync.Once
	botID string
}

func (o *overview) tool(hasDefault bool) mcp.Tool {
	gdesc := "Guild ID"
	opts := []mcp.PropertyOption{}
	if hasDefault {
		gdesc += " (defaults to the configured guild)"
	} else {
		opts = append(opts, mcp.Required())
	}
	return mcp.NewTool("server_overview",
		mcp.WithDescription("One-call snapshot of a guild: settings, the bot's role position, roles (top to bottom), and the channel tree grouped by category with permission overwrites. Start here before making structural changes."),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithOpenWorldHintAnnotation(true),
		mcp.WithString("guild_id", append(opts, mcp.Description(gdesc))...),
	)
}

type ovChannel struct {
	ID         string            `json:"id"`
	Name       string            `json:"name"`
	Type       int               `json:"type"`
	Position   int               `json:"position"`
	ParentID   string            `json:"parent_id,omitempty"`
	Topic      string            `json:"topic,omitempty"`
	NSFW       bool              `json:"nsfw,omitempty"`
	Slowmode   int               `json:"rate_limit_per_user,omitempty"`
	Overwrites []json.RawMessage `json:"permission_overwrites,omitempty"`
	Channels   []*ovChannel      `json:"channels,omitempty"`
}

type ovRole struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Position    int    `json:"position"`
	Permissions string `json:"permissions"`
	Color       int    `json:"color,omitempty"`
	Hoist       bool   `json:"hoist,omitempty"`
	Managed     bool   `json:"managed,omitempty"`
	Mentionable bool   `json:"mentionable,omitempty"`
	Admin       bool   `json:"administrator,omitempty"`
}

func (o *overview) handle(ctx context.Context, r mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	gid := r.GetString("guild_id", o.d.DefaultGuild)
	if !discord.IsSnowflake(gid) {
		return mcp.NewToolResultError("guild_id is required and must be a numeric ID"), nil
	}
	base := "/guilds/" + gid
	if err := o.d.Scope.Check(ctx, http.MethodGet, base); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	get := func(path string, q url.Values, v any) error {
		raw, err := o.d.Client.Do(ctx, discord.Request{Method: http.MethodGet, Path: path, Query: q})
		if err != nil {
			return err
		}
		return json.Unmarshal(raw, v)
	}

	var guild map[string]any
	if err := get(base, url.Values{"with_counts": {"true"}}, &guild); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	var roles []ovRole
	if err := get(base+"/roles", nil, &roles); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	var channels []*ovChannel
	if err := get(base+"/channels", nil, &channels); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Bot identity and highest role position (it can only manage roles below it).
	o.once.Do(func() {
		var me struct {
			ID string `json:"id"`
		}
		if err := get("/users/@me", nil, &me); err == nil {
			o.botID = me.ID
		}
	})
	bot := map[string]any{"id": o.botID}
	if o.botID != "" {
		var member struct {
			Roles []string `json:"roles"`
		}
		if err := get(base+"/members/"+o.botID, nil, &member); err == nil {
			pos := map[string]int{}
			for _, rl := range roles {
				pos[rl.ID] = rl.Position
			}
			highest := 0
			for _, id := range member.Roles {
				if pos[id] > highest {
					highest = pos[id]
				}
			}
			bot["highest_role_position"] = highest
			bot["role_ids"] = member.Roles
		}
	}

	for i := range roles {
		if names, err := BitsToPermissions(roles[i].Permissions); err == nil {
			for _, n := range names {
				if n == "ADMINISTRATOR" {
					roles[i].Admin = true
				}
			}
		}
	}
	sort.Slice(roles, func(i, j int) bool { return roles[i].Position > roles[j].Position })

	byPos := func(cs []*ovChannel) {
		sort.SliceStable(cs, func(i, j int) bool { return cs[i].Position < cs[j].Position })
	}
	cats := map[string]*ovChannel{}
	var top []*ovChannel
	for _, c := range channels {
		if c.Type == 4 {
			cats[c.ID] = c
			top = append(top, c)
		}
	}
	var uncategorized []*ovChannel
	for _, c := range channels {
		if c.Type == 4 {
			continue
		}
		if p, ok := cats[c.ParentID]; ok {
			p.Channels = append(p.Channels, c)
		} else {
			uncategorized = append(uncategorized, c)
		}
	}
	byPos(top)
	byPos(uncategorized)
	for _, c := range top {
		byPos(c.Channels)
	}

	keep := []string{"id", "name", "description", "owner_id", "approximate_member_count", "approximate_presence_count",
		"features", "verification_level", "default_message_notifications", "explicit_content_filter",
		"system_channel_id", "rules_channel_id", "public_updates_channel_id", "afk_channel_id", "afk_timeout",
		"preferred_locale", "premium_tier", "premium_subscription_count", "mfa_level", "vanity_url_code"}
	g := map[string]any{}
	for _, k := range keep {
		if v, ok := guild[k]; ok && v != nil {
			g[k] = v
		}
	}

	out := map[string]any{
		"guild":         g,
		"bot":           bot,
		"roles":         roles,
		"uncategorized": uncategorized,
		"categories":    top,
		"channel_types": "0 text, 2 voice, 4 category, 5 announcement, 13 stage, 15 forum, 16 media",
	}
	data, err := json.Marshal(out)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(string(data)), nil
}

// ---- discord_request (escape hatch) ----

var rawMethods = []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete}

func rawTool(readOnly bool) mcp.Tool {
	methods := rawMethods
	desc := "Call any Discord REST API v10 route directly, for things no dedicated tool covers (application commands, stage instances, polls, guild templates, ...). Path is relative to https://discord.com/api/v10, e.g. /guilds/123/templates. The guild allowlist still applies."
	if readOnly {
		methods = []string{http.MethodGet}
		desc += " Read-only mode: GET only."
	}
	return mcp.NewTool("discord_request",
		mcp.WithDescription(desc),
		mcp.WithDestructiveHintAnnotation(!readOnly),
		mcp.WithReadOnlyHintAnnotation(readOnly),
		mcp.WithOpenWorldHintAnnotation(true),
		mcp.WithString("method", mcp.Required(), mcp.Enum(methods...), mcp.Description("HTTP method")),
		mcp.WithString("path", mcp.Required(), mcp.Description("API path starting with /, without query string")),
		mcp.WithObject("query", mcp.Description("Query parameters"), mcp.AdditionalProperties(true)),
		mcp.WithObject("body", mcp.Description("JSON object body"), mcp.AdditionalProperties(true)),
		mcp.WithArray("body_list", mcp.Description("JSON array body (for bulk routes); used instead of body"), mcp.Items(map[string]any{})),
		mcp.WithString("reason", mcp.Description("Audit-log reason")),
	)
}

// ValidateRawPath rejects paths that could escape the intended route.
func ValidateRawPath(p string) error {
	if !strings.HasPrefix(p, "/") {
		return errors.New("path must start with /")
	}
	lower := strings.ToLower(p)
	for _, bad := range []string{"..", "//", "?", "#", "%2e", "%2f", "%5c", "\\"} {
		if strings.Contains(lower, bad) {
			return fmt.Errorf("path must not contain %q", bad)
		}
	}
	return nil
}

func rawHandler(d Deps, readOnly bool) server.ToolHandlerFunc {
	return func(ctx context.Context, r mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		method := strings.ToUpper(r.GetString("method", ""))
		if !contains(rawMethods, method) {
			return mcp.NewToolResultError("method must be one of " + strings.Join(rawMethods, ", ")), nil
		}
		if readOnly && method != http.MethodGet {
			return mcp.NewToolResultError("server is in read-only mode"), nil
		}
		path := r.GetString("path", "")
		if err := ValidateRawPath(path); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		args := r.GetArguments()
		req := discord.Request{Method: method, Path: path, Query: url.Values{}, Reason: r.GetString("reason", "")}
		if qm, ok := args["query"].(map[string]any); ok {
			for k, v := range qm {
				req.Query.Set(k, queryString(v))
			}
		}
		if bl, ok := args["body_list"].([]any); ok {
			req.Body = bl
		} else if bo, ok := args["body"].(map[string]any); ok {
			req.Body = bo
		}
		return execute(ctx, d, "discord_request", req, nil)
	}
}
