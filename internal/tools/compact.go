package tools

import (
	"encoding/json"
)

// Compact message shape: drops cosmetic profile data (nameplates, avatar
// decorations, display-name styles, clan badges) that dominates raw payloads.

type rawUser struct {
	ID         string `json:"id"`
	Username   string `json:"username"`
	GlobalName string `json:"global_name"`
	Bot        bool   `json:"bot"`
}

type cUser struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Display  string `json:"display_name,omitempty"`
	Bot      bool   `json:"bot,omitempty"`
}

func compactUser(u *rawUser) *cUser {
	if u == nil || u.ID == "" {
		return nil
	}
	c := &cUser{ID: u.ID, Username: u.Username, Bot: u.Bot}
	if u.GlobalName != u.Username {
		c.Display = u.GlobalName
	}
	return c
}

type rawMessage struct {
	ID              string            `json:"id"`
	Type            int               `json:"type"`
	Content         string            `json:"content"`
	Timestamp       string            `json:"timestamp"`
	EditedTimestamp *string           `json:"edited_timestamp"`
	Pinned          bool              `json:"pinned"`
	Author          *rawUser          `json:"author"`
	Actor           *rawUser          `json:"actor"`
	Mentions        []rawUser         `json:"mentions"`
	MentionRoles    []string          `json:"mention_roles"`
	MentionEveryone bool              `json:"mention_everyone"`
	Attachments     []json.RawMessage `json:"attachments"`
	Embeds          []json.RawMessage `json:"embeds"`
	Components      []json.RawMessage `json:"components"`
	StickerItems    []struct {
		Name string `json:"name"`
	} `json:"sticker_items"`
	Reactions []struct {
		Count int `json:"count"`
		Emoji struct {
			ID   *string `json:"id"`
			Name string  `json:"name"`
		} `json:"emoji"`
	} `json:"reactions"`
	MessageReference *struct {
		MessageID string `json:"message_id"`
		ChannelID string `json:"channel_id"`
	} `json:"message_reference"`
	ReferencedMessage *rawMessage     `json:"referenced_message"`
	Thread            *cThread        `json:"thread"`
	Poll              json.RawMessage `json:"poll"`
}

type cThread struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type cAttachment struct {
	Filename    string `json:"filename"`
	URL         string `json:"url"`
	ContentType string `json:"content_type,omitempty"`
	Size        int    `json:"size,omitempty"`
	Width       int    `json:"width,omitempty"`
	Height      int    `json:"height,omitempty"`
	Description string `json:"description,omitempty"`
}

type cMessage struct {
	ID          string            `json:"id"`
	Type        int               `json:"type,omitempty"`
	TypeName    string            `json:"type_name,omitempty"`
	Timestamp   string            `json:"timestamp"`
	Edited      string            `json:"edited,omitempty"`
	Author      *cUser            `json:"author,omitempty"`
	Actor       *cUser            `json:"actor,omitempty"`
	Content     string            `json:"content,omitempty"`
	Mentions    []cUser           `json:"mentions,omitempty"`
	MentionRole []string          `json:"mention_roles,omitempty"`
	Everyone    bool              `json:"mention_everyone,omitempty"`
	Attachments []cAttachment     `json:"attachments,omitempty"`
	Embeds      []json.RawMessage `json:"embeds,omitempty"`
	Components  int               `json:"component_rows,omitempty"`
	Stickers    []string          `json:"stickers,omitempty"`
	Reactions   map[string]int    `json:"reactions,omitempty"`
	ReplyTo     *cReply           `json:"reply_to,omitempty"`
	Thread      *cThread          `json:"thread,omitempty"`
	Poll        json.RawMessage   `json:"poll,omitempty"`
	Pinned      bool              `json:"pinned,omitempty"`
}

type cReply struct {
	MessageID string `json:"message_id"`
	Author    string `json:"author,omitempty"`
	Content   string `json:"content,omitempty"`
}

// messageTypeNames covers the non-default types seen in ordinary channels.
var messageTypeNames = map[int]string{
	1: "recipient_add", 2: "recipient_remove", 3: "call", 4: "channel_name_change",
	5: "channel_icon_change", 6: "pin", 7: "member_join", 8: "boost",
	12: "channel_follow_add", 18: "thread_created", 19: "reply", 20: "slash_command",
	21: "thread_starter", 23: "context_menu_command", 24: "automod_action",
	46: "poll_result",
}

func compactMessage(m *rawMessage) cMessage {
	c := cMessage{
		ID: m.ID, Timestamp: m.Timestamp, Content: m.Content,
		Author: compactUser(m.Author), Actor: compactUser(m.Actor),
		MentionRole: m.MentionRoles, Everyone: m.MentionEveryone,
		Embeds: m.Embeds, Components: len(m.Components), Pinned: m.Pinned,
		Thread: m.Thread,
	}
	if m.Type != 0 {
		c.Type = m.Type
		c.TypeName = messageTypeNames[m.Type]
	}
	if m.EditedTimestamp != nil {
		c.Edited = *m.EditedTimestamp
	}
	for i := range m.Mentions {
		if u := compactUser(&m.Mentions[i]); u != nil {
			c.Mentions = append(c.Mentions, *u)
		}
	}
	for _, raw := range m.Attachments {
		var a cAttachment
		if json.Unmarshal(raw, &a) == nil {
			c.Attachments = append(c.Attachments, a)
		}
	}
	for _, s := range m.StickerItems {
		c.Stickers = append(c.Stickers, s.Name)
	}
	if len(m.Reactions) > 0 {
		c.Reactions = make(map[string]int, len(m.Reactions))
		for _, r := range m.Reactions {
			key := r.Emoji.Name
			if r.Emoji.ID != nil {
				key += ":" + *r.Emoji.ID
			}
			c.Reactions[key] = r.Count
		}
	}
	if len(m.Poll) > 0 && string(m.Poll) != "null" {
		c.Poll = m.Poll
	}
	if m.MessageReference != nil && m.MessageReference.MessageID != "" {
		c.ReplyTo = &cReply{MessageID: m.MessageReference.MessageID}
		if rm := m.ReferencedMessage; rm != nil {
			if rm.Author != nil {
				c.ReplyTo.Author = rm.Author.Username
			}
			c.ReplyTo.Content = truncate(rm.Content, 200)
		}
	}
	return c
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

// compactMessages reshapes a message or message array. On any decode
// problem it returns the input unchanged rather than losing data.
func compactMessages(raw json.RawMessage) json.RawMessage {
	var out any
	if len(raw) > 0 && raw[0] == '[' {
		var ms []rawMessage
		if json.Unmarshal(raw, &ms) != nil {
			return raw
		}
		cs := make([]cMessage, len(ms))
		for i := range ms {
			cs[i] = compactMessage(&ms[i])
		}
		out = cs
	} else {
		var m rawMessage
		if json.Unmarshal(raw, &m) != nil || m.ID == "" {
			return raw
		}
		out = compactMessage(&m)
	}
	b, err := json.Marshal(out)
	if err != nil {
		return raw
	}
	return b
}
