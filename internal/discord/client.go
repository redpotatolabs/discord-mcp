// Package discord is a thin, rate-limited Discord REST client. It deliberately
// works on raw JSON rather than discordgo structs so every API field (including
// ones newer than the discordgo release) passes through untouched.
package discord

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/bwmarrin/discordgo"
)

// DefaultBaseURL is the Discord REST API root used in production.
const DefaultBaseURL = "https://discord.com/api/v10"

// Client issues REST calls through a discordgo session, which supplies
// per-route rate-limit buckets and 429/502 retries. The gateway is never opened.
type Client struct {
	session *discordgo.Session
	baseURL string
}

// New builds a Client from a bot token (without the "Bot " prefix).
func New(token, baseURL string) (*Client, error) {
	token = strings.TrimSpace(token)
	token = strings.TrimSpace(strings.TrimPrefix(token, "Bot "))
	if token == "" || token == "Bot" {
		return nil, errors.New("discord token is empty")
	}
	s, err := discordgo.New("Bot " + token)
	if err != nil {
		return nil, fmt.Errorf("create discord session: %w", err)
	}
	s.MaxRestRetries = 3
	s.ShouldRetryOnRateLimit = true
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	return &Client{session: s, baseURL: strings.TrimRight(baseURL, "/")}, nil
}

// Request describes one REST call. Path is relative to the API root and must
// start with "/". Body is JSON-marshalled when non-nil.
type Request struct {
	Method string
	Path   string
	Query  url.Values
	Body   any
	Reason string // X-Audit-Log-Reason
}

// APIError is a non-2xx response from Discord.
type APIError struct {
	Status int
	Body   string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("discord API error %d: %s", e.Status, e.Body)
}

// Do performs the request and returns the raw response body (empty for 204).
func (c *Client) Do(ctx context.Context, r Request) (json.RawMessage, error) {
	if !strings.HasPrefix(r.Path, "/") {
		return nil, fmt.Errorf("path must start with '/': %q", r.Path)
	}
	route := r.Path
	full := c.baseURL + r.Path
	if len(r.Query) > 0 {
		full += "?" + r.Query.Encode()
	}

	var body []byte
	if r.Body != nil {
		b, err := json.Marshal(r.Body)
		if err != nil {
			return nil, fmt.Errorf("encode request body: %w", err)
		}
		body = b
	}

	opts := []discordgo.RequestOption{discordgo.WithContext(ctx)}
	if r.Reason != "" {
		// Header values must be URL-encoded for non-ASCII reasons.
		opts = append(opts, discordgo.WithAuditLogReason(url.PathEscape(r.Reason)))
	}

	resp, err := c.session.RequestRaw(r.Method, full, "application/json", body, bucketFor(r.Method, route), 0, opts...)
	if err != nil {
		var re *discordgo.RESTError
		if errors.As(err, &re) {
			status := 0
			if re.Response != nil {
				status = re.Response.StatusCode
			}
			return nil, &APIError{Status: status, Body: string(re.ResponseBody)}
		}
		return nil, err
	}
	return resp, nil
}

// bucketFor collapses snowflakes in non-major path segments so related routes
// share a rate-limit bucket, mirroring Discord's own bucketing. Major
// parameters (the ID right after guilds/channels/webhooks) stay distinct.
func bucketFor(method, path string) string {
	segs := strings.Split(strings.Trim(path, "/"), "/")
	for i := range segs {
		if i == 1 && (segs[0] == "guilds" || segs[0] == "channels" || segs[0] == "webhooks") {
			continue
		}
		if isSnowflake(segs[i]) {
			segs[i] = "{id}"
		}
	}
	// Reactions share one bucket per channel regardless of emoji.
	for i := range segs {
		if segs[i] == "reactions" {
			segs = segs[:i+1]
			break
		}
	}
	return method + " /" + strings.Join(segs, "/")
}

func isSnowflake(s string) bool {
	if len(s) < 15 || len(s) > 21 {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// IsSnowflake reports whether s looks like a Discord ID.
func IsSnowflake(s string) bool { return isSnowflake(s) }
