# discord-mcp

A Go MCP server for managing Discord servers through a bot account. It deploys nothing, so there is no `test` branch or test site.

- `main` only takes squash-merged pull requests with a passing `checks` CI job. Work goes on a branch from `main` and into a pull request. Never push to `main` and never force push.
- The repo is public. Never commit tokens, `.env`, real server or user IDs, or personal details.
- Before pushing, run `go vet ./...` and `go test -race ./...` (or `make vet test`; `make all` adds lint and govulncheck).
- CI (`.github/workflows/ci.yml`) runs on a GitHub-hosted runner with no secrets. Keep it that way: fork pull requests run there.
