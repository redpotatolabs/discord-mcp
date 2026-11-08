# discord-mcp

A stdio [MCP](https://modelcontextprotocol.io) server that lets Claude manage a Discord server through a bot account: channels, categories, threads, forums, permissions, roles, members and moderation, messages, reactions, pins, invites, webhooks, emojis, stickers, soundboard, scheduled events, AutoMod, welcome screen and onboarding, audit log, and guild settings.

It has 99 tools. 95 map one-to-one onto Discord REST routes and are declared in `internal/tools/catalog.go`. The other four are:

- `server_overview`: one call returns the guild settings, the bot's highest role position, roles from top to bottom, and the channel tree grouped by category with permission overwrites.
- `permissions`: converts between permission names and bitfield strings locally, with no API call.
- `discord_request`: calls any REST v10 route that has no dedicated tool, such as application commands, stage instances or guild templates.
- `send_dm`: sends a direct message to a user. With a guild allowlist on, the user must be a member of an allowed guild, and this is the only way the server will reach a DM.

The server never opens the Discord gateway. It only uses REST, so it needs no long-running connection.

## Setup

1. **Create the bot.** Go to <https://discord.com/developers/applications>, click **New Application**, then open **Bot**.
   - Click **Reset Token** and copy the token.
   - Under **Privileged Gateway Intents**, turn on **Server Members Intent**. `list_members` and `search_members` need it.
2. **Invite it.** Open the URL below, after putting in your Application ID from **General Information**. `permissions=8` is Administrator.
   ```
   https://discord.com/oauth2/authorize?client_id=APPLICATION_ID&scope=bot&permissions=8
   ```
   Then go to **Server Settings → Roles** and drag the bot's role to the top. A bot can only manage roles and members that are below its own highest role.
3. **Configure it.** In the repo, copy the example config and fill in the token and guild. `.env` is gitignored.
   ```sh
   cp .env.example .env
   chmod 600 .env
   ```
   To get the guild ID, turn on **User Settings → Advanced → Developer Mode**, then right-click the server and choose **Copy Server ID**.
4. **Install it and register it with Claude Code.**
   ```sh
   make install                     # builds, symlinks ~/.local/bin/discord-mcp → repo binary
   claude mcp add --scope user discord -- ~/.local/bin/discord-mcp
   ```

## Configuration

Settings are read from environment variables, seeded from the first `.env` found. The server looks in this order: `DISCORD_MCP_ENV_FILE` if set, then `.env` next to the binary (symlinks are resolved, so this is the repo), then `~/.config/discord-mcp/.env`. Real environment variables override the file.

| Variable | Meaning |
|---|---|
| `DISCORD_TOKEN` / `DISCORD_TOKEN_FILE` | Bot token, given directly or as a path to a file holding it. |
| `DISCORD_GUILD_ID` | Default guild, used when a tool call omits `guild_id`. |
| `DISCORD_ALLOWED_GUILD_IDS` | Comma-separated guild allowlist. If unset, it defaults to `DISCORD_GUILD_ID`. Set it to `*` to allow every guild the bot is in. |
| `DISCORD_READ_ONLY` | `true` registers only the read tools, and `discord_request` accepts only GET. |
| `LOG_LEVEL` | `debug`, `info`, `warn` or `error`. Logs go to stderr, because stdout carries the MCP protocol. |

## Safety model

- **Guild allowlist.** Every request is checked against the allowlist before it is sent. For channel, thread, webhook and invite IDs, the server first looks up which guild owns the ID (and caches the answer). DM channels are blocked, except through `send_dm`, which first checks that the recipient is a member of an allowed guild. Writes to routes that don't belong to a guild are blocked.
- **Audit-log reasons.** Every mutating tool takes a `reason`, which appears in the server's audit log. Each mutating call also writes a line to stderr with the tool, method, path and status. Message content is not logged.
- **Annotations.** Tools carry `readOnlyHint` and `destructiveHint`, so the client can ask for approval before destructive calls (delete, ban, kick, prune, bulk delete) and allow reads freely.
- **Input checks.** Path IDs must be numeric snowflakes. Raw paths may not contain `..`, `//`, encoded slashes or a query string.

## What a bot can't do

Transfer server ownership, delete the server, manage billing or boosts, or change settings that need the owner's own 2FA. Uploading new stickers needs a multipart request, so it isn't supported. Listing, editing and deleting stickers is supported.

## Development

```sh
make test    # race detector + coverage, against a fake Discord API
make lint    # golangci-lint v2
make vuln    # govulncheck
make build
```

To add an endpoint, append an `Endpoint` to `Catalog`. The path placeholders, parameter types, the `reason` and `extra` arguments, the scope check, and the read-only filtering all come from that one declaration.
