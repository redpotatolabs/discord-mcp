package tools

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/epistemi/discord-mcp/internal/discord"
)

const (
	guild     = "111111111111111111"
	channel   = "333333333333333333"
	msg       = "666666666666666666"
	user      = "777777777777777777"
	stranger  = "888888888888888888"
	dmChannel = "999999999999999999"
)

func endpoint(t *testing.T, name string) Endpoint {
	t.Helper()
	for _, e := range Catalog {
		if e.Name == name {
			return e
		}
	}
	t.Fatalf("no endpoint %s", name)
	return Endpoint{}
}

func TestCatalogSanity(t *testing.T) {
	seen := map[string]bool{"server_overview": true, "permissions": true, "discord_request": true, "send_dm": true}
	for _, e := range Catalog {
		if seen[e.Name] {
			t.Errorf("duplicate tool name %s", e.Name)
		}
		seen[e.Name] = true
		if e.Desc == "" {
			t.Errorf("%s: missing description", e.Name)
		}
		if !strings.HasPrefix(e.Path, "/") {
			t.Errorf("%s: bad path %s", e.Name, e.Path)
		}
		if !contains(rawMethods, e.Method) {
			t.Errorf("%s: bad method %s", e.Name, e.Method)
		}
		if e.Destructive && e.ReadOnly() {
			t.Errorf("%s: GET marked destructive", e.Name)
		}
		for _, ph := range e.placeholders() {
			if defaultPathDesc[ph] == "" && e.PathDesc[ph] == "" {
				t.Errorf("%s: placeholder %s has no description", e.Name, ph)
			}
		}
		params := map[string]bool{}
		for _, p := range e.Params {
			if params[p.Name] {
				t.Errorf("%s: duplicate param %s", e.Name, p.Name)
			}
			params[p.Name] = true
		}
		// Schema must serialize (catches bad property options).
		if _, err := json.Marshal(e.Tool(true)); err != nil {
			t.Errorf("%s: %v", e.Name, err)
		}
	}
}

func TestBuildPathDefaultGuildAndValidation(t *testing.T) {
	e := endpoint(t, "add_member_role")
	ctx := context.Background()

	req, err := e.Build(ctx, map[string]any{"user_id": user, "role_id": "888888888888888888", "reason": "promo"}, guild)
	if err != nil {
		t.Fatal(err)
	}
	if req.Path != "/guilds/"+guild+"/members/"+user+"/roles/888888888888888888" || req.Reason != "promo" || req.Body != nil {
		t.Errorf("req = %+v", req)
	}

	if _, err := e.Build(ctx, map[string]any{"user_id": user, "role_id": "1"}, guild); err == nil {
		t.Error("expected invalid role_id error")
	}
	if _, err := e.Build(ctx, map[string]any{"user_id": "../x", "role_id": "888888888888888888"}, guild); err == nil {
		t.Error("expected traversal rejection")
	}
	if _, err := e.Build(ctx, map[string]any{"user_id": user, "role_id": "888888888888888888"}, ""); err == nil {
		t.Error("expected guild_id required")
	}
}

func TestBuildBodyExtraDefaultsQuery(t *testing.T) {
	ctx := context.Background()

	ev := endpoint(t, "create_scheduled_event")
	req, err := ev.Build(ctx, map[string]any{"name": "Game night", "entity_type": float64(3), "extra": map[string]any{"name": "overridden", "x": nil}}, guild)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]any{"name": "Game night", "entity_type": int64(3), "privacy_level": 2, "x": nil}
	if !reflect.DeepEqual(req.Body, want) {
		t.Errorf("body = %#v", req.Body)
	}

	lm := endpoint(t, "list_messages")
	req, err = lm.Build(ctx, map[string]any{"channel_id": channel, "limit": float64(50), "before": msg}, "")
	if err != nil {
		t.Fatal(err)
	}
	if req.Query.Encode() != "before="+msg+"&limit=50" || req.Body != nil {
		t.Errorf("query = %s body=%v", req.Query.Encode(), req.Body)
	}

	ro := endpoint(t, "reorder_roles")
	positions := []any{map[string]any{"id": "1", "position": float64(2)}}
	req, err = ro.Build(ctx, map[string]any{"positions": positions}, guild)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(req.Body, positions) {
		t.Errorf("raw body = %#v", req.Body)
	}

	cc := endpoint(t, "create_channel")
	if _, err := cc.Build(ctx, map[string]any{"type": float64(0)}, guild); err == nil {
		t.Error("expected name required")
	}
	if _, err := cc.Build(ctx, map[string]any{"name": "x", "nsfw": "yes"}, guild); err == nil {
		t.Error("expected bool type error")
	}

	// Emoji path values are escaped.
	ar := endpoint(t, "add_reaction")
	req, err = ar.Build(ctx, map[string]any{"channel_id": channel, "message_id": msg, "emoji": "👍"}, "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(req.Path, "/reactions/%F0%9F%91%8D/@me") {
		t.Errorf("path = %s", req.Path)
	}
}

func TestImageParam(t *testing.T) {
	png := []byte("\x89PNG\r\n\x1a\n0000")
	p := filepath.Join(t.TempDir(), "icon.png")
	if err := os.WriteFile(p, png, 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := toDataURI(context.Background(), p)
	if err != nil || !strings.HasPrefix(got, "data:image/png;base64,") {
		t.Fatalf("got %q err %v", got, err)
	}
	txt := filepath.Join(t.TempDir(), "a.txt")
	_ = os.WriteFile(txt, []byte("hello"), 0o600)
	if _, err := toDataURI(context.Background(), txt); err == nil {
		t.Error("expected non-image rejection")
	}
	if v, _ := toDataURI(context.Background(), "data:image/gif;base64,AA=="); v != "data:image/gif;base64,AA==" {
		t.Error("data URI should pass through")
	}
}

func TestPermissions(t *testing.T) {
	bits, err := PermissionsToBits([]string{"view_channel", "SEND_MESSAGES"})
	if err != nil || bits != "3072" {
		t.Fatalf("bits=%s err=%v", bits, err)
	}
	names, err := BitsToPermissions("3072")
	if err != nil || !reflect.DeepEqual(names, []string{"VIEW_CHANNEL", "SEND_MESSAGES"}) {
		t.Fatalf("names=%v err=%v", names, err)
	}
	if _, err := PermissionsToBits([]string{"FLY"}); err == nil {
		t.Error("expected unknown permission")
	}
	names, _ = BitsToPermissions("9223372036854775808") // bit 63
	if !reflect.DeepEqual(names, []string{"UNKNOWN_BIT_63"}) {
		t.Errorf("names = %v", names)
	}
	// Default @everyone bitfield from a real 2026 guild decodes fully.
	names, _ = BitsToPermissions("2248473465835073")
	for _, n := range names {
		if strings.HasPrefix(n, "UNKNOWN") {
			t.Errorf("unexpected %s in default @everyone permissions", n)
		}
	}
}

func TestCompactMessages(t *testing.T) {
	raw := json.RawMessage(`[
	 {"id":"2","type":19,"content":"nice","timestamp":"t2","edited_timestamp":null,
	  "author":{"id":"u1","username":"red_queen_","global_name":"RedQueen","collectibles":{"nameplate":{"asset":"x"}},"clan":{"tag":"148"}},
	  "mentions":[{"id":"u2","username":"existemi","global_name":"Potato","avatar_decoration_data":{"asset":"y"}}],
	  "attachments":[{"id":"a","filename":"logo.png","url":"https://cdn/x.png","content_type":"image/png","size":10,"width":5,"height":5,"placeholder":"zzz"}],
	  "embeds":[],"components":[],"sticker_items":[{"id":"s","name":"Wave","format_type":3}],
	  "reactions":[{"count":2,"emoji":{"id":null,"name":"🔥"}}],
	  "message_reference":{"message_id":"1","channel_id":"c"},
	  "referenced_message":{"id":"1","content":"make a logo","author":{"id":"u2","username":"existemi"}}},
	 {"id":"3","type":7,"content":"","timestamp":"t3","author":{"id":"b","username":"Roy's MCP","bot":true},
	  "actor":{"id":"u2","username":"existemi","global_name":"Potato"}}
	]`)
	out := string(compactMessages(raw))
	for _, gone := range []string{"collectibles", "clan", "avatar_decoration", "placeholder", "nameplate"} {
		if strings.Contains(out, gone) {
			t.Errorf("compact output still contains %q: %s", gone, out)
		}
	}
	for _, kept := range []string{`"content":"nice"`, `"display_name":"RedQueen"`, `"filename":"logo.png"`, `"url":"https://cdn/x.png"`,
		`"stickers":["Wave"]`, `"🔥":2`, `"reply_to":{"message_id":"1","author":"existemi","content":"make a logo"}`,
		`"type_name":"member_join"`, `"actor":{"id":"u2"`, `"bot":true`} {
		if !strings.Contains(out, kept) {
			t.Errorf("compact output missing %s: %s", kept, out)
		}
	}
	if len(out) >= len(raw) {
		t.Errorf("compact output not smaller: %d >= %d", len(out), len(raw))
	}

	// Single object and garbage pass-through.
	if got := string(compactMessages(json.RawMessage(`{"id":"9","content":"x","timestamp":"t"}`))); got != `{"id":"9","timestamp":"t","content":"x"}` {
		t.Errorf("single = %s", got)
	}
	if got := string(compactMessages(json.RawMessage(`{"weird":true}`))); got != `{"weird":true}` {
		t.Errorf("non-message should pass through, got %s", got)
	}
}

func TestValidateRawPath(t *testing.T) {
	for _, p := range []string{"guilds/1", "/guilds/1/../2", "/guilds//1", "/guilds/1?x=1", "/guilds/%2e%2e/2", "/a\\b"} {
		if ValidateRawPath(p) == nil {
			t.Errorf("%q should be rejected", p)
		}
	}
	if err := ValidateRawPath("/guilds/1/templates"); err != nil {
		t.Error(err)
	}
}

// ---- end to end through the MCP server ----

type hit struct{ method, path, body string }

func newTestServer(t *testing.T, readOnly bool, allowed []string, opts ...func(*Deps)) (*server.MCPServer, *[]hit) {
	t.Helper()
	var hits []hit
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		hits = append(hits, hit{r.Method, r.URL.Path, string(b)})
		switch {
		case r.URL.Path == "/channels/"+channel:
			_, _ = w.Write([]byte(`{"id":"` + channel + `","guild_id":"` + guild + `"}`))
		case r.URL.Path == "/guilds/"+guild:
			_, _ = w.Write([]byte(`{"id":"` + guild + `","name":"Test","owner_id":"1","emojis":[1,2,3]}`))
		case r.URL.Path == "/guilds/"+guild+"/roles":
			_, _ = w.Write([]byte(`[{"id":"` + guild + `","name":"@everyone","position":0,"permissions":"0"},{"id":"9","name":"Admin","position":2,"permissions":"8"},{"id":"10","name":"Bot","position":3,"permissions":"0","managed":true}]`))
		case r.URL.Path == "/guilds/"+guild+"/channels":
			_, _ = w.Write([]byte(`[{"id":"c1","name":"General","type":4,"position":0},{"id":"t2","name":"chat","type":0,"position":1,"parent_id":"c1"},{"id":"t1","name":"rules","type":0,"position":0,"parent_id":"c1"},{"id":"x","name":"loose","type":0,"position":0}]`))
		case r.URL.Path == "/users/@me":
			_, _ = w.Write([]byte(`{"id":"` + user + `"}`))
		case r.URL.Path == "/guilds/"+guild+"/members/"+user:
			_, _ = w.Write([]byte(`{"roles":["10"]}`))
		case r.URL.Path == "/guilds/"+guild+"/members/"+stranger:
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"message":"Unknown Member","code":10007}`))
		case r.Method == http.MethodPost && r.URL.Path == "/users/@me/channels":
			_, _ = w.Write([]byte(`{"id":"` + dmChannel + `","type":1}`))
		case r.Method == http.MethodPost && r.URL.Path == "/channels/"+dmChannel+"/messages":
			if strings.Contains(string(b), "blocked") {
				w.WriteHeader(http.StatusForbidden)
				_, _ = w.Write([]byte(`{"message":"Cannot send messages to this user","code":50007}`))
				return
			}
			_, _ = w.Write([]byte(`{"id":"dm1","content":"hi","timestamp":"t","author":{"id":"b","username":"bot","bot":true,"collectibles":{}}}`))
		case r.Method == http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		default:
			_, _ = w.Write([]byte(`{"id":"new"}`))
		}
	}))
	t.Cleanup(api.Close)

	c, err := discord.New("tok", api.URL)
	if err != nil {
		t.Fatal(err)
	}
	s := server.NewMCPServer("test", "0")
	d := Deps{Client: c, Scope: discord.NewScope(c, allowed), DefaultGuild: guild}
	for _, o := range opts {
		o(&d)
	}
	Register(s, d, readOnly)
	return s, &hits
}

func call(t *testing.T, s *server.MCPServer, name string, args map[string]any) (string, bool) {
	t.Helper()
	tool := s.GetTool(name)
	if tool == nil {
		return "", false
	}
	var req mcp.CallToolRequest
	req.Params.Name = name
	req.Params.Arguments = args
	res, err := tool.Handler(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	text := res.Content[0].(mcp.TextContent).Text
	if res.IsError {
		return "ERR: " + text, true
	}
	return text, true
}

func TestEndToEnd(t *testing.T) {
	s, hits := newTestServer(t, false, []string{guild})

	out, _ := call(t, s, "send_message", map[string]any{"channel_id": channel, "content": "hi"})
	if out != `{"id":"new"}` {
		t.Fatalf("send_message = %s", out)
	}
	last := (*hits)[len(*hits)-1]
	if last.method != "POST" || last.path != "/channels/"+channel+"/messages" || last.body != `{"content":"hi"}` {
		t.Errorf("last hit = %+v", last)
	}

	out, _ = call(t, s, "delete_role", map[string]any{"role_id": "888888888888888888"})
	if out != `{"ok":true}` {
		t.Errorf("delete_role = %s", out)
	}

	out, _ = call(t, s, "get_guild", map[string]any{"guild_id": "222222222222222222"})
	if !strings.HasPrefix(out, "ERR:") {
		t.Errorf("out-of-scope guild should fail: %s", out)
	}

	out, _ = call(t, s, "discord_request", map[string]any{"method": "GET", "path": "/guilds/" + guild + "/templates"})
	if strings.HasPrefix(out, "ERR:") {
		t.Errorf("raw request failed: %s", out)
	}

	out, _ = call(t, s, "server_overview", map[string]any{})
	var ov struct {
		Guild map[string]any   `json:"guild"`
		Bot   map[string]any   `json:"bot"`
		Roles []map[string]any `json:"roles"`
		Cats  []struct {
			Channels []struct {
				Name string `json:"name"`
			} `json:"channels"`
		} `json:"categories"`
		Uncat []any `json:"uncategorized"`
	}
	if err := json.Unmarshal([]byte(out), &ov); err != nil {
		t.Fatalf("overview: %v: %s", err, out)
	}
	if ov.Guild["name"] != "Test" || ov.Guild["emojis"] != nil {
		t.Errorf("guild = %v", ov.Guild)
	}
	if ov.Bot["highest_role_position"] != float64(3) {
		t.Errorf("bot = %v", ov.Bot)
	}
	if ov.Roles[0]["name"] != "Bot" || ov.Roles[1]["administrator"] != true {
		t.Errorf("roles = %v", ov.Roles)
	}
	if len(ov.Cats) != 1 || ov.Cats[0].Channels[0].Name != "rules" || len(ov.Uncat) != 1 {
		t.Errorf("tree = %+v uncat=%v", ov.Cats, ov.Uncat)
	}

	// Shaped tools compact by default; full=true returns the raw payload.
	out, _ = call(t, s, "get_message", map[string]any{"channel_id": channel, "message_id": msg})
	if out != `{"id":"new","timestamp":""}` {
		t.Errorf("compact get_message = %s", out)
	}
	out, _ = call(t, s, "get_message", map[string]any{"channel_id": channel, "message_id": msg, "full": true})
	if out != `{"id":"new"}` {
		t.Errorf("full get_message = %s", out)
	}

	out, _ = call(t, s, "permissions", map[string]any{"names": []any{"ADMINISTRATOR"}})
	if out != "8" {
		t.Errorf("permissions = %s", out)
	}
}

func TestSendDM(t *testing.T) {
	s, hits := newTestServer(t, false, []string{guild})

	out, _ := call(t, s, "send_dm", map[string]any{"user_id": user, "content": "hi"})
	if out != `{"id":"dm1","timestamp":"t","author":{"id":"b","username":"bot","bot":true},"content":"hi"}` {
		t.Fatalf("send_dm = %s", out)
	}
	n := len(*hits)
	if got := (*hits)[n-2]; got.method != "POST" || got.path != "/users/@me/channels" || got.body != `{"recipient_id":"`+user+`"}` {
		t.Errorf("open DM hit = %+v", got)
	}
	if got := (*hits)[n-1]; got.path != "/channels/"+dmChannel+"/messages" || got.body != `{"content":"hi"}` {
		t.Errorf("send hit = %+v", got)
	}

	// Non-member: rejected before any DM is opened.
	before := len(*hits)
	out, _ = call(t, s, "send_dm", map[string]any{"user_id": stranger, "content": "hi"})
	if !strings.Contains(out, "not a member of any allowed guild") {
		t.Errorf("stranger = %s", out)
	}
	for _, h := range (*hits)[before:] {
		if h.method == http.MethodPost {
			t.Errorf("POST made for non-member: %+v", h)
		}
	}

	out, _ = call(t, s, "send_dm", map[string]any{"user_id": user, "content": "blocked"})
	if !strings.Contains(out, "50007") || !strings.Contains(out, "blocked the bot") {
		t.Errorf("blocked = %s", out)
	}
	out, _ = call(t, s, "send_dm", map[string]any{"user_id": user})
	if !strings.HasPrefix(out, "ERR:") {
		t.Errorf("empty message should fail: %s", out)
	}
	out, _ = call(t, s, "send_dm", map[string]any{"user_id": "@me", "content": "x"})
	if !strings.HasPrefix(out, "ERR:") {
		t.Errorf("bad user_id should fail: %s", out)
	}

	// The generic paths stay blocked: DMs only go through send_dm.
	out, _ = call(t, s, "discord_request", map[string]any{"method": "POST", "path": "/users/@me/channels", "body": map[string]any{"recipient_id": stranger}})
	if !strings.HasPrefix(out, "ERR:") {
		t.Errorf("raw DM open should stay blocked: %s", out)
	}
}

func TestSendDMPrefix(t *testing.T) {
	s, hits := newTestServer(t, false, []string{guild}, func(d *Deps) { d.DMPrefix = "Opening line:" })

	if _, ok := call(t, s, "send_dm", map[string]any{"user_id": user, "content": "hi"}); !ok {
		t.Fatal("send_dm failed")
	}
	if got := (*hits)[len(*hits)-1].body; got != `{"content":"Opening line:\nhi"}` {
		t.Errorf("content body = %s", got)
	}

	// Embed-only DMs still carry the opening line.
	if _, ok := call(t, s, "send_dm", map[string]any{"user_id": user, "embeds": []any{map[string]any{"title": "x"}}}); !ok {
		t.Fatal("send_dm embeds failed")
	}
	if got := (*hits)[len(*hits)-1].body; got != `{"content":"Opening line:","embeds":[{"title":"x"}]}` {
		t.Errorf("embed body = %s", got)
	}

	// The prefix alone is not a message.
	if out, _ := call(t, s, "send_dm", map[string]any{"user_id": user}); !strings.HasPrefix(out, "ERR:") {
		t.Errorf("empty message should fail: %s", out)
	}
}

func TestReadOnlyMode(t *testing.T) {
	s, hits := newTestServer(t, true, nil)
	for _, name := range []string{"delete_channel", "send_dm"} {
		if _, ok := call(t, s, name, map[string]any{"channel_id": channel}); ok {
			t.Errorf("%s should not be registered in read-only mode", name)
		}
	}
	if _, ok := call(t, s, "list_roles", map[string]any{}); !ok {
		t.Error("list_roles should be registered")
	}
	out, _ := call(t, s, "discord_request", map[string]any{"method": "DELETE", "path": "/channels/" + channel})
	if !strings.HasPrefix(out, "ERR:") {
		t.Errorf("raw DELETE should fail in read-only: %s", out)
	}
	for _, h := range *hits {
		if h.method != http.MethodGet {
			t.Errorf("non-GET request in read-only mode: %+v", h)
		}
	}
}
