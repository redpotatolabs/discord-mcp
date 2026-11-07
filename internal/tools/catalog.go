package tools

import "net/http"

// Param shorthands.
func b(name string, k Kind, desc string) Param { return Param{Name: name, Kind: k, Desc: desc} }
func br(name string, k Kind, desc string) Param {
	return Param{Name: name, Kind: k, Desc: desc, Required: true}
}
func q(name string, k Kind, desc string) Param {
	return Param{Name: name, Kind: k, Desc: desc, In: Query}
}

const (
	get   = http.MethodGet
	post  = http.MethodPost
	put   = http.MethodPut
	patch = http.MethodPatch
	del   = http.MethodDelete
)

const (
	channelTypes = "Channel type: 0 text, 2 voice, 4 category, 5 announcement, 13 stage, 15 forum, 16 media"
	permBits     = "Permission bitfield as a decimal string (use the permissions tool to convert from names)"
	overwrites   = "Permission overwrites: [{id, type (0 role, 1 member), allow, deny}] with allow/deny as bitfield strings"
)

// Pagination helpers.
var (
	limitP  = func(max string) Param { return q("limit", Int, "Max results (1-"+max+")") }
	beforeP = q("before", String, "Return results before this ID")
	afterP  = q("after", String, "Return results after this ID")
)

var channelFields = []Param{
	b("topic", String, "Channel topic (0-1024 chars; 0-4096 for forum/media)"),
	b("parent_id", String, "Category ID to place the channel under (null via extra to remove)"),
	b("position", Int, "Sorting position"),
	b("nsfw", Bool, "Age-restricted"),
	b("rate_limit_per_user", Int, "Slowmode in seconds (0-21600)"),
	b("bitrate", Int, "Voice bitrate in bits/s"),
	b("user_limit", Int, "Voice user limit (0 = unlimited)"),
	b("permission_overwrites", Objects, overwrites),
	b("default_auto_archive_duration", Int, "Default thread auto-archive minutes: 60, 1440, 4320, 10080"),
	b("default_thread_rate_limit_per_user", Int, "Default slowmode for new threads"),
	b("available_tags", Objects, "Forum/media tags: [{name, moderated, emoji_id, emoji_name}] (include id to keep existing tags)"),
	b("default_reaction_emoji", Object, "Forum default reaction: {emoji_id} or {emoji_name}"),
	b("default_sort_order", Int, "Forum sort: 0 latest activity, 1 creation date"),
	b("default_forum_layout", Int, "Forum layout: 0 not set, 1 list, 2 gallery"),
	b("rtc_region", String, "Voice region ID (null via extra for automatic)"),
	b("video_quality_mode", Int, "1 auto, 2 full (720p)"),
}

var roleFields = []Param{
	b("name", String, "Role name"),
	b("permissions", String, permBits),
	b("color", Int, "RGB colour as an integer (e.g. 0xFF0000 = 16711680)"),
	b("hoist", Bool, "Display role members separately in the sidebar"),
	b("mentionable", Bool, "Allow anyone to @mention this role"),
	b("icon", Image, "Role icon (server needs the ROLE_ICONS feature)"),
	b("unicode_emoji", String, "Unicode emoji used as the role icon"),
}

var messageFields = []Param{
	b("content", String, "Message text (up to 2000 chars, markdown supported)"),
	b("embeds", Objects, "Embed objects (up to 10): {title, description, url, color, fields:[{name,value,inline}], footer:{text}, image:{url}, thumbnail:{url}, author:{name}}"),
	b("components", Objects, "Message components (action rows with buttons/selects)"),
	b("allowed_mentions", Object, `Mention controls, e.g. {"parse": []} to suppress all pings`),
	b("flags", Int, "Message flags (4 = suppress embeds, 4096 = silent)"),
}

var eventFields = []Param{
	b("name", String, "Event name"),
	b("description", String, "Event description"),
	b("scheduled_start_time", String, "Start time, ISO8601"),
	b("scheduled_end_time", String, "End time, ISO8601 (required for external events)"),
	b("entity_type", Int, "1 stage instance, 2 voice, 3 external"),
	b("channel_id", String, "Stage/voice channel ID (omit for external)"),
	b("entity_metadata", Object, `For external events: {"location": "..."}`),
	b("privacy_level", Int, "2 = guild only (the only allowed value)"),
	b("image", Image, "Cover image"),
	b("recurrence_rule", Object, "Recurrence rule object"),
}

var automodFields = []Param{
	b("name", String, "Rule name"),
	b("event_type", Int, "1 message send, 2 member update"),
	b("trigger_metadata", Object, "Trigger config: {keyword_filter, regex_patterns, presets, allow_list, mention_total_limit, mention_raid_protection_enabled}"),
	b("actions", Objects, "Actions: [{type: 1 block | 2 alert (metadata.channel_id) | 3 timeout (metadata.duration_seconds) | 4 block member interaction, metadata}]"),
	b("enabled", Bool, "Whether the rule is enabled"),
	b("exempt_roles", Strings, "Role IDs exempt from the rule (max 20)"),
	b("exempt_channels", Strings, "Channel IDs exempt from the rule (max 50)"),
}

// Catalog lists every REST-backed tool.
var Catalog = []Endpoint{
	// ---- Guild ----
	{Name: "list_guilds", Method: get, Path: "/users/@me/guilds",
		Desc:   "List the guilds (servers) the bot is a member of.",
		Params: []Param{q("with_counts", Bool, "Include approximate member/presence counts")}},
	{Name: "get_guild", Method: get, Path: "/guilds/{guild_id}",
		Desc:   "Get guild settings and metadata (features, verification level, system channels, roles, emojis).",
		Params: []Param{q("with_counts", Bool, "Include approximate member/presence counts")}},
	{Name: "edit_guild", Method: patch, Path: "/guilds/{guild_id}", Extra: true,
		Desc: "Edit guild settings (name, icon, verification, notifications, system/rules/AFK channels, features, ...).",
		Params: []Param{
			b("name", String, "Guild name"),
			b("description", String, "Guild description (Community servers)"),
			b("icon", Image, "Guild icon"),
			b("banner", Image, "Guild banner"),
			b("splash", Image, "Invite splash"),
			b("verification_level", Int, "0 none, 1 low, 2 medium, 3 high, 4 very high"),
			b("default_message_notifications", Int, "0 all messages, 1 only mentions"),
			b("explicit_content_filter", Int, "0 disabled, 1 members without roles, 2 all members"),
			b("afk_channel_id", String, "AFK voice channel ID"),
			b("afk_timeout", Int, "AFK timeout seconds: 60, 300, 900, 1800, 3600"),
			b("system_channel_id", String, "Channel for join/boost system messages"),
			b("system_channel_flags", Int, "System channel flags bitfield"),
			b("rules_channel_id", String, "Rules channel (Community)"),
			b("public_updates_channel_id", String, "Community updates channel"),
			b("safety_alerts_channel_id", String, "Safety alerts channel"),
			b("preferred_locale", String, "Preferred locale, e.g. en-US"),
			b("features", Strings, "Full list of mutable guild features (e.g. COMMUNITY, INVITES_DISABLED, DISCOVERABLE)"),
			b("premium_progress_bar_enabled", Bool, "Show boost progress bar"),
		}},
	{Name: "leave_guild", Method: del, Path: "/users/@me/guilds/{guild_id}", Destructive: true,
		Desc: "Make the bot leave a guild. The bot loses all access until re-invited."},
	{Name: "get_audit_log", Method: get, Path: "/guilds/{guild_id}/audit-logs",
		Desc: "Read the guild audit log.",
		Params: []Param{
			q("user_id", String, "Filter by the user who performed the action"),
			q("action_type", Int, "Filter by audit-log event type number"),
			beforeP, afterP, limitP("100"),
		}},
	{Name: "get_prune_count", Method: get, Path: "/guilds/{guild_id}/prune",
		Desc: "Count members that a prune would remove.",
		Params: []Param{
			q("days", Int, "Inactive days (1-30, default 7)"),
			q("include_roles", Strings, "Also prune members with these role IDs"),
		}},
	{Name: "prune_members", Method: post, Path: "/guilds/{guild_id}/prune", Destructive: true,
		Desc: "Kick inactive members without roles (or with include_roles).",
		Params: []Param{
			b("days", Int, "Inactive days (1-30, default 7)"),
			b("include_roles", Strings, "Also prune members with these role IDs"),
			b("compute_prune_count", Bool, "Return the pruned count (slow on big guilds)"),
		}},
	{Name: "list_voice_regions", Method: get, Path: "/guilds/{guild_id}/regions",
		Desc: "List voice regions available to the guild."},
	{Name: "get_widget_settings", Method: get, Path: "/guilds/{guild_id}/widget",
		Desc: "Get the server widget settings."},
	{Name: "edit_widget", Method: patch, Path: "/guilds/{guild_id}/widget",
		Desc:   "Edit the server widget.",
		Params: []Param{b("enabled", Bool, "Widget enabled"), b("channel_id", String, "Widget invite channel")}},
	{Name: "get_vanity_url", Method: get, Path: "/guilds/{guild_id}/vanity-url",
		Desc: "Get the vanity invite code and its uses."},
	{Name: "get_welcome_screen", Method: get, Path: "/guilds/{guild_id}/welcome-screen",
		Desc: "Get the Community welcome screen."},
	{Name: "edit_welcome_screen", Method: patch, Path: "/guilds/{guild_id}/welcome-screen",
		Desc: "Edit the Community welcome screen.",
		Params: []Param{
			b("enabled", Bool, "Welcome screen enabled"),
			b("description", String, "Server description shown on the welcome screen"),
			b("welcome_channels", Objects, "Up to 5: [{channel_id, description, emoji_id, emoji_name}]"),
		}},
	{Name: "get_onboarding", Method: get, Path: "/guilds/{guild_id}/onboarding",
		Desc: "Get the Community onboarding configuration (prompts, default channels)."},
	{Name: "edit_onboarding", Method: put, Path: "/guilds/{guild_id}/onboarding", Extra: true,
		Desc: "Replace the onboarding configuration.",
		Params: []Param{
			b("prompts", Objects, "Onboarding prompts with options"),
			b("default_channel_ids", Strings, "Channels members are auto-joined to"),
			b("enabled", Bool, "Onboarding enabled"),
			b("mode", Int, "0 default channels only, 1 also count questions"),
		}},
	{Name: "list_integrations", Method: get, Path: "/guilds/{guild_id}/integrations",
		Desc: "List guild integrations (bots, Twitch/YouTube, ...)."},
	{Name: "delete_integration", Method: del, Path: "/guilds/{guild_id}/integrations/{integration_id}", Destructive: true,
		Desc: "Remove an integration (kicks its bot, if any)."},

	// ---- Channels ----
	{Name: "list_channels", Method: get, Path: "/guilds/{guild_id}/channels",
		Desc: "List all channels and categories in the guild (not threads). Prefer server_overview for a structured tree."},
	{Name: "get_channel", Method: get, Path: "/channels/{channel_id}",
		Desc: "Get a channel or thread, including its permission overwrites."},
	{Name: "create_channel", Method: post, Path: "/guilds/{guild_id}/channels", Extra: true,
		Desc:   "Create a channel or category.",
		Params: append([]Param{br("name", String, "Channel name (1-100 chars)"), b("type", Int, channelTypes)}, channelFields...)},
	{Name: "edit_channel", Method: patch, Path: "/channels/{channel_id}", Extra: true,
		Desc: "Edit a channel, category or thread (name, topic, category, permissions, slowmode, archive/lock for threads, ...).",
		Params: append([]Param{
			b("name", String, "Channel name"),
			b("type", Int, "Convert between text (0) and announcement (5)"),
			b("archived", Bool, "Thread: archived"),
			b("locked", Bool, "Thread: locked"),
			b("auto_archive_duration", Int, "Thread: 60, 1440, 4320, 10080"),
			b("applied_tags", Strings, "Forum thread: tag IDs"),
			b("lock_permissions", Bool, "Sync permissions with the new parent category"),
		}, channelFields...)},
	{Name: "delete_channel", Method: del, Path: "/channels/{channel_id}", Destructive: true,
		Desc: "Delete a channel, category or thread. Cannot be undone."},
	{Name: "reorder_channels", Method: patch, Path: "/guilds/{guild_id}/channels",
		Desc: "Move channels: change positions and/or parent categories in bulk.",
		Params: []Param{{Name: "positions", Kind: Objects, In: RawBody, Required: true,
			Desc: "[{id, position, parent_id, lock_permissions}]"}}},
	{Name: "set_channel_permission", Method: put, Path: "/channels/{channel_id}/permissions/{overwrite_id}",
		Desc: "Create or replace a permission overwrite for a role or member on a channel/category.",
		Params: []Param{
			br("type", Int, "0 role, 1 member"),
			b("allow", String, "Allowed permission bitfield string"),
			b("deny", String, "Denied permission bitfield string"),
		}},
	{Name: "delete_channel_permission", Method: del, Path: "/channels/{channel_id}/permissions/{overwrite_id}",
		Desc: "Remove a permission overwrite from a channel."},
	{Name: "follow_announcement_channel", Method: post, Path: "/channels/{channel_id}/followers",
		Desc:   "Follow an announcement channel into another channel.",
		Params: []Param{br("webhook_channel_id", String, "Target channel that receives crossposts")}},

	// ---- Invites ----
	{Name: "list_guild_invites", Method: get, Path: "/guilds/{guild_id}/invites",
		Desc: "List all active invites for the guild."},
	{Name: "list_channel_invites", Method: get, Path: "/channels/{channel_id}/invites",
		Desc: "List invites for a channel."},
	{Name: "create_invite", Method: post, Path: "/channels/{channel_id}/invites",
		Desc: "Create an invite to a channel.",
		Params: []Param{
			b("max_age", Int, "Seconds until expiry (0 = never, default 86400)"),
			b("max_uses", Int, "Max uses (0 = unlimited)"),
			b("temporary", Bool, "Grant temporary membership"),
			b("unique", Bool, "Always create a new invite"),
		}},
	{Name: "get_invite", Method: get, Path: "/invites/{code}",
		Desc:   "Look up an invite.",
		Params: []Param{q("with_counts", Bool, "Include approximate counts")}},
	{Name: "delete_invite", Method: del, Path: "/invites/{code}", Destructive: true,
		Desc: "Revoke an invite."},

	// ---- Threads ----
	{Name: "list_active_threads", Method: get, Path: "/guilds/{guild_id}/threads/active",
		Desc: "List all active (unarchived) threads in the guild."},
	{Name: "list_archived_threads", Method: get, Path: "/channels/{channel_id}/threads/archived/{visibility}",
		PathDesc: map[string]string{"visibility": "public or private"},
		Desc:     "List archived threads in a channel.",
		Params:   []Param{q("before", String, "ISO8601 timestamp to page before"), limitP("100")}},
	{Name: "start_thread", Method: post, Path: "/channels/{channel_id}/threads", Extra: true,
		Desc: "Start a thread without a message, or a forum/media post (pass message).",
		Params: []Param{
			br("name", String, "Thread name"),
			b("type", Int, "11 public, 12 private (text channels only)"),
			b("auto_archive_duration", Int, "60, 1440, 4320, 10080"),
			b("invitable", Bool, "Private threads: non-moderators can add members"),
			b("rate_limit_per_user", Int, "Slowmode seconds"),
			b("message", Object, "Forum/media starter message: {content, embeds, ...}"),
			b("applied_tags", Strings, "Forum tag IDs"),
		}},
	{Name: "start_thread_from_message", Method: post, Path: "/channels/{channel_id}/messages/{message_id}/threads",
		Desc: "Start a thread from an existing message.",
		Params: []Param{
			br("name", String, "Thread name"),
			b("auto_archive_duration", Int, "60, 1440, 4320, 10080"),
			b("rate_limit_per_user", Int, "Slowmode seconds"),
		}},
	{Name: "list_thread_members", Method: get, Path: "/channels/{channel_id}/thread-members",
		Desc:   "List members of a thread.",
		Params: []Param{q("with_member", Bool, "Include guild member objects"), afterP, limitP("100")}},
	{Name: "add_thread_member", Method: put, Path: "/channels/{channel_id}/thread-members/{user_id}",
		Desc: "Add a user to a thread (use @me for the bot)."},
	{Name: "remove_thread_member", Method: del, Path: "/channels/{channel_id}/thread-members/{user_id}",
		Desc: "Remove a user from a thread."},

	// ---- Messages ----
	{Name: "list_messages", Method: get, Path: "/channels/{channel_id}/messages",
		Desc:   "Read recent messages in a channel or thread (newest first).",
		Params: []Param{limitP("100"), beforeP, afterP, q("around", String, "Return messages around this ID")}},
	{Name: "get_message", Method: get, Path: "/channels/{channel_id}/messages/{message_id}",
		Desc: "Get a single message."},
	{Name: "send_message", Method: post, Path: "/channels/{channel_id}/messages", Extra: true,
		Desc: "Send a message to a channel or thread. Use extra for polls, stickers (sticker_ids), etc.",
		Params: append([]Param{
			b("message_reference", Object, `Reply target: {"message_id": "..."}`),
			b("tts", Bool, "Text-to-speech"),
		}, messageFields...)},
	{Name: "edit_message", Method: patch, Path: "/channels/{channel_id}/messages/{message_id}", Extra: true,
		Desc:   "Edit a message the bot sent (or only flags on others' messages).",
		Params: messageFields},
	{Name: "delete_message", Method: del, Path: "/channels/{channel_id}/messages/{message_id}", Destructive: true,
		Desc: "Delete a message."},
	{Name: "bulk_delete_messages", Method: post, Path: "/channels/{channel_id}/messages/bulk-delete", Destructive: true,
		Desc:   "Delete 2-100 messages at once (messages must be under 14 days old).",
		Params: []Param{br("messages", Strings, "Message IDs")}},
	{Name: "crosspost_message", Method: post, Path: "/channels/{channel_id}/messages/{message_id}/crosspost",
		Desc: "Publish a message in an announcement channel to followers."},
	{Name: "list_pins", Method: get, Path: "/channels/{channel_id}/pins",
		Desc: "List pinned messages."},
	{Name: "pin_message", Method: put, Path: "/channels/{channel_id}/pins/{message_id}",
		Desc: "Pin a message."},
	{Name: "unpin_message", Method: del, Path: "/channels/{channel_id}/pins/{message_id}",
		Desc: "Unpin a message."},
	{Name: "add_reaction", Method: put, Path: "/channels/{channel_id}/messages/{message_id}/reactions/{emoji}/@me",
		Desc: "React to a message as the bot."},
	{Name: "remove_reaction", Method: del, Path: "/channels/{channel_id}/messages/{message_id}/reactions/{emoji}/{user_id}",
		Desc: "Remove a user's reaction (@me for the bot's own)."},
	{Name: "list_reactions", Method: get, Path: "/channels/{channel_id}/messages/{message_id}/reactions/{emoji}",
		Desc:   "List users who reacted with an emoji.",
		Params: []Param{afterP, limitP("100")}},
	{Name: "clear_reactions", Method: del, Path: "/channels/{channel_id}/messages/{message_id}/reactions", Destructive: true,
		Desc: "Remove all reactions from a message."},

	// ---- Roles ----
	{Name: "list_roles", Method: get, Path: "/guilds/{guild_id}/roles",
		Desc: "List roles (with permissions bitfields and positions)."},
	{Name: "create_role", Method: post, Path: "/guilds/{guild_id}/roles", Extra: true,
		Desc:   "Create a role. New roles start at the bottom; use reorder_roles to move them.",
		Params: roleFields},
	{Name: "edit_role", Method: patch, Path: "/guilds/{guild_id}/roles/{role_id}", Extra: true,
		Desc:   "Edit a role. The bot can only edit roles below its highest role.",
		Params: roleFields},
	{Name: "delete_role", Method: del, Path: "/guilds/{guild_id}/roles/{role_id}", Destructive: true,
		Desc: "Delete a role."},
	{Name: "reorder_roles", Method: patch, Path: "/guilds/{guild_id}/roles",
		Desc: "Change role positions in bulk (higher position = higher in the list).",
		Params: []Param{{Name: "positions", Kind: Objects, In: RawBody, Required: true,
			Desc: "[{id, position}]"}}},
	{Name: "add_member_role", Method: put, Path: "/guilds/{guild_id}/members/{user_id}/roles/{role_id}",
		Desc: "Give a member a role."},
	{Name: "remove_member_role", Method: del, Path: "/guilds/{guild_id}/members/{user_id}/roles/{role_id}",
		Desc: "Take a role from a member."},

	// ---- Members & moderation ----
	{Name: "list_members", Method: get, Path: "/guilds/{guild_id}/members",
		Desc:   "List guild members (requires the Server Members intent enabled on the bot). Paginate with after = last user ID.",
		Params: []Param{limitP("1000"), afterP}},
	{Name: "search_members", Method: get, Path: "/guilds/{guild_id}/members/search",
		Desc:   "Search members whose username or nickname starts with a query.",
		Params: []Param{{Name: "query", Kind: String, In: Query, Required: true, Desc: "Name prefix"}, limitP("1000")}},
	{Name: "get_member", Method: get, Path: "/guilds/{guild_id}/members/{user_id}",
		Desc: "Get a guild member (roles, nickname, join date, timeout)."},
	{Name: "get_user", Method: get, Path: "/users/{user_id}",
		Desc: "Get a user's public profile (@me for the bot itself)."},
	{Name: "edit_member", Method: patch, Path: "/guilds/{guild_id}/members/{user_id}", Extra: true,
		Desc: "Edit a member: nickname, full role list, voice mute/deafen/move, timeout. To disconnect from voice or clear a timeout pass null via extra.",
		Params: []Param{
			b("nick", String, "Nickname (empty string to reset)"),
			b("roles", Strings, "Complete list of role IDs (replaces existing roles)"),
			b("mute", Bool, "Server-mute in voice"),
			b("deaf", Bool, "Server-deafen in voice"),
			b("channel_id", String, "Move to this voice channel"),
			b("communication_disabled_until", String, "Timeout until this ISO8601 time (max 28 days)"),
			b("flags", Int, "Guild member flags"),
		}},
	{Name: "edit_bot_member", Method: patch, Path: "/guilds/{guild_id}/members/@me", Extra: true,
		Desc:   "Change the bot's own nickname (and banner/avatar/bio where supported) in the guild.",
		Params: []Param{b("nick", String, "Nickname for the bot")}},
	{Name: "kick_member", Method: del, Path: "/guilds/{guild_id}/members/{user_id}", Destructive: true,
		Desc: "Kick a member from the guild."},
	{Name: "list_bans", Method: get, Path: "/guilds/{guild_id}/bans",
		Desc:   "List bans.",
		Params: []Param{limitP("1000"), beforeP, afterP}},
	{Name: "get_ban", Method: get, Path: "/guilds/{guild_id}/bans/{user_id}",
		Desc: "Get a ban (with reason)."},
	{Name: "ban_member", Method: put, Path: "/guilds/{guild_id}/bans/{user_id}", Destructive: true,
		Desc:   "Ban a user (works for non-members too).",
		Params: []Param{b("delete_message_seconds", Int, "Delete their messages from the last N seconds (0-604800)")}},
	{Name: "unban_member", Method: del, Path: "/guilds/{guild_id}/bans/{user_id}",
		Desc: "Lift a ban."},
	{Name: "bulk_ban", Method: post, Path: "/guilds/{guild_id}/bulk-ban", Destructive: true,
		Desc: "Ban up to 200 users at once.",
		Params: []Param{
			br("user_ids", Strings, "User IDs to ban"),
			b("delete_message_seconds", Int, "Delete their messages from the last N seconds (0-604800)"),
		}},

	// ---- Emojis, stickers, soundboard ----
	{Name: "list_emojis", Method: get, Path: "/guilds/{guild_id}/emojis",
		Desc: "List custom emojis."},
	{Name: "create_emoji", Method: post, Path: "/guilds/{guild_id}/emojis",
		Desc: "Upload a custom emoji (max 256 KiB).",
		Params: []Param{
			br("name", String, "Emoji name"),
			br("image", Image, "Emoji image"),
			b("roles", Strings, "Restrict to these role IDs"),
		}},
	{Name: "edit_emoji", Method: patch, Path: "/guilds/{guild_id}/emojis/{emoji_id}",
		Desc:   "Rename an emoji or change its role restrictions.",
		Params: []Param{b("name", String, "Emoji name"), b("roles", Strings, "Restrict to these role IDs")}},
	{Name: "delete_emoji", Method: del, Path: "/guilds/{guild_id}/emojis/{emoji_id}", Destructive: true,
		Desc: "Delete a custom emoji."},
	{Name: "list_stickers", Method: get, Path: "/guilds/{guild_id}/stickers",
		Desc: "List guild stickers."},
	{Name: "edit_sticker", Method: patch, Path: "/guilds/{guild_id}/stickers/{sticker_id}",
		Desc: "Edit a guild sticker.",
		Params: []Param{
			b("name", String, "Sticker name"),
			b("description", String, "Sticker description"),
			b("tags", String, "Autocomplete emoji name"),
		}},
	{Name: "delete_sticker", Method: del, Path: "/guilds/{guild_id}/stickers/{sticker_id}", Destructive: true,
		Desc: "Delete a guild sticker."},
	{Name: "list_soundboard_sounds", Method: get, Path: "/guilds/{guild_id}/soundboard-sounds",
		Desc: "List guild soundboard sounds."},
	{Name: "create_soundboard_sound", Method: post, Path: "/guilds/{guild_id}/soundboard-sounds",
		Desc: "Upload a soundboard sound (mp3/ogg, max 512 KiB, ≤5.2s).",
		Params: []Param{
			br("name", String, "Sound name"),
			br("sound", Image, "Audio file"),
			b("volume", Number, "Volume 0-1 (default 1)"),
			b("emoji_id", String, "Custom emoji ID"),
			b("emoji_name", String, "Unicode emoji"),
		}},
	{Name: "delete_soundboard_sound", Method: del, Path: "/guilds/{guild_id}/soundboard-sounds/{sound_id}", Destructive: true,
		Desc: "Delete a soundboard sound."},

	// ---- Webhooks ----
	{Name: "list_guild_webhooks", Method: get, Path: "/guilds/{guild_id}/webhooks",
		Desc: "List all webhooks in the guild."},
	{Name: "list_channel_webhooks", Method: get, Path: "/channels/{channel_id}/webhooks",
		Desc: "List webhooks for a channel."},
	{Name: "create_webhook", Method: post, Path: "/channels/{channel_id}/webhooks",
		Desc:   "Create a webhook. The response contains its secret token; treat it as a credential.",
		Params: []Param{br("name", String, "Webhook name"), b("avatar", Image, "Webhook avatar")}},
	{Name: "edit_webhook", Method: patch, Path: "/webhooks/{webhook_id}",
		Desc: "Edit a webhook (name, avatar, move to another channel).",
		Params: []Param{
			b("name", String, "Webhook name"),
			b("avatar", Image, "Webhook avatar"),
			b("channel_id", String, "Move to this channel"),
		}},
	{Name: "delete_webhook", Method: del, Path: "/webhooks/{webhook_id}", Destructive: true,
		Desc: "Delete a webhook."},

	// ---- Scheduled events ----
	{Name: "list_scheduled_events", Method: get, Path: "/guilds/{guild_id}/scheduled-events",
		Desc:   "List scheduled events.",
		Params: []Param{q("with_user_count", Bool, "Include interested-user counts")}},
	{Name: "create_scheduled_event", Method: post, Path: "/guilds/{guild_id}/scheduled-events", Extra: true,
		Desc:     "Create a scheduled event.",
		Defaults: map[string]any{"privacy_level": 2},
		Params:   eventFields},
	{Name: "edit_scheduled_event", Method: patch, Path: "/guilds/{guild_id}/scheduled-events/{event_id}", Extra: true,
		Desc:   "Edit a scheduled event, or start/end/cancel it via status (2 active, 3 completed, 4 canceled).",
		Params: append([]Param{b("status", Int, "2 active, 3 completed, 4 canceled")}, eventFields...)},
	{Name: "delete_scheduled_event", Method: del, Path: "/guilds/{guild_id}/scheduled-events/{event_id}", Destructive: true,
		Desc: "Delete a scheduled event."},
	{Name: "list_event_users", Method: get, Path: "/guilds/{guild_id}/scheduled-events/{event_id}/users",
		Desc:   "List users interested in an event.",
		Params: []Param{limitP("100"), q("with_member", Bool, "Include member objects"), beforeP, afterP}},

	// ---- Auto-moderation ----
	{Name: "list_automod_rules", Method: get, Path: "/guilds/{guild_id}/auto-moderation/rules",
		Desc: "List AutoMod rules."},
	{Name: "create_automod_rule", Method: post, Path: "/guilds/{guild_id}/auto-moderation/rules", Extra: true,
		Desc:   "Create an AutoMod rule. trigger_type: 1 keyword, 3 spam, 4 keyword preset, 5 mention spam, 6 member profile.",
		Params: append([]Param{br("trigger_type", Int, "1 keyword, 3 spam, 4 keyword preset, 5 mention spam, 6 member profile")}, automodFields...)},
	{Name: "edit_automod_rule", Method: patch, Path: "/guilds/{guild_id}/auto-moderation/rules/{rule_id}", Extra: true,
		Desc:   "Edit an AutoMod rule.",
		Params: automodFields},
	{Name: "delete_automod_rule", Method: del, Path: "/guilds/{guild_id}/auto-moderation/rules/{rule_id}", Destructive: true,
		Desc: "Delete an AutoMod rule."},
}
