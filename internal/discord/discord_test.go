package discord

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
)

const (
	allowedGuild = "111111111111111111"
	otherGuild   = "222222222222222222"
	chanAllowed  = "333333333333333333"
	chanOther    = "444444444444444444"
	chanDM       = "555555555555555555"
)

type recorded struct {
	method, path, query, auth, reason, body string
}

func fakeDiscord(t *testing.T) (*Client, *[]recorded, *int32) {
	t.Helper()
	var calls []recorded
	var channelLookups int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		calls = append(calls, recorded{r.Method, r.URL.Path, r.URL.RawQuery, r.Header.Get("Authorization"), r.Header.Get("X-Audit-Log-Reason"), string(body)})
		switch r.URL.Path {
		case "/channels/" + chanAllowed:
			atomic.AddInt32(&channelLookups, 1)
			_, _ = w.Write([]byte(`{"id":"` + chanAllowed + `","guild_id":"` + allowedGuild + `"}`))
		case "/channels/" + chanOther:
			_, _ = w.Write([]byte(`{"id":"` + chanOther + `","guild_id":"` + otherGuild + `"}`))
		case "/channels/" + chanDM:
			_, _ = w.Write([]byte(`{"id":"` + chanDM + `","type":1}`))
		case "/forbidden":
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"message":"Missing Permissions","code":50013}`))
		case "/empty":
			w.WriteHeader(http.StatusNoContent)
		default:
			_, _ = w.Write([]byte(`{"ok":1}`))
		}
	}))
	t.Cleanup(srv.Close)
	c, err := New("Bot abc.def", srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	return c, &calls, &channelLookups
}

func TestDoSendsAuthReasonQueryBody(t *testing.T) {
	c, calls, _ := fakeDiscord(t)
	_, err := c.Do(context.Background(), Request{
		Method: http.MethodPatch, Path: "/guilds/" + allowedGuild,
		Query:  url.Values{"with_counts": {"true"}},
		Body:   map[string]any{"name": "x"},
		Reason: "tidy up ✨",
	})
	if err != nil {
		t.Fatal(err)
	}
	got := (*calls)[0]
	if got.auth != "Bot abc.def" {
		t.Errorf("auth = %q (token prefix should not be doubled)", got.auth)
	}
	if got.query != "with_counts=true" || got.body != `{"name":"x"}` {
		t.Errorf("query/body = %q %q", got.query, got.body)
	}
	if dec, _ := url.PathUnescape(got.reason); dec != "tidy up ✨" {
		t.Errorf("reason = %q", got.reason)
	}
}

func TestDoAPIError(t *testing.T) {
	c, _, _ := fakeDiscord(t)
	_, err := c.Do(context.Background(), Request{Method: http.MethodGet, Path: "/forbidden"})
	var ae *APIError
	if !errors.As(err, &ae) || ae.Status != 403 || !strings.Contains(ae.Body, "Missing Permissions") {
		t.Fatalf("err = %v", err)
	}
}

func TestDoNoContent(t *testing.T) {
	c, _, _ := fakeDiscord(t)
	resp, err := c.Do(context.Background(), Request{Method: http.MethodDelete, Path: "/empty"})
	if err != nil || len(resp) != 0 {
		t.Fatalf("resp=%q err=%v", resp, err)
	}
}

func TestDoRejectsRelativePath(t *testing.T) {
	c, _, _ := fakeDiscord(t)
	if _, err := c.Do(context.Background(), Request{Method: http.MethodGet, Path: "guilds"}); err == nil {
		t.Fatal("expected error")
	}
}

func TestNewRejectsEmptyToken(t *testing.T) {
	if _, err := New("  Bot ", ""); err == nil {
		t.Fatal("expected error")
	}
}

func TestBucketFor(t *testing.T) {
	cases := map[string]string{
		"/channels/333333333333333333/messages/999999999999999999":                 "DELETE /channels/333333333333333333/messages/{id}",
		"/channels/333333333333333333/messages/999999999999999999/reactions/x/@me": "DELETE /channels/333333333333333333/messages/{id}/reactions",
	}
	for in, want := range cases {
		if got := bucketFor(http.MethodDelete, in); got != want {
			t.Errorf("bucketFor(%s) = %s, want %s", in, got, want)
		}
	}
}

func TestScope(t *testing.T) {
	c, _, lookups := fakeDiscord(t)
	s := NewScope(c, []string{allowedGuild})
	ctx := context.Background()

	ok := []struct{ m, p string }{
		{"GET", "/guilds/" + allowedGuild + "/roles"},
		{"DELETE", "/guilds/" + allowedGuild + "/bans/1"},
		{"POST", "/channels/" + chanAllowed + "/messages"},
		{"GET", "/users/@me"},
		{"GET", "/users/@me/guilds"},
		{"DELETE", "/users/@me/guilds/" + allowedGuild},
	}
	for _, c := range ok {
		if err := s.Check(ctx, c.m, c.p); err != nil {
			t.Errorf("%s %s: unexpected %v", c.m, c.p, err)
		}
	}
	bad := []struct{ m, p string }{
		{"GET", "/guilds/" + otherGuild},
		{"POST", "/guilds"},
		{"POST", "/channels/" + chanOther + "/messages"},
		{"POST", "/channels/" + chanDM + "/messages"},
		{"POST", "/users/@me/channels"},
		{"DELETE", "/users/@me/guilds/" + otherGuild},
		{"POST", "/applications/1/commands"},
	}
	for _, c := range bad {
		if err := s.Check(ctx, c.m, c.p); err == nil {
			t.Errorf("%s %s: expected rejection", c.m, c.p)
		}
	}

	// Channel ownership is cached.
	_ = s.Check(ctx, "GET", "/channels/"+chanAllowed+"/pins")
	if n := atomic.LoadInt32(lookups); n != 1 {
		t.Errorf("channel lookups = %d, want 1 (cached)", n)
	}
}

func TestScopeUnrestricted(t *testing.T) {
	c, calls, _ := fakeDiscord(t)
	s := NewScope(c, nil)
	if !s.Unrestricted() || !s.GuildAllowed(otherGuild) {
		t.Fatal("expected unrestricted")
	}
	if err := s.Check(context.Background(), "POST", "/channels/"+chanOther+"/messages"); err != nil {
		t.Fatal(err)
	}
	if len(*calls) != 0 {
		t.Errorf("unrestricted scope should not resolve channels, made %d calls", len(*calls))
	}
}

func TestIsSnowflake(t *testing.T) {
	for s, want := range map[string]bool{"123456789012345678": true, "12": false, "12345678901234567a": false, "@me": false} {
		if IsSnowflake(s) != want {
			t.Errorf("IsSnowflake(%q) != %v", s, want)
		}
	}
}
