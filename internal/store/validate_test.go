package store

import (
	"reflect"
	"strings"
	"testing"
)

func TestNorm(t *testing.T) {
	if got := normText("  a\r\nb\rc \n"); got != "a\nb\nc" {
		t.Errorf("normText = %q", got)
	}
	if got := normName("  Foo/Bar "); got != "foo/bar" {
		t.Errorf("normName = %q", got)
	}
}

func TestCheckName(t *testing.T) {
	long := strings.Repeat("a", 33)
	tests := []struct {
		kind   nameKind
		in     string
		want   string
		reason string // empty = valid
	}{
		{tagName, " Release ", "release", ""},
		{tagName, "a-b-c1", "a-b-c1", ""},
		{tagName, "", "", "empty"},
		{tagName, "   ", "", "empty"},
		{tagName, long, "", "too_long"},
		{tagName, strings.Repeat("a", 32), strings.Repeat("a", 32), ""},
		{tagName, "a--b", "", "bad_name"},
		{tagName, "-a", "", "bad_name"},
		{tagName, "a_b", "", "bad_name"},
		{tagName, "a/b", "", "bad_name"},
		{tagName, "ünï", "", "bad_name"},
		{tagName, "con", "", "bad_name"},
		{tagName, "COM1", "", "bad_name"},
		{tagName, "lpt9", "", "bad_name"},
		{tagName, "com0", "com0", ""},
		{entryID, "A/B.c_d-e", "a/b.c_d-e", ""},
		{entryID, "a/b/c", "a/b/c", ""},
		{entryID, "a/b/c/d", "", "bad_name"},
		{entryID, "a//b", "", "bad_name"},
		{entryID, "/a", "", "bad_name"},
		{entryID, "a/", "", "bad_name"},
		{entryID, "a.", "", "bad_name"},
		{entryID, "a..b", "", "bad_name"},
		{entryID, "nul.x", "", "bad_name"},
		{entryID, "x/aux", "", "bad_name"},
		{entryID, "x/aux.tar.gz", "", "bad_name"},
		{entryID, "console", "console", ""},
		{entryID, "a b", "", "bad_name"},
		{entryID, strings.Repeat("a", 101), "", "too_long"},
		{entryID, strings.Repeat("a", 100), strings.Repeat("a", 100), ""},
		{entryID, "", "", "empty"},
	}
	for _, tt := range tests {
		got, p := checkName(tt.kind, "f", tt.in)
		if tt.reason == "" {
			if p != nil || got != tt.want {
				t.Errorf("checkName(%d,%q) = %q, %v; want %q", tt.kind, tt.in, got, p, tt.want)
			}
			continue
		}
		if p == nil || p.Code != CodeInvalid || p.Details["reason"] != tt.reason || p.Details["field"] != "f" {
			t.Errorf("checkName(%d,%q) = %q, %+v; want reason %s", tt.kind, tt.in, got, p, tt.reason)
		}
	}
}

func TestCheckText(t *testing.T) {
	tests := []struct {
		kind   textKind
		in     string
		want   string
		reason string
	}{
		{entryDesc, "  hello \r\n", "hello", ""},
		{entryDesc, "", "", "empty"},
		{entryDesc, " \n ", "", "empty"},
		{entryDesc, "a\nb", "", "multiline"},
		{entryDesc, "a\rb", "", "multiline"},
		{entryDesc, strings.Repeat("é", 100), strings.Repeat("é", 100), ""}, // code points, not bytes
		{entryDesc, strings.Repeat("é", 101), "", "too_long"},
		{tagDesc, strings.Repeat("a", 200), strings.Repeat("a", 200), ""},
		{tagDesc, strings.Repeat("a", 201), "", "too_long"},
		{entryBody, "a\r\nb\n\n", "a\nb", ""},
		{entryBody, strings.Repeat("a", 2000), strings.Repeat("a", 2000), ""},
		{entryBody, strings.Repeat("a", 2001), "", "too_long"},
		{entryBody, "\n", "", "empty"},
	}
	for _, tt := range tests {
		got, p := checkText(tt.kind, tt.in)
		if tt.reason == "" {
			if p != nil || got != tt.want {
				t.Errorf("checkText(%v,%q) = %q, %v", tt.kind, tt.in, got, p)
			}
			continue
		}
		if p == nil || p.Details["reason"] != tt.reason || p.Details["field"] != tt.kind.field {
			t.Errorf("checkText(%v,%q) = %+v; want reason %s", tt.kind, tt.in, p, tt.reason)
		}
	}
	_, p := checkText(entryDesc, strings.Repeat("a", 134))
	if p.Details["limit"] != 100 || p.Details["actual"] != 134 || p.Message != "description: too long (134 > 100)" {
		t.Errorf("too_long details: %+v", p)
	}
}

func TestCheckTags(t *testing.T) {
	exists := func(n string) bool { return n != "ghost" && n != "ghost2" }
	tests := []struct {
		name  string
		in    []string
		want  []string
		codes []string // code/reason per problem
	}{
		{"ok sorted", []string{"b", " A "}, []string{"a", "b"}, nil},
		{"empty", nil, nil, []string{"invalid/empty"}},
		{"too many", []string{"a", "b", "c", "d", "e", "f"}, []string{"a", "b", "c", "d", "e", "f"}, []string{"invalid/too_many_tags"}},
		{"dup", []string{"a", "A"}, []string{"a"}, []string{"invalid/duplicate_tag"}},
		{"bad", []string{"a", "b_c"}, []string{"a"}, []string{"invalid/bad_name"}},
		{"unknown", []string{"ghost2", "a", "ghost"}, []string{"a", "ghost", "ghost2"}, []string{"unknown_tag/"}},
	}
	for _, tt := range tests {
		got, ps := checkTags(tt.in, exists)
		var codes []string
		for _, p := range ps {
			r, _ := p.Details["reason"].(string)
			codes = append(codes, p.Code+"/"+r)
		}
		if !reflect.DeepEqual(codes, tt.codes) {
			t.Errorf("%s: codes = %v, want %v", tt.name, codes, tt.codes)
		}
		if tt.codes == nil && !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%s: got %v", tt.name, got)
		}
	}
	_, ps := checkTags([]string{"ghost2", "ghost"}, exists)
	if !reflect.DeepEqual(ps[0].Details["tags"], []string{"ghost", "ghost2"}) {
		t.Errorf("unknown details: %v", ps[0].Details)
	}
}
