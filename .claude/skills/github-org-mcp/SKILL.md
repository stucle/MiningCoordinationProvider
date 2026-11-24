---
name: github-org-mcp
description: Route git/GitHub operations (clone, commit, push, fetch, rebase, tag, PR comments) through the per-organ Go MCP server instances github-personal and github-organization, instead of raw git or gh. Use whenever a task needs git/GitHub actions scoped to one of these two configured organs.
---

# github-org-mcp

Two MCP server instances, same binary (`~/MCP/github-routing/github-org-mcp`),
each bound to exactly one `organName`:

- `github-personal` — tool prefix `mcp__github-personal__*`, `organName` must be the literal `"personal"`
- `github-organization` — tool prefix `mcp__github-organization__*`, `organName` must be the literal `"organization"`

## Rules

- `organName` is a **fixed literal value** — never inferred, never pattern-matched.
  Always pass exactly `"personal"` or `"organization"`, matching the instance you're
  calling. A mismatch is rejected by the server before anything runs.
- Pick the instance based on which account/org the target repo belongs to. If unclear,
  ask the user instead of guessing.
- Every tool requires `organName` plus its own fields.

## Tools

| Tool | Required args | Notes |
|---|---|---|
| `git_clone` | `organName, repoUrl, dest` | token is injected via header, never exposed in `repoUrl` |
| `git_fetch` | `organName, workdir` (`remote` optional) | |
| `git_commit` | `organName, workdir, message` (`paths` optional) | local only, no token needed |
| `git_push` | `organName, workdir, remote, branch` (`force` optional) | outside the default allowlist → triggers `MCP_ALERT` |
| `git_rebase` | `organName, workdir, onto` | outside the default allowlist → triggers `MCP_ALERT` |
| `git_tag` | `organName, workdir, name` (`message` optional) | outside the default allowlist → triggers `MCP_ALERT` |
| `list_pr_comments` | `organName, owner, repo, prNumber` | review comments + issue (team) comments |
| `add_pr_comment` | `organName, owner, repo, prNumber, body` | |
| `create_pull_request` | `organName, owner, repo, head, base, title` (`body` optional) | |
| `reload_config` | `organName` (`token` optional) | reloads the stored token in `env.json`; the process then exits, Claude Code respawns it on the next tool call |

## MCP_ALERT is not an error

`git_clone`, `git_commit`, `git_push`, `git_rebase`, `git_tag` sit outside each organ's
default `safeCommands` allowlist (`fetch, pull, branch, checkout, stash`). Calling them
may return a response text line like `MCP_ALERT: command "..." is outside safeCommands
allowlist`. This is observability only (logged) and **never blocks execution** — treat it
as expected, not a failure to fix.

## Token — never surface it

Each organ's token lives in `~/.claude/github-org-mcp/env.json` and is injected by the Go
process straight into git via `-c http.extraHeader=...`. Never:
- add the token to `repoUrl`/`remote` yourself
- read or print the contents of `env.json`
- include the token in anything shown to the user

## Out of scope

Don't use this server for GitHub work outside the two configured organs (`personal`,
`organization`). For anything else (issues, releases, Actions logs, ad-hoc REST/GraphQL),
use the `gh-personal` wrapper (`github-personal` skill) or ask the user about scope.
