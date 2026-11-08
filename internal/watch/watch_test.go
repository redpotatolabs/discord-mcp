package watch

import (
	"reflect"
	"testing"

	"github.com/bwmarrin/discordgo"
)

const (
	bot     = "100"
	watched = "200"
	other   = "201"
	friend  = "300"
	rando   = "301"
	guildID = "400"
)

func msg(channel, author string, mut func(*discordgo.Message)) *discordgo.Message {
	m := &discordgo.Message{
		ID: "500", ChannelID: channel, GuildID: guildID, Content: "hello",
		Author: &discordgo.User{ID: author, Username: "u" + author},
	}
	if mut != nil {
		mut(m)
	}
	return m
}

func replyToBot(m *discordgo.Message) {
	m.ReferencedMessage = &discordgo.Message{ID: "499", Content: "we started RED-14", Author: &discordgo.User{ID: bot}}
}

func mentionBot(m *discordgo.Message) { m.Mentions = []*discordgo.User{{ID: bot}} }

func TestMatches(t *testing.T) {
	w := New(bot, []string{watched}, []string{friend}, nil)
	tests := []struct {
		name string
		m    *discordgo.Message
		want bool
	}{
		{"reply to bot", msg(watched, rando, replyToBot), true},
		{"mention bot", msg(watched, rando, mentionBot), true},
		{"plain message", msg(watched, friend, nil), false},
		{"reply to someone else", msg(watched, friend, func(m *discordgo.Message) {
			m.ReferencedMessage = &discordgo.Message{ID: "1", Author: &discordgo.User{ID: rando}}
		}), false},
		{"unwatched channel", msg(other, friend, replyToBot), false},
		{"dm", msg(watched, friend, func(m *discordgo.Message) { replyToBot(m); m.GuildID = "" }), false},
		{"from a bot", msg(watched, rando, func(m *discordgo.Message) { mentionBot(m); m.Author.Bot = true }), false},
		{"nil", nil, false},
	}
	for _, tt := range tests {
		if got := w.Matches(tt.m); got != tt.want {
			t.Errorf("%s: Matches = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestHandle(t *testing.T) {
	var got []map[string]any
	w := New(bot, []string{watched}, []string{friend}, func(method string, p map[string]any) {
		if method != Method {
			t.Errorf("method = %s", method)
		}
		got = append(got, p)
	})

	w.Handle(msg(watched, friend, func(m *discordgo.Message) {
		replyToBot(m)
		m.Member = &discordgo.Member{Nick: "Friend"}
		m.Attachments = []*discordgo.MessageAttachment{{URL: "https://cdn/x.png"}}
	}))
	w.Handle(msg(watched, rando, mentionBot))
	w.Handle(msg(other, friend, mentionBot)) // dropped

	if len(got) != 2 {
		t.Fatalf("got %d events, want 2", len(got))
	}
	wantContent := "[in reply to the bot's message: \"we started RED-14\"]\nhello\n[attachment: https://cdn/x.png]"
	if got[0]["content"] != wantContent {
		t.Errorf("content = %q", got[0]["content"])
	}
	wantMeta := map[string]string{"chat_id": watched, "message_id": "500", "user": "Friend", "user_id": friend, "trusted": "true", "reply_to": "499"}
	if !reflect.DeepEqual(got[0]["meta"], wantMeta) {
		t.Errorf("meta = %v", got[0]["meta"])
	}
	if meta := got[1]["meta"].(map[string]string); meta["trusted"] != "false" || meta["user"] != "u"+rando || got[1]["content"] != "hello" {
		t.Errorf("untrusted event = %v", got[1])
	}
}

func TestTruncate(t *testing.T) {
	if got := truncate("héllo", 3); got != "hél…" {
		t.Errorf("truncate = %q", got)
	}
	if got := truncate("hi", 3); got != "hi" {
		t.Errorf("truncate = %q", got)
	}
}
