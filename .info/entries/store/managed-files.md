---
description: Which files in .info forge-info-mcp owns, and why everything else is silently ignored.
tags: [file-format, scope]
---
forge-info-mcp owns regular `.md` files (not dot-named) directly in `tags/` or under real directories in `entries/`. Anything else people put in `.info/` (notes, symlinks, `tags/` subdirectories, dotfiles) is ignored: not reported, served, written or deleted.

The earlier design reported such files as `stray_file` but gave the tools no way to fix them, so every agent saw permanent noise it was told to fix and couldn't. The rule now is: either a file is ours, and problems with it are reported, or it isn't, and we leave it alone. An owned file with a bad name (wrong case, too deep, `nul.md`) is still reported, as `invalid`/`bad_name`, because it is obviously meant to be an entry or tag; only a person can rename it.

Ignoring never means overwriting: if a write's path is blocked by something unowned, it fails with `conflict`.
