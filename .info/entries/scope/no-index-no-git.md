---
description: Why the MVP has no index, cache, locking or git awareness, and what to do instead.
tags: [scope]
---
Files are the only truth: every call rescans `.info/`. Stores are small (hundreds of short entries), so a scan is cheap, and there is no cache to go stale when someone edits files by hand or switches branches. Concurrent writers are rare; atomic temp-file renames mean readers never see half-written files and the last write wins.

Git awareness (staleness), a `check` command, hooks, `tag_rename` and `tag_merge` are listed under Later in PRD.md. Each is meant to be additive, so MVP files stay valid. Don't add them in the MVP; raise a PRD change instead.
