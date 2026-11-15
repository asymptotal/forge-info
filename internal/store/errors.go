package store

import "fmt"

// Error codes (TDD §4).
const (
	CodeInvalid      = "invalid"
	CodeUnknownTag   = "unknown_tag"
	CodeParseError   = "parse_error"
	CodeStrayFile    = "stray_file"
	CodeNotFound     = "not_found"
	CodeExists       = "exists"
	CodeTagInUse     = "tag_in_use"
	CodeInvalidInput = "invalid_input"
	CodeIOError      = "io_error"
)

// Problem is both a problem found in a file (reads) and a tool error (writes).
type Problem struct {
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Path    string         `json:"path,omitempty"`
	Details map[string]any `json:"details,omitempty"`
}

func (p *Problem) Error() string {
	if p.Path != "" {
		return fmt.Sprintf("%s: %s (%s)", p.Code, p.Message, p.Path)
	}
	return fmt.Sprintf("%s: %s", p.Code, p.Message)
}

func newProblem(code, path, format string, args ...any) *Problem {
	return &Problem{Code: code, Path: path, Message: fmt.Sprintf(format, args...)}
}

func invalidField(field, reason, message string, details map[string]any) *Problem {
	if details == nil {
		details = map[string]any{}
	}
	details["field"] = field
	details["reason"] = reason
	return &Problem{Code: CodeInvalid, Message: field + ": " + message, Details: details}
}

func ioProblem(path string, err error) *Problem {
	return newProblem(CodeIOError, path, "%v", err)
}

// withPath returns a copy of the problems with Path set.
func withPath(path string, ps ...*Problem) []Problem {
	out := make([]Problem, 0, len(ps))
	for _, p := range ps {
		q := *p
		q.Path = path
		out = append(out, q)
	}
	return out
}
