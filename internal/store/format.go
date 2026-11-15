package store

import (
	"strings"
	"unicode/utf8"
)

const bom = "\xef\xbb\xbf"

type frontmatter struct {
	description string
	tags        []string
}

// decode checks UTF-8, strips a BOM and normalises line endings.
func decode(data []byte) (string, *Problem) {
	if !utf8.Valid(data) {
		return "", newProblem(CodeParseError, "", "file is not valid UTF-8")
	}
	return normLines(strings.TrimPrefix(string(data), bom)), nil
}

// parseEntry parses an entry file into raw (unvalidated) fields.
func parseEntry(data []byte) (frontmatter, string, *Problem) {
	var fm frontmatter
	text, p := decode(data)
	if p != nil {
		return fm, "", p
	}
	lines := strings.Split(text, "\n")
	if strings.TrimSpace(lines[0]) != "---" {
		return fm, "", newProblem(CodeParseError, "", "frontmatter must start with a --- line")
	}
	if len(lines) < 4 || strings.TrimSpace(lines[3]) != "---" {
		return fm, "", newProblem(CodeParseError, "",
			"frontmatter must be exactly a description line, a tags line and a closing --- line")
	}
	seen := map[string]bool{}
	for _, line := range lines[1:3] {
		key, val, ok := strings.Cut(line, ":")
		key, val = strings.TrimSpace(key), strings.TrimSpace(val)
		if !ok || (key != "description" && key != "tags") {
			return fm, "", newProblem(CodeParseError, "", "unexpected frontmatter line %q", strings.TrimSpace(line))
		}
		if seen[key] {
			return fm, "", newProblem(CodeParseError, "", "duplicate key %q", key)
		}
		seen[key] = true
		if key == "description" {
			fm.description = val
			continue
		}
		if !strings.HasPrefix(val, "[") || !strings.HasSuffix(val, "]") {
			return fm, "", newProblem(CodeParseError, "", "tags must be a bracketed list, e.g. [a, b]")
		}
		if inner := strings.TrimSpace(val[1 : len(val)-1]); inner != "" {
			for _, t := range strings.Split(inner, ",") {
				fm.tags = append(fm.tags, strings.TrimSpace(t))
			}
		}
	}
	return fm, strings.Join(lines[4:], "\n"), nil
}

// renderEntry produces the canonical bytes. Inputs must already be validated
// and normalised; tags must be sorted.
func renderEntry(description string, tags []string, body string) []byte {
	return []byte("---\ndescription: " + description + "\ntags: [" + strings.Join(tags, ", ") + "]\n---\n" + body + "\n")
}

// parseTag returns the raw description of a tag file.
func parseTag(data []byte) (string, *Problem) { return decode(data) }

func renderTag(description string) []byte { return []byte(description + "\n") }
