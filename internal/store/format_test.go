package store

import (
	"reflect"
	"testing"
)

func TestParseEntry(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want frontmatter
		body string
		bad  bool
	}{
		{"canonical", "---\ndescription: d\ntags: [a, b]\n---\nbody\n", frontmatter{"d", []string{"a", "b"}}, "body\n", false},
		{"crlf", "---\r\ndescription: d\r\ntags: [a]\r\n---\r\nx\r\ny\r\n", frontmatter{"d", []string{"a"}}, "x\ny\n", false},
		{"bom", "\xef\xbb\xbf---\ndescription: d\ntags: [a]\n---\nb", frontmatter{"d", []string{"a"}}, "b", false},
		{"swapped order", "---\ntags: [a]\ndescription: d\n---\nb\n", frontmatter{"d", []string{"a"}}, "b\n", false},
		{"whitespace", "--- \n  description :   d e  \n tags:[ a ,b ]  \n---  \nb", frontmatter{"d e", []string{"a", "b"}}, "b", false},
		{"colon in description", "---\ndescription: a: b\ntags: [x]\n---\nb", frontmatter{"a: b", []string{"x"}}, "b", false},
		{"dashes in body", "---\ndescription: d\ntags: [x]\n---\n---\nb", frontmatter{"d", []string{"x"}}, "---\nb", false},
		{"empty tags", "---\ndescription: d\ntags: []\n---\nb", frontmatter{"d", nil}, "b", false},
		{"no body", "---\ndescription: d\ntags: [x]\n---", frontmatter{"d", []string{"x"}}, "", false},
		{"empty tag item", "---\ndescription: d\ntags: [a,,b]\n---\nb", frontmatter{"d", []string{"a", "", "b"}}, "b", false},
		{"no frontmatter", "hello\n", frontmatter{}, "", true},
		{"empty file", "", frontmatter{}, "", true},
		{"unclosed", "---\ndescription: d\ntags: [a]\nbody\n", frontmatter{}, "", true},
		{"missing tags", "---\ndescription: d\n---\nb\n", frontmatter{}, "", true},
		{"extra key", "---\ndescription: d\ntags: [a]\nfoo: x\n---\nb", frontmatter{}, "", true},
		{"unknown key", "---\ndescription: d\ntitle: [a]\n---\nb", frontmatter{}, "", true},
		{"duplicate key", "---\ndescription: d\ndescription: e\n---\nb", frontmatter{}, "", true},
		{"blank line in fm", "---\ndescription: d\n\ntags: [a]\n---\nb", frontmatter{}, "", true},
		{"uppercase key", "---\nDescription: d\ntags: [a]\n---\nb", frontmatter{}, "", true},
		{"tags not list", "---\ndescription: d\ntags: a, b\n---\nb", frontmatter{}, "", true},
		{"yaml block list", "---\ndescription: d\ntags:\n  - a\n---\nb", frontmatter{}, "", true},
		{"no colon", "---\ndescription\ntags: [a]\n---\nb", frontmatter{}, "", true},
		{"bad utf8", "---\ndescription: d\xff\ntags: [a]\n---\nb", frontmatter{}, "", true},
	}
	for _, tt := range tests {
		fm, body, p := parseEntry([]byte(tt.in))
		if tt.bad {
			if p == nil || p.Code != CodeParseError {
				t.Errorf("%s: want parse_error, got %+v", tt.name, p)
			}
			continue
		}
		if p != nil || !reflect.DeepEqual(fm, tt.want) || body != tt.body {
			t.Errorf("%s: got %+v %q %v", tt.name, fm, body, p)
		}
	}
}

func TestRenderRoundTrip(t *testing.T) {
	canon := "---\ndescription: Legal states.\ntags: [session, workflow]\n---\nBody line 1.\n\nBody 2.\n"
	if got := string(renderEntry("Legal states.", []string{"session", "workflow"}, "Body line 1.\n\nBody 2.")); got != canon {
		t.Fatalf("render = %q", got)
	}
	fm, body, p := parseEntry([]byte(canon))
	if p != nil {
		t.Fatal(p)
	}
	if got := string(renderEntry(fm.description, fm.tags, normText(body))); got != canon {
		t.Errorf("round trip = %q", got)
	}
	// Non-canonical inputs converge on the canonical bytes.
	for _, in := range []string{
		"\xef\xbb\xbf---\r\ntags: [workflow,session]\r\ndescription:  Legal states. \r\n---\r\n\r\nBody line 1.\r\n\r\nBody 2.\r\n\r\n",
		"---\ntags: [ Session , WORKFLOW ]\ndescription: Legal states.\n---\nBody line 1.\n\nBody 2.",
	} {
		fm, body, p := parseEntry([]byte(in))
		if p != nil {
			t.Fatalf("%q: %v", in, p)
		}
		names, ps := checkTags(fm.tags, func(string) bool { return true })
		desc, _ := checkText(entryDesc, fm.description)
		b, _ := checkText(entryBody, body)
		if len(ps) > 0 {
			t.Fatal(ps)
		}
		if got := string(renderEntry(desc, names, b)); got != canon {
			t.Errorf("%q -> %q", in, got)
		}
	}
}

func TestTagFormat(t *testing.T) {
	got, p := parseTag([]byte("\xef\xbb\xbf  Release things \r\n\r\n"))
	if p != nil {
		t.Fatal(p)
	}
	d, p := checkText(tagDesc, got)
	if p != nil || d != "Release things" {
		t.Fatalf("%q %v", d, p)
	}
	if string(renderTag(d)) != "Release things\n" {
		t.Errorf("renderTag = %q", renderTag(d))
	}
	if _, p := parseTag([]byte{0xff}); p == nil || p.Code != CodeParseError {
		t.Errorf("bad utf8: %+v", p)
	}
}
