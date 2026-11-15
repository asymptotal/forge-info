package store

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"
)

const (
	maxTagName  = 32
	maxEntryID  = 100
	maxTags     = 5
	maxSegments = 3
)

type nameKind int

const (
	tagName nameKind = iota
	entryID
)

type textKind struct {
	field     string
	max       int
	multiline bool
}

var (
	entryDesc = textKind{"description", 100, false}
	tagDesc   = textKind{"description", 200, false}
	entryBody = textKind{"body", 2000, true}
)

var (
	tagRe = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)
	segRe = regexp.MustCompile(`^[a-z0-9]+([._-][a-z0-9]+)*$`)

	reserved = map[string]bool{"con": true, "prn": true, "aux": true, "nul": true}
)

func init() {
	for i := '1'; i <= '9'; i++ {
		reserved["com"+string(i)] = true
		reserved["lpt"+string(i)] = true
	}
}

// normLines converts CRLF and CR to LF.
func normLines(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	return strings.ReplaceAll(s, "\r", "\n")
}

// normText normalises descriptions and bodies.
func normText(s string) string { return strings.TrimSpace(normLines(s)) }

// normName normalises tag names and entry ids.
func normName(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

func isReserved(seg string) bool {
	base, _, _ := strings.Cut(seg, ".")
	return reserved[base]
}

// checkName normalises raw and validates it as a tag name or entry id.
func checkName(kind nameKind, field, raw string) (string, *Problem) {
	s := normName(raw)
	limit := maxTagName
	if kind == entryID {
		limit = maxEntryID
	}
	n := utf8.RuneCountInString(s)
	switch {
	case n == 0:
		return "", invalidField(field, "empty", "empty", nil)
	case n > limit:
		return "", invalidField(field, "too_long", fmt.Sprintf("too long (%d > %d)", n, limit),
			map[string]any{"limit": limit, "actual": n})
	}
	segs := []string{s}
	re := tagRe
	if kind == entryID {
		segs = strings.Split(s, "/")
		re = segRe
	}
	ok := len(segs) <= maxSegments
	for _, seg := range segs {
		if !re.MatchString(seg) || isReserved(seg) {
			ok = false
		}
	}
	if !ok {
		return "", invalidField(field, "bad_name", fmt.Sprintf("%q is not a valid name", s), nil)
	}
	return s, nil
}

// checkText normalises raw and validates it against the field's limits.
func checkText(k textKind, raw string) (string, *Problem) {
	s := normText(raw)
	n := utf8.RuneCountInString(s)
	switch {
	case n == 0:
		return "", invalidField(k.field, "empty", "empty", nil)
	case !k.multiline && strings.Contains(s, "\n"):
		return "", invalidField(k.field, "multiline", "must be a single line", nil)
	case n > k.max:
		return "", invalidField(k.field, "too_long", fmt.Sprintf("too long (%d > %d)", n, k.max),
			map[string]any{"limit": k.max, "actual": n})
	}
	return s, nil
}

// checkTags normalises and validates a tag list. exists reports whether a tag
// exists. It returns the sorted tag names and every problem found.
func checkTags(raw []string, exists func(string) bool) ([]string, []Problem) {
	var probs []*Problem
	if len(raw) == 0 {
		probs = append(probs, invalidField("tags", "empty", "at least one tag is required", nil))
	}
	if len(raw) > maxTags {
		probs = append(probs, invalidField("tags", "too_many_tags",
			fmt.Sprintf("too many (%d > %d)", len(raw), maxTags),
			map[string]any{"limit": maxTags, "actual": len(raw)}))
	}
	seen := map[string]bool{}
	var names, unknown []string
	for _, r := range raw {
		name, p := checkName(tagName, "tags", r)
		if p != nil {
			p.Details["tag"] = strings.TrimSpace(r)
			p.Message = fmt.Sprintf("tags: %q: %s", strings.TrimSpace(r), strings.TrimPrefix(p.Message, "tags: "))
			probs = append(probs, p)
			continue
		}
		if seen[name] {
			probs = append(probs, invalidField("tags", "duplicate_tag", fmt.Sprintf("duplicate tag %q", name),
				map[string]any{"tag": name}))
			continue
		}
		seen[name] = true
		names = append(names, name)
		if !exists(name) {
			unknown = append(unknown, name)
		}
	}
	if len(unknown) > 0 {
		sort.Strings(unknown)
		probs = append(probs, &Problem{Code: CodeUnknownTag,
			Message: "unknown tags: " + strings.Join(unknown, ", "),
			Details: map[string]any{"tags": unknown}})
	}
	sort.Strings(names)
	return names, withPath("", probs...)
}
