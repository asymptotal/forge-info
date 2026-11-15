# forge-info

Go implementation of infomcp: a file-backed knowledge store for agents, served over MCP.

- [PRD.md](PRD.md) (what and why) and [TDD.md](TDD.md) (how) are the source of truth. Implement what they say; if something is ambiguous or wrong, raise it and update the doc rather than deciding silently in code.
- Keep it simple. Validation, normalisation and path safety go through the shared functions in TDD.md §3; don't add per-case rules.
- This is the MVP. Everything under "Out of scope" and "Later" in PRD.md is deliberately out of scope: no git, no `check`, no hooks, no `tag_rename`/`tag_merge`. Don't add them.
- Dependencies: the MCP Go SDK (`github.com/modelcontextprotocol/go-sdk`) and the standard library only. Frontmatter is parsed by hand, not with a YAML library.
- The binary must work on Linux, macOS and Windows: use `path/filepath`, and keep the Windows rules in TDD.md §3 (reserved names, case-insensitive file systems) in mind.
- Development happens in the devcontainer (`.devcontainer/`), Go 1.27.
