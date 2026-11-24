# github-org-mcp — how to use it

A hand-rolled Go MCP server (no external deps, JSON-RPC2 over stdio) for basic
git/GitHub operations, where each process is scoped to exactly one GitHub organ
(`organName`) and one token.

## Why two instances

One binary, run twice with a different `ORG_NAME` environment variable, gives two
separate servers in Claude:

- `github-personal` — the personal GitHub account
- `github-organization` — the work organization

On every tool call you must pass an `organName` that matches the instance you're
calling (e.g. calling a tool on `github-personal` requires `organName: "personal"`).
If it doesn't match, the server rejects the call outright and runs nothing — this
guards against accidentally pushing to the wrong org by a typo.

## Available tools

- `git_clone`, `git_fetch`, `git_commit`, `git_push`, `git_rebase`, `git_tag`
- `list_pr_comments`, `add_pr_comment`, `create_pull_request`
- `reload_config` — reload the token after rotating it

All of them require `organName`. The git tools run real `git` on disk (via `workdir`);
the PR tools call the GitHub REST API directly.

## How the token works

The token never appears in your request, never in `repoUrl`. It's stored in
`~/.claude/github-org-mcp/env.json`, keyed by `organName`, and the Go process loads it
and attaches it as an `Authorization` header right before calling `git`/the GitHub API.
So the token never ends up in logs, in the text returned to you, or in `.git/config`.

To rotate a token (expired, revoked, newly issued):

1. Call the `reload_config` tool with the matching `organName` and the new `token`.
2. The server writes the new token into `env.json`, replies, then exits.
3. The next tool call on that instance makes Claude Code respawn the process
   automatically — nothing else to do.

Calling `reload_config` with `token` left empty just reloads the existing token from
`env.json`, unchanged.

## What "MCP_ALERT" means

Each organ has a list of git subcommands considered "safe" (default: `fetch, pull,
branch, checkout, stash`). Calling anything outside that list — `clone`, `commit`,
`push`, `rebase`, `tag` — still runs normally, but the response includes an
`MCP_ALERT: ...` line and the event is appended to a log file
(`alerts-footprint.jsonl`). This is observability only, not a block — seeing this line
means the command already ran successfully.

## Deliberate limitations

- Scoped only to the two organs declared in `env.json`. Anything else on GitHub
  (issue/release management, Actions, ad-hoc API calls) is out of scope for this
  server.
- No guard against dangerous operations (`push --force`, tag overwrite, etc.) —
  `MCP_ALERT` only logs, it never prevents. Think before you push/force.
- `git add`/`commit`/`rebase`/`tag` run locally and need no token; only
  `clone`/`fetch`/`push` get the token attached.

## Rebuilding

`~/MCP/github-routing` is the local clone of the `github-org-mcp` GitHub repo.

```
cd ~/MCP/github-routing
go build ./...
```

Produces `./github-org-mcp` — the path used as `command` in the `mcpServers` config
(see the accompanying config file). Restart Claude Code (or the relevant MCP process)
after rebuilding to pick up the new binary.

# mcpServers config for github-org-mcp

Add to the `mcpServers` object in Claude's config (`~/.claude.json`, either at the global
or per-project level). Same binary, only `ORG_NAME` differs:

```json
{
  "mcpServers": {
    "github-personal": {
      "type": "stdio",
      "command": "~/MCP/github-routing/github-org-mcp",
      "args": [],
      "env": {
        "ORG_NAME": "personal"
      }
    },
    "github-organization": {
      "type": "stdio",
      "command": "~/MCP/github-routing/github-org-mcp",
      "args": [],
      "env": {
        "ORG_NAME": "organization"
      }
    }
  }
}
```

## Notes

- `~/MCP/github-routing` is the local clone of the `github-org-mcp` GitHub repo — the
  `command` paths below point into that clone.
- `github-personal` is already present in the current `~/.claude.json`; `github-organization`
  is not yet added — the block above is for adding that second instance.
- `command` is the path to the built binary (`go build ./...` from `~/MCP/github-routing`).
  If the clone moves, only `command` here needs updating — the binary itself carries no
  dependency on the source path.
- No need to set `ACCESS_TOKEN`/`--token` in `env` if `env.json` already has a token for that organ — the stored token is used as a fallback. Only set it to overwrite (the new
  value gets written back into `env.json`).
- The real token file is `~/.claude/github-org-mcp/env.json` (mode 600, never committed,
  not the `env.json` checked into the source directory — that one is an empty template).
- After editing `mcpServers`, restart Claude Code (or that MCP instance) to pick up the
  new config.
