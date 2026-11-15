# forge-info-mcp

A file-backed knowledge store for coding agents, served over [MCP](https://modelcontextprotocol.io).

Agents working in a repository keep rediscovering the same things: conventions, architecture, constraints, the reasons behind odd-looking code. forge-info-mcp gives them a place to keep that knowledge as many short **entries**, one concept each, with a one-line description and a few **tags**. Agents search, triage from the descriptions, and fetch only the entries they need.

The entries are plain Markdown files in `.info/` in your repository, so they are reviewed and versioned like code. There is no database, index or cache: every call reads the files.

## Install

With Go 1.27 or later:

```sh
go install github.com/asymptotal/forge-info/cmd/forge-info-mcp@latest
```

Or download a binary for Linux, macOS or Windows from [Releases](https://github.com/asymptotal/forge-info/releases), check it against `checksums.txt`, and put it on your `PATH`.

## Set up a repository

Create the store at the root of your repository:

```sh
forge-info-mcp init
```

This creates `.info/entries/` and `.info/tags/`. Commit them.

Then register the server with your MCP client. For Claude Code, from the repository root:

```sh
claude mcp add --scope project info -- forge-info-mcp serve
```

This writes `.mcp.json`, which you can commit so everyone working in the repository gets the server. For other clients, add a stdio server that runs `forge-info-mcp serve`:

```json
{
  "mcpServers": {
    "info": { "command": "forge-info-mcp", "args": ["serve"] }
  }
}
```

`serve` finds the store by walking up from the working directory to the nearest `.info`. Pass `--root path/to/.info` to name it explicitly.

## What agents get

The server's instructions teach agents the working rules (search before working in an area and before adding, reuse existing tags, one concept per entry, record why rather than what) and list the current tags.

| Tool | What it does |
|---|---|
| `entry_search` | Find entries by terms and tags; returns ids, descriptions and tags |
| `entry_get` | Read one entry |
| `entry_add`, `entry_update`, `entry_drop` | Create, replace or delete an entry |
| `tag_list` | List tags with descriptions and usage counts |
| `tag_add`, `tag_update`, `tag_drop` | Create, describe or delete a tag |

Every read also reports broken files, so problems from hand edits or merges get noticed and fixed.

## The files

An entry, `.info/entries/session/state-transitions.md`:

```markdown
---
description: Legal session state transitions, and why the engine rejects the rest.
tags: [session, workflow]
---
Body in Markdown, up to 2,000 characters.
```

A tag, `.info/tags/session.md`, is just its one-line description.

Ids are lowercase, up to three `/`-separated segments. The same content always produces the same bytes, so diffs stay clean. forge-info-mcp only manages `.md` files in `entries/` and `tags/`; anything else you put in `.info/` is left alone.

The full rules are in [PRD.md](PRD.md) (what and why) and [TDD.md](TDD.md) (how).

## Development

The [devcontainer](.devcontainer/) has Go 1.27 and Claude Code. Then:

```sh
go test ./...
go run ./cmd/forge-info-mcp serve
```

This repository uses its own store: `.mcp.json` runs the server from source, and `.info/` holds notes on why the code is the way it is.

Releases are built by GoReleaser when a `vX.Y.Z` tag is pushed.

## License

[MIT](LICENSE)
