# infomcp: technical design (MVP)

How the [PRD](PRD.md) is built. Where this document is silent, apply the shared rules in §3 and §4; don't invent case-by-case behaviour.

## 1. Store

```
.info/                  <- store root
  entries/
    release-tag-format.md
    session/state-transitions.md
  tags/
    release.md
    session.md
```

- **Root:** `--root <dir>` names the `.info` directory explicitly. Without it, the server walks up from the working directory to the first `.info`. If that directory doesn't contain `entries/` and `tags/`, the server refuses to start.
- **`init [<dir>]`** creates `<dir>/.info/entries/.gitkeep` and `<dir>/.info/tags/.gitkeep`. It fails if `.info` already exists.
- Names starting with `.` are ignored everywhere (`.gitkeep`, `.DS_Store`, temp files).

## 2. File formats

**Entry** (`entries/<id>.md`):

```
---
description: Legal session state transitions, and why the engine rejects the rest.
tags: [session, workflow]
---
Body in Markdown.
```

- The frontmatter is a fixed line format, parsed by hand, not YAML: a `---` line, then exactly the two lines `description:` and `tags: [...]` in either order, then a `---` line. Everything after that is the body.
- Anything else in the frontmatter is a `parse_error`.

**Tag** (`tags/<name>.md`): the whole file, trimmed, is the description. No frontmatter.

**Canonical output.** Every write produces identical bytes for identical content:

- `description` comes first, then `tags` sorted and joined by `, `.
- No blank line after the closing `---`.
- The body ends with exactly one `\n`.
- LF line endings, UTF-8, no byte-order mark.

**Reads accept** CRLF line endings, a byte-order mark, either key order, and the usual whitespace around keys, values, commas and brackets.

## 3. Shared rules

Every input, whether a tool argument or a value read from a file, goes through the same few functions, in `internal/store/validate.go`.

**Normalisation** (`normText`, `normName`):

- Text (descriptions, body): CRLF/CR → LF, then trim surrounding whitespace.
- Names (ids, tags): trim, then lowercase.

**Names** (`checkName`):

| | Rule |
|---|---|
| Tag name | `^[a-z0-9]+(-[a-z0-9]+)*$`, ≤ 32 chars |
| Entry id | 1–3 `/`-separated segments, each `^[a-z0-9]+([._-][a-z0-9]+)*$`, whole id ≤ 100 chars |
| Both | No segment that is a Windows device name (`con`, `prn`, `aux`, `nul`, `com1`–`9`, `lpt1`–`9`), including with an extension (`nul.x`) |

**Text** (`checkText`), with lengths counted in code points after normalising:

| Field | Limit | Line breaks |
|---|---|---|
| Entry description | 1–100 | not allowed |
| Tag description | 1–200 | not allowed |
| Body | 1–2,000 | allowed |

**Tag lists** (`checkTags`): 1–5 tags, each a valid name, no duplicates, each existing. A tag **exists** if `tags/<name>.md` is a regular file with a valid name, even if its description is broken.

**Paths** (`resolve`): a path is only ever built from a name that passed `checkName`. Inside the root, symlinks are never followed; any symlink encountered is a `stray_file`. On case-insensitive file systems a lookup must also confirm the exact name on disk (by listing the directory), so `FOO.md` is never served as `foo`.

**Writes** (`writeFile`): write to a `.`-prefixed temp file in the target directory, then rename it into place. Parent directories are created as needed. Directories that `entry_drop` leaves empty are removed, up to `entries/`. A directory still holding only dotfiles is left in place.

## 4. Errors and problems

One shape for both, with the same codes:

```json
{ "code": "invalid", "message": "description: too long (134 > 100)", "path": "entries/foo.md",
  "details": { "field": "description", "reason": "too_long", "limit": 100, "actual": 134 } }
```

| Code | Meaning |
|---|---|
| `invalid` | A field fails §3. `details.field` names it, and `details.reason` is one of `empty`, `too_long`, `multiline`, `bad_name`, `too_many_tags`, `duplicate_tag`. |
| `unknown_tag` | A named tag doesn't exist. `details.tags` lists them. |
| `parse_error` | An entry's frontmatter doesn't follow §2 (files only). |
| `stray_file` | Not a regular `.md` file, too deeply nested, a directory under `tags/`, or a symlink (files only). |
| `not_found` | The target entry or tag doesn't exist. |
| `exists` | The `_add` target already exists, valid or broken. |
| `tag_in_use` | `tag_drop` on a tag that valid entries use. `details.entries` lists them. |
| `invalid_input` | Bad tool arguments, e.g. `limit` out of range. |
| `io_error` | The file system failed. |

- **Writes** return the first failure, checking in this order: name, tags, description, body.
- **Reads** report every problem in every broken file, as `problems`. `path` is relative to the root, and the list is sorted by path, then code.
- **Tool errors** set `isError: true` and carry the JSON as text content only, with no `structuredContent`. In go-sdk, the handler returns a Go `error` whose `Error()` is that JSON. This is the only way that works with typed, schema-checked outputs.
- **Malformed arguments** (a missing field, a wrong type, an unknown property) are rejected by the SDK's input schema before the handler runs, with the SDK's own plain-text message. `invalid_input` covers the checks the schema can't express, such as `limit` being out of range.

## 5. Tools

Every read result includes `problems` (possibly empty). Results use `structuredContent`, with the same JSON as text.

- **Summary:** `{id, description, tags}`
- **Entry:** summary + `body`
- **Tag:** `{name, description, count}`, where `count` is the number of valid entries using the tag

| Tool | Input | Returns | Notes |
|---|---|---|---|
| `entry_search` | `query?`, `tags?`, `limit` = 50 (1–200) | `{total, truncated, entries: [summary], problems}` | Broken entries are left out; every broken file in the store is reported. |
| `entry_get` | `id` | `{entry, problems}`, or `{entry: null, problems, raw}` if broken | Checks only this file. No `raw` for a `stray_file`. |
| `entry_add` | `id`, `description`, `tags`, `body` | entry | Fails with `exists`. |
| `entry_update` | same as `entry_add` | `{changed, entry}` | Replaces the entry, broken or not. If the canonical bytes are unchanged, `changed` is false and nothing is written. |
| `entry_drop` | `id` | `{id}` | Works on broken entries. |
| `tag_list` | none | `{tags: [tag], problems}` | Sorted by name. A broken description is shown as `null`. |
| `tag_add` | `name`, `description` | tag | Fails with `exists`. |
| `tag_update` | `name`, `description` | `{changed, tag}` | Works on broken tags. |
| `tag_drop` | `name` | `{name}` | Fails with `tag_in_use` if valid entries use it. Works on broken tags. |

**Search:**

- `query` is split on whitespace. Every term must appear, as a case-insensitive substring, in the id, description, a tag name or the body.
- Results are ordered by total non-overlapping occurrences (highest first), then by id. With no query, by id.
- `tags` requires all of the named tags. An unknown tag fails the search with `unknown_tag`.

**Server instructions** are built at startup:

- what the store is;
- the working rules (search before working or adding, check `tag_list` before `tag_add`, one concept per entry, write descriptions an agent can triage from, no headings, fix reported problems);
- every tag with its description and count (names and counts only above 100 tags);
- a note that the list is a startup snapshot.

Each tool description restates the rules that apply to it.

**Concurrency:** no locking. The last write wins, and atomic renames mean readers never see half-written files.

## 6. Code layout

```
cmd/infomcp/        CLI: serve, init; exit 0 on success, 2 on error
internal/store/     validate.go (§3), format.go (parse/render), store.go (scan, CRUD)
internal/server/    MCP tools, schemas, instructions; thin wrapper over store
```

- `store` has no MCP dependency and holds all the logic.
- `server` maps store results and errors to MCP results.
- Dependencies are `github.com/modelcontextprotocol/go-sdk` and the standard library. Use `path/filepath` throughout.

## 7. Testing

- **Unit tests:** table-driven tests for `validate` and `format`, including round trips (parse ∘ render = identity on canonical files).
- **Store tests:** run against `t.TempDir()` fixtures, covering broken files, symlinks and dotfiles.
- **Server tests:** end to end over the SDK's in-memory transport.
- **CI:** tests run on Linux, macOS and Windows.
