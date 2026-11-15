# forge-info-mcp: product requirements (MVP)

A local, file-backed knowledge store for agents, served over MCP.

## Problem

Agents working in a repository keep rediscovering the same things: conventions, architecture, flows, constraints. That knowledge either lives nowhere, or in long documents an agent has to read end to end to find the one paragraph that matters.

## Solution

Store project knowledge as many short **entries**, each covering one concept, each with a one-line description and one or more **tags**. Tags have descriptions too. Agents search, read the descriptions, and fetch only the entries they need.

The files live in the repo under `.info/`, so they are reviewed and versioned like code. Agents read and write them only through the MCP server.

## Principles

1. **Files are the truth.** No database, cache or index. Every call reads the files.
2. **The tool is the interface.** Agents use the MCP tools, not the files.
3. **Normalise the obvious, reject the rest.** Surrounding whitespace is trimmed, line endings are normalised, and names are case-insensitive. Anything else that's wrong is rejected with a clear error.
4. **One bad file never blocks the rest.** Reads skip broken files and report them. Writes check only what they write.
5. **Own only what is ours.** forge-info-mcp manages the `.md` files under `.info/entries/` and `.info/tags/`. Anything else people put in `.info/` (other files, symlinks, dotfiles, subdirectories of `tags/`) is ignored: never reported, served or touched. A managed file that is broken is always reported.
6. **Small entries.** One concept per entry. Record why, not what.

## Users and use

- **Agents** (Claude Code and other MCP clients): search the store before working in an area, fetch the relevant entries, and add or update entries when they learn something durable.
- **Humans:** run `forge-info-mcp init` once, register the server with their client, and review `.info/` changes in PRs.

## Scope

The MVP covers:

- Nine tools: `entry_search`, `entry_get`, `entry_add`, `entry_update`, `entry_drop`, `tag_list`, `tag_add`, `tag_update`, `tag_drop`.
- Server instructions that teach agents the working rules and list the current tags.
- CLI: `forge-info-mcp serve [--root <dir>]`, `forge-info-mcp init [<dir>]` and `forge-info-mcp version`.
- One binary for Linux, macOS and Windows, installed with `go install` or downloaded from GitHub Releases.

## Out of scope

- Any git awareness (staleness, confirmation).
- Validating Markdown structure in bodies.
- Multi-file operations such as `tag_rename` and `tag_merge`.
- A `check` command, hooks, CI integration.
- Embeddings, sync, a UI, locking.

## Later

In rough order, each one additive (MVP files stay valid): git-backed staleness; a `check` command for CI; `tag_rename` / `tag_merge`; body structure validation; package managers (Homebrew, Scoop).

## Done when

- An agent in Claude Code can find, read, add, update and drop entries and tags.
- Broken managed files are reported. Broken content can be fixed through the tools; a file with an invalid name is reported for a person to rename or remove.
- The same content always produces the same bytes on disk.
- Tests pass on Linux, macOS and Windows.
