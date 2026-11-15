package server

import (
	"fmt"
	"strings"

	"github.com/asymptotal/forge-info/internal/store"
)

const maxTagsWithDescriptions = 100

const (
	descSearch    = "Search entries. Every whitespace-separated term in query must appear (case-insensitive substring) in the id, description, a tag name or the body; tags requires all named tags (unknown tag: unknown_tag). Ordered by number of matches, then id; no query lists by id. Returns summaries (id, description, tags), total and truncated; broken entries are left out and reported in problems. Search before working in an area and before adding."
	descGet       = "Get one entry by id: description, tags and body. If the file is broken, entry is null, problems says why and raw holds the file text so it can be fixed with entry_update."
	descAdd       = "Add a new entry. One concept per entry; record why, not what. Description: one line, max 100 chars, written so an agent can triage from it. Tags: 1 to 5 existing tags (check tag_list; add missing ones with tag_add). Body: Markdown, max 2000 chars, no headings. Fails with exists if the id is taken (even by a broken file), or conflict if the path is blocked by a file a person must move; search first and prefer entry_update."
	descUpdate    = "Replace an entry completely (description, tags and body are all required), including a broken one. changed is false and nothing is written if the result is identical. Same rules as entry_add."
	descDrop      = "Delete an entry by id, including a broken one."
	descTagList   = "List all tags with description and count of valid entries using them, sorted by name. A tag with a broken description shows null. Check this before tag_add."
	descTagAdd    = "Add a tag: lowercase letters and digits separated by single hyphens, max 32 chars, with a one-line description of max 200 chars. Fails with exists if it is already there. The tag list in the server instructions is only a startup snapshot."
	descTagUpdate = "Replace a tag's description (one line, max 200 chars), including a broken one."
	descTagDrop   = "Delete a tag, including a broken one. Fails with tag_in_use (details.entries) if valid entries still use it; update those entries first."
)

const rules = `infomcp is a file-backed knowledge store for this project: many short entries, each covering one concept, with a one-line description and 1 to 5 tags. Tags have descriptions too. Entries live in .info/ and are read and written only through these tools.

Working rules:
- Search (entry_search) before working in an area and before adding anything; fetch only the entries you need with entry_get.
- Check tag_list before tag_add; reuse existing tags.
- One concept per entry. Record why, not what.
- Write descriptions an agent can triage from without opening the entry.
- No headings in bodies.
- Fix problems reported in results (broken entries or tags) with entry_update, tag_update or the drop tools. A file with an invalid name (reason bad_name) can only be renamed or removed by a person: tell the user.`

func instructions(st *store.Store) (string, error) {
	res, err := st.ListTags()
	if err != nil {
		return "", err
	}
	var b strings.Builder
	b.WriteString(rules)
	b.WriteString("\n\nTags")
	if len(res.Tags) > maxTagsWithDescriptions {
		b.WriteString(" (name: entries):\n")
	} else {
		b.WriteString(" (name: entries, description):\n")
	}
	if len(res.Tags) == 0 {
		b.WriteString("(none yet)\n")
	}
	for _, t := range res.Tags {
		switch {
		case len(res.Tags) > maxTagsWithDescriptions:
			fmt.Fprintf(&b, "- %s: %d\n", t.Name, t.Count)
		case t.Description == nil:
			fmt.Fprintf(&b, "- %s: %d, (broken description)\n", t.Name, t.Count)
		default:
			fmt.Fprintf(&b, "- %s: %d, %s\n", t.Name, t.Count, *t.Description)
		}
	}
	b.WriteString("\nThis tag list is a snapshot from server startup; call tag_list for the current state.")
	return b.String(), nil
}
