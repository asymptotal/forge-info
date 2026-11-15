---
description: Why lookups list the directory to confirm the exact filename on macOS and Windows.
tags: [cross-platform, file-format]
---
Names are lowercased before use, but on case-insensitive file systems (default macOS APFS, NTFS) opening `foo.md` also succeeds for a hand-made `FOO.md`. Serving it would make the same store behave differently on Linux, where `FOO.md` is simply a different file. So a lookup confirms the exact name by listing the directory instead of trusting `os.Stat`, and behaves the same everywhere: `foo` is not found, `FOO.md` is reported as `invalid`/`bad_name`, and writes to `foo` fail with `conflict` rather than overwrite it.

Related rule for the same reason: segments that are Windows device names (`con`, `nul`, `com1`, also `nul.x`) are rejected on every OS, so a store written on Linux still checks out on Windows.
