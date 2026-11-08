// Package tools defines the MCP tools. Most are declarative Endpoint specs
// mapped 1:1 onto Discord REST routes; a few (overview, permissions, raw
// request) are hand-written.
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"maps"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/epistemi/discord-mcp/internal/discord"
)

// Kind is a parameter's JSON type (plus "image", a string converted to a data URI).
type Kind string

// Parameter kinds.
const (
	String  Kind = "string"
	Int     Kind = "integer"
	Number  Kind = "number"
	Bool    Kind = "boolean"
	Strings Kind = "string[]"
	Objects Kind = "object[]"
	Object  Kind = "object"
	Image   Kind = "image" // data URI, http(s) URL or local file path → data URI
)

// In says where a parameter goes.
type In int

// Parameter locations.
const (
	Body In = iota
	Query
	RawBody // the parameter's value is the entire request body
)

// Param is one tool argument.
type Param struct {
	Name     string
	Kind     Kind
	Desc     string
	Required bool
	In       In
	Enum     []string
}

// Endpoint is a tool backed by a single Discord REST route.
type Endpoint struct {
	Name   string
	Desc   string
	Method string
	// Path with {placeholders}; each becomes a required string argument.
	// {guild_id} is optional when a default guild is configured.
	Path   string
	Params []Param
	// PathDesc overrides the default description of path placeholders.
	PathDesc map[string]string
	// Extra adds an "extra" object merged into the JSON body for fields not
	// modelled explicitly.
	Extra       bool
	Destructive bool
	// Defaults are body fields sent unless the caller sets them.
	Defaults map[string]any
	// Shape, if set, compacts the response by default; callers can pass
	// full=true to get the raw Discord payload.
	Shape func(json.RawMessage) json.RawMessage
}

// ReadOnly reports whether the endpoint never mutates state.
func (e Endpoint) ReadOnly() bool { return e.Method == http.MethodGet }

// Deps is what handlers need at runtime.
type Deps struct {
	Client       *discord.Client
	Scope        *discord.Scope
	DefaultGuild string
}

var placeholderRe = regexp.MustCompile(`\{([a-z_]+)\}`)

func (e Endpoint) placeholders() []string {
	var out []string
	for _, m := range placeholderRe.FindAllStringSubmatch(e.Path, -1) {
		out = append(out, m[1])
	}
	return out
}

var defaultPathDesc = map[string]string{
	"guild_id":       "Guild (server) ID",
	"channel_id":     "Channel or thread ID",
	"message_id":     "Message ID",
	"role_id":        "Role ID",
	"user_id":        "User ID",
	"emoji_id":       "Custom emoji ID",
	"sticker_id":     "Sticker ID",
	"sound_id":       "Soundboard sound ID",
	"webhook_id":     "Webhook ID",
	"event_id":       "Scheduled event ID",
	"rule_id":        "Auto-moderation rule ID",
	"integration_id": "Integration ID",
	"overwrite_id":   "Role or user ID the permission overwrite applies to",
	"command_id":     "Application command ID",
	"application_id": "Application ID (the bot's user ID)",
	"code":           "Invite code (the part after discord.gg/)",
	"emoji":          "Unicode emoji (e.g. 👍) or custom emoji as name:id",
}

// Tool builds the MCP tool definition.
func (e Endpoint) Tool(hasDefaultGuild bool) mcp.Tool {
	opts := []mcp.ToolOption{
		mcp.WithDescription(e.Desc),
		mcp.WithReadOnlyHintAnnotation(e.ReadOnly()),
		mcp.WithDestructiveHintAnnotation(e.Destructive),
		mcp.WithIdempotentHintAnnotation(e.Method != http.MethodPost),
		mcp.WithOpenWorldHintAnnotation(true),
	}
	for _, ph := range e.placeholders() {
		desc := defaultPathDesc[ph]
		if d, ok := e.PathDesc[ph]; ok {
			desc = d
		}
		popts := []mcp.PropertyOption{mcp.Description(desc)}
		if ph == "guild_id" && hasDefaultGuild {
			popts[0] = mcp.Description(desc + " (defaults to the configured guild)")
		} else {
			popts = append(popts, mcp.Required())
		}
		opts = append(opts, mcp.WithString(ph, popts...))
	}
	for _, p := range e.Params {
		opts = append(opts, p.option())
	}
	if e.Extra {
		opts = append(opts, mcp.WithObject("extra",
			mcp.Description("Additional raw JSON body fields using Discord API field names (merged under explicit arguments). Use null to clear a field."),
			mcp.AdditionalProperties(true)))
	}
	if !e.ReadOnly() {
		opts = append(opts, mcp.WithString("reason", mcp.Description("Audit-log reason shown in the server's audit log")))
	}
	if e.Shape != nil {
		opts = append(opts, mcp.WithBoolean("full", mcp.Description("Return raw Discord objects instead of the compact form (which drops cosmetic profile data)")))
	}
	return mcp.NewTool(e.Name, opts...)
}

func (p Param) option() mcp.ToolOption {
	popts := []mcp.PropertyOption{mcp.Description(p.Desc)}
	if p.Required {
		popts = append(popts, mcp.Required())
	}
	if len(p.Enum) > 0 {
		popts = append(popts, mcp.Enum(p.Enum...))
	}
	switch p.Kind {
	case Int, Number:
		return mcp.WithNumber(p.Name, popts...)
	case Bool:
		return mcp.WithBoolean(p.Name, popts...)
	case Strings:
		return mcp.WithArray(p.Name, append(popts, mcp.WithStringItems())...)
	case Objects:
		return mcp.WithArray(p.Name, append(popts, mcp.Items(map[string]any{"type": "object"}))...)
	case Object:
		return mcp.WithObject(p.Name, append(popts, mcp.AdditionalProperties(true))...)
	case Image:
		popts[0] = mcp.Description(p.Desc + ". Accepts a local file path, an http(s) URL, or a data: URI.")
		return mcp.WithString(p.Name, popts...)
	default:
		return mcp.WithString(p.Name, popts...)
	}
}

// Build turns tool arguments into a REST request (without executing it).
func (e Endpoint) Build(ctx context.Context, args map[string]any, defaultGuild string) (discord.Request, error) {
	req := discord.Request{Method: e.Method, Query: url.Values{}}

	path := e.Path
	for _, ph := range e.placeholders() {
		v, _ := args[ph].(string)
		v = strings.TrimSpace(v)
		if v == "" && ph == "guild_id" {
			v = defaultGuild
		}
		if v == "" {
			return req, fmt.Errorf("%s is required", ph)
		}
		if err := validatePathValue(ph, v); err != nil {
			return req, err
		}
		path = strings.Replace(path, "{"+ph+"}", url.PathEscape(v), 1)
	}
	req.Path = path

	body := map[string]any{}
	maps.Copy(body, e.Defaults)
	if extra, ok := args["extra"].(map[string]any); ok && e.Extra {
		maps.Copy(body, extra)
	}
	var raw any
	for _, p := range e.Params {
		v, present := args[p.Name]
		if !present || v == nil {
			if p.Required {
				return req, fmt.Errorf("%s is required", p.Name)
			}
			continue
		}
		v, err := coerce(ctx, p, v)
		if err != nil {
			return req, err
		}
		switch p.In {
		case Query:
			req.Query.Set(p.Name, queryString(v))
		case RawBody:
			raw = v
		default:
			body[p.Name] = v
		}
	}

	switch {
	case raw != nil:
		req.Body = raw
	case len(body) > 0:
		req.Body = body
	case e.Method == http.MethodPost || e.Method == http.MethodPatch:
		// Discord rejects PATCH/POST JSON routes without a body on some routes.
		req.Body = map[string]any{}
	}

	if r, _ := args["reason"].(string); r != "" && !e.ReadOnly() {
		req.Reason = r
	}
	return req, nil
}

var codeRe = regexp.MustCompile(`^[A-Za-z0-9-]{1,64}$`)

func validatePathValue(name, v string) error {
	switch {
	case name == "emoji":
		if strings.Contains(v, "/") || strings.Contains(v, "..") {
			return fmt.Errorf("invalid emoji %q", v)
		}
		return nil
	case name == "code":
		if !codeRe.MatchString(v) {
			return fmt.Errorf("invalid invite code %q", v)
		}
		return nil
	case name == "visibility":
		if v != "public" && v != "private" {
			return fmt.Errorf("visibility must be public or private, got %q", v)
		}
		return nil
	case name == "user_id" && v == "@me":
		return nil
	case strings.HasSuffix(name, "_id"):
		if !discord.IsSnowflake(v) {
			return fmt.Errorf("%s must be a numeric Discord ID, got %q", name, v)
		}
		return nil
	}
	if strings.ContainsAny(v, "/?#") || strings.Contains(v, "..") {
		return fmt.Errorf("invalid %s %q", name, v)
	}
	return nil
}

func coerce(ctx context.Context, p Param, v any) (any, error) {
	switch p.Kind {
	case Int:
		f, ok := v.(float64)
		if !ok {
			if s, isStr := v.(string); isStr {
				n, err := strconv.ParseInt(s, 10, 64)
				if err != nil {
					return nil, fmt.Errorf("%s must be an integer", p.Name)
				}
				return n, nil
			}
			return nil, fmt.Errorf("%s must be an integer", p.Name)
		}
		return int64(f), nil
	case Number:
		if _, ok := v.(float64); !ok {
			return nil, fmt.Errorf("%s must be a number", p.Name)
		}
	case Bool:
		if _, ok := v.(bool); !ok {
			return nil, fmt.Errorf("%s must be a boolean", p.Name)
		}
	case String:
		if _, ok := v.(string); !ok {
			return nil, fmt.Errorf("%s must be a string", p.Name)
		}
		if len(p.Enum) > 0 && !contains(p.Enum, v.(string)) {
			return nil, fmt.Errorf("%s must be one of %v", p.Name, p.Enum)
		}
	case Image:
		s, ok := v.(string)
		if !ok {
			return nil, fmt.Errorf("%s must be a string", p.Name)
		}
		return toDataURI(ctx, s)
	case Strings, Objects:
		if _, ok := v.([]any); !ok {
			return nil, fmt.Errorf("%s must be an array", p.Name)
		}
	case Object:
		if _, ok := v.(map[string]any); !ok {
			return nil, fmt.Errorf("%s must be an object", p.Name)
		}
	}
	return v, nil
}

func contains(xs []string, s string) bool { return slices.Contains(xs, s) }

func queryString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case bool:
		return strconv.FormatBool(t)
	case int64:
		return strconv.FormatInt(t, 10)
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case []any:
		parts := make([]string, 0, len(t))
		for _, x := range t {
			parts = append(parts, fmt.Sprint(x))
		}
		return strings.Join(parts, ",")
	default:
		return fmt.Sprint(t)
	}
}

// maxResult caps tool output so one huge listing can't flood the context.
const maxResult = 200_000

// execute runs a request through the scope guard and formats the result,
// applying shape (if non-nil) to successful responses.
func execute(ctx context.Context, d Deps, tool string, req discord.Request, shape func(json.RawMessage) json.RawMessage) (*mcp.CallToolResult, error) {
	if err := d.Scope.Check(ctx, req.Method, req.Path); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	resp, err := d.Client.Do(ctx, req)
	if req.Method != http.MethodGet {
		status := "success"
		if err != nil {
			status = "error"
		}
		slog.Info("audit", "tool", tool, "method", req.Method, "path", req.Path, "status", status)
	}
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	if shape != nil && len(resp) > 0 {
		resp = shape(resp)
	}
	return mcp.NewToolResultText(formatResult(resp)), nil
}

func formatResult(resp json.RawMessage) string {
	if len(resp) == 0 {
		return `{"ok":true}`
	}
	if len(resp) > maxResult {
		return string(resp[:maxResult]) + fmt.Sprintf("\n…[truncated: %d of %d bytes shown; narrow the query with limit/before/after]", maxResult, len(resp))
	}
	return string(resp)
}

// Handler returns the MCP handler for an endpoint.
func (e Endpoint) Handler(d Deps) server.ToolHandlerFunc {
	return func(ctx context.Context, r mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		req, err := e.Build(ctx, r.GetArguments(), d.DefaultGuild)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		shape := e.Shape
		if r.GetBool("full", false) {
			shape = nil
		}
		return execute(ctx, d, e.Name, req, shape)
	}
}
