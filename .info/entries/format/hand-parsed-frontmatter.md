---
description: Why entry frontmatter is a fixed two-line format parsed by hand instead of YAML.
tags: [file-format, scope]
---
Frontmatter is exactly `description:` and `tags: [...]` between `---` lines, parsed in `internal/store/format.go`. A YAML library would add a dependency (the project allows only the MCP SDK and stdlib) and would accept far more than we want: anchors, multi-line scalars, quoting styles, extra keys. Each of those is another way for two files with the same content to differ in bytes, which breaks the canonical-output guarantee and makes PR diffs noisy.

A fixed format keeps parse ∘ render an identity on canonical files, and anything outside it is simply a `parse_error` the agent can fix with `entry_update`. If richer metadata is ever needed, extend the line format rather than switching to YAML.
