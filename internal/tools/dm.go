package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"net/http"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/epistemi/discord-mcp/internal/discord"
)

// send_dm is the one sanctioned path to DMs. The scope guard blocks DM
// channels outright (they belong to no guild); this tool instead requires the
// recipient to be a member of an allowed guild before opening the DM.

func dmTool(prefix string) mcp.Tool {
	desc := "Send a direct message to a user. When a guild allowlist is configured, the user must be a member of an allowed guild. Fails if the user has DMs from server members disabled or has blocked the bot."
	if prefix != "" {
		desc += fmt.Sprintf(" The server automatically starts every DM with the opening line %q; do not include it yourself.", prefix)
	}
	return mcp.NewTool("send_dm",
		mcp.WithDescription(desc),
		mcp.WithReadOnlyHintAnnotation(false),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(false),
		mcp.WithOpenWorldHintAnnotation(true),
		mcp.WithString("user_id", mcp.Required(), mcp.Description("Recipient user ID")),
		mcp.WithString("content", mcp.Description("Message text (up to 2000 chars, markdown supported)")),
		mcp.WithArray("embeds", mcp.Description("Embed objects (up to 10)"), mcp.Items(map[string]any{"type": "object"})),
		mcp.WithObject("extra", mcp.Description("Additional raw message body fields (components, allowed_mentions, poll, ...)"), mcp.AdditionalProperties(true)),
	)
}

func dmHandler(d Deps) server.ToolHandlerFunc {
	return func(ctx context.Context, r mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		userID := strings.TrimSpace(r.GetString("user_id", ""))
		if !discord.IsSnowflake(userID) {
			return mcp.NewToolResultError("user_id must be a numeric Discord ID"), nil
		}
		args := r.GetArguments()
		body := map[string]any{}
		if extra, ok := args["extra"].(map[string]any); ok {
			maps.Copy(body, extra)
		}
		if c := r.GetString("content", ""); c != "" {
			body["content"] = c
		}
		if e, ok := args["embeds"].([]any); ok && len(e) > 0 {
			body["embeds"] = e
		}
		if body["content"] == nil && body["embeds"] == nil && body["poll"] == nil && body["components"] == nil {
			return mcp.NewToolResultError("provide content, embeds, or other message fields via extra"), nil
		}
		if d.DMPrefix != "" {
			body["content"] = withPrefix(d.DMPrefix, body["content"])
		}

		guild, err := sharedGuild(ctx, d, userID)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		raw, err := d.Client.Do(ctx, discord.Request{
			Method: http.MethodPost, Path: "/users/@me/channels",
			Body: map[string]any{"recipient_id": userID},
		})
		if err != nil {
			return mcp.NewToolResultError("open DM channel: " + err.Error()), nil
		}
		var ch struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(raw, &ch); err != nil || !discord.IsSnowflake(ch.ID) {
			return mcp.NewToolResultError("open DM channel: unexpected response " + string(raw)), nil
		}

		resp, err := d.Client.Do(ctx, discord.Request{
			Method: http.MethodPost, Path: "/channels/" + ch.ID + "/messages", Body: body,
		})
		status := "success"
		if err != nil {
			status = "error"
		}
		slog.Info("audit", "tool", "send_dm", "user_id", userID, "via_guild", guild, "status", status)
		if err != nil {
			var ae *discord.APIError
			if errors.As(err, &ae) && strings.Contains(ae.Body, "50007") {
				return mcp.NewToolResultError("Discord refused the DM (50007): the user has DMs from server members disabled or has blocked the bot"), nil
			}
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText(formatResult(compactMessages(resp))), nil
	}
}

// sharedGuild returns an allowed guild the user belongs to. With no
// allowlist it returns "" and leaves the shared-server rule to Discord.
func sharedGuild(ctx context.Context, d Deps, userID string) (string, error) {
	guilds := d.Scope.AllowedGuilds()
	if guilds == nil {
		return "", nil
	}
	for _, g := range guilds {
		_, err := d.Client.Do(ctx, discord.Request{Method: http.MethodGet, Path: "/guilds/" + g + "/members/" + userID})
		if err == nil {
			return g, nil
		}
		var ae *discord.APIError
		if !errors.As(err, &ae) || ae.Status != http.StatusNotFound {
			return "", fmt.Errorf("check membership in guild %s: %w", g, err)
		}
	}
	return "", fmt.Errorf("user %s is not a member of any allowed guild (%s)", userID, strings.Join(guilds, ", "))
}

// withPrefix puts the opening line above the message text, or makes it the
// whole text when the message has none (embed- or component-only DMs).
func withPrefix(prefix string, content any) string {
	if c, ok := content.(string); ok && c != "" {
		return prefix + "\n" + c
	}
	return prefix
}
