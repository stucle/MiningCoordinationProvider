# github-org-mcp

Go MCP server (hand-rolled JSON-RPC2 over stdio, zero external deps) for basic git/GitHub operations, scoped to one GitHub organ (`organName`) per process.

## Build

From this source directory, wherever it lives:

```
go build ./...
```

Produces `./github-org-mcp`. The binary is self-contained — it carries no dependency on the source directory's path, only on the fixed config location below.
If the `mcpServers` entries point at this binary by path, moving this directory means updating that `command` path in Claude's config and in the `github-org-mcp` skill, not anything in here.

## Identity and config

- The config source of truth is `env.json` holding   `[{organName, accessToken, safeCommands}]`.
It **must** live under the   Claude config folder — the exact path varies per machine, but is fixed for that machine, with no runtime override:
  `<home>/.claude/github-org-mcp/env.json`  (`os.UserHomeDir()` + `.claude/github-org-mcp/env.json`).
  The `env.json` checked into this source directory is only a template (empty `accessToken`s) for reference — it is never read at runtime.
- Each process is bound to exactly one `organName` at start (`--org` flag or `ORG_NAME` env).
  Every `tools/call` must pass a matching `organName` argument (exact string match, no pattern) or the call is rejected without executing anything.
- Token resolution priority at start (`ResolveToken` in `config.go`):
  1. `ACCESS_TOKEN` env/`--token` flag passed at start (if non-empty) — wins, and is written back into `env.json` for that `organName`.
  2. Otherwise, the `accessToken` already stored in `env.json`.
  3. If both are empty, the process exits with a fatal "start failed" error.
- The resolved token never has to leave the Go process. `git_clone`/`git_fetch`/`git_push`
  inject it themselves as a `-c http.extraHeader=Authorization: Basic ...` global git flag
  (`authHeaderArgs` in `gitops.go`), ahead of the subcommand — callers pass plain
  `repoUrl`/`remote` with no embedded credentials, and the header is never included in
  `guardCheck`'s alert args or `tool_response` text.

## reload_config

- A dedicated tool, `reload_config` (args: `organName` required as usual, `token` optional). If `token` is non-empty it overwrites the stored `accessToken` for that `organName` in `env.json`; if empty, the existing stored token is reused unchanged.
- After responding, the process calls `os.Exit(0)`.
  It does not restart or re-exec itself — "restarting the instance" is Claude Code respawning the process (per its `mcpServers` entry) the next time any tool on that instance is called,
  at which point the respawned process reads the already-updated `env.json`.

## Multi-instance

One Claude `mcpServers` entry per `organName`, same binary, different `ORG_NAME` env — e.g. `github-personal` (`ORG_NAME=personal`) and `github-organization` (`ORG_NAME=organization`).
Restarting an entry = the harness respawning the process with the same env, which re-resolves the token.

## safeCommands guard

`safeCommands` in `env.json` is an allowlist of git subcommands considered routine for that organ (default: `fetch, pull, branch, checkout, stash`).
Any git tool call using a subcommand **outside** that allowlist (e.g. `commit`, `push`, `rebase`, `tag`, `clone`)
logs a `MCP_ALERT: {...}` JSON line to stderr and appends the same record to `~hooks~/alerts-footprint.jsonl` (override via `ALERT_LOG_PATH`).
This is observability only — it never blocks execution.

## Tools

- `git_clone`, `git_fetch`, `git_commit`, `git_push`, `git_rebase`, `git_tag`
- `list_pr_comments`, `add_pr_comment`, `create_pull_request`
- `reload_config`

All require `organName` as an argument.
