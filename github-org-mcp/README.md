# github-org-mcp

Go MCP server (hand-rolled JSON-RPC2 over stdio, zero external deps) for basic
git/GitHub operations, scoped to one GitHub organ (`organName`) per process.

## Build

```
cd /Volumes/RAMDisk/space/Go
go build ./...
```

Produces `./github-org-mcp`.

## Identity and config

- `env.json` in this directory holds `[{organName, accessToken, safeCommands}]`.
- Each process is bound to exactly one `organName` at start (`--org` flag or
  `ORG_NAME` env). Every `tools/call` must pass a matching `organName`
  argument (exact string match, no pattern) or the call is rejected without
  executing anything.
- Token resolution priority (`ResolveToken` in `config.go`):
  1. `ACCESS_TOKEN` env/`--token` flag passed at start (if non-empty) — wins,
     and is written back into `env.json` for that `organName`.
  2. Otherwise, the `accessToken` already stored in `env.json`.
  3. If both are empty, the process exits with a fatal "start failed" error.

## Multi-instance

One Claude `mcpServers` entry per `organName`, same binary, different
`ORG_NAME` env — e.g. `github-personal` (`ORG_NAME=personal`) and
`github-organization` (`ORG_NAME=organization`). Restarting an entry = the
harness respawning the process with the same env, which re-resolves the token.

## safeCommands guard

`safeCommands` in `env.json` is an allowlist of git subcommands considered
routine for that organ (default: `fetch, pull, branch, checkout, stash`).
Any git tool call using a subcommand **outside** that allowlist (e.g.
`commit`, `push`, `rebase`, `tag`, `clone`) logs a `MCP_ALERT: {...}` JSON
line to stderr and appends the same record to
`/Volumes/RAMDisk/log/Go/alerts-footprint.jsonl` (override via
`ALERT_LOG_PATH`). This is observability only — it never blocks execution.

## Tools

- `git_clone`, `git_fetch`, `git_commit`, `git_push`, `git_rebase`, `git_tag`
- `list_pr_comments`, `add_pr_comment`, `create_pull_request`

All require `organName` as an argument.
