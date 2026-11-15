package store

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func newStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	if err := Init(dir); err != nil {
		t.Fatal(err)
	}
	s, err := Open(filepath.Join(dir, ".info"))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func put(t *testing.T, s *Store, rel, content string) {
	t.Helper()
	p := filepath.Join(s.root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func exists(s *Store, rel string) bool {
	_, err := os.Lstat(filepath.Join(s.root, filepath.FromSlash(rel)))
	return err == nil
}

func entryFile(desc string, tags string, body string) string {
	return "---\ndescription: " + desc + "\ntags: [" + tags + "]\n---\n" + body + "\n"
}

func mustTag(t *testing.T, s *Store, names ...string) {
	t.Helper()
	for _, n := range names {
		if _, err := s.AddTag(n, "Tag "+n); err != nil {
			t.Fatal(err)
		}
	}
}

func code(err error) string {
	var p *Problem
	if errors.As(err, &p) {
		return p.Code
	}
	return "other:" + errString(err)
}

func errString(err error) string {
	if err == nil {
		return "nil"
	}
	return err.Error()
}

func problemKeys(ps []Problem) []string {
	out := []string{}
	for _, p := range ps {
		out = append(out, p.Path+"|"+p.Code)
	}
	return out
}

func ids(r *SearchResult) []string {
	out := []string{}
	for _, e := range r.Entries {
		out = append(out, e.ID)
	}
	return out
}

func TestInitFindOpen(t *testing.T) {
	dir := t.TempDir()
	if err := Init(dir); err != nil {
		t.Fatal(err)
	}
	if err := Init(dir); err == nil {
		t.Error("second init should fail")
	}
	for _, f := range []string{".info/entries/.gitkeep", ".info/tags/.gitkeep"} {
		if _, err := os.Stat(filepath.Join(dir, f)); err != nil {
			t.Error(err)
		}
	}
	deep := filepath.Join(dir, "a", "b")
	os.MkdirAll(deep, 0o755)
	root, err := Find(deep)
	if err != nil || root != filepath.Join(dir, ".info") {
		t.Errorf("Find = %q, %v", root, err)
	}
	if _, err := Open(dir); err == nil {
		t.Error("Open without entries/tags should fail")
	}
}

func TestCRUDAndChanged(t *testing.T) {
	s := newStore(t)
	mustTag(t, s, "a", "b")
	e, err := s.AddEntry(" Foo/Bar ", " desc \r\n", []string{"B", "a"}, "\r\nbody\r\n\r\n")
	if err != nil {
		t.Fatal(err)
	}
	if e.ID != "foo/bar" || e.Description != "desc" || !reflect.DeepEqual(e.Tags, []string{"a", "b"}) || e.Body != "body" {
		t.Errorf("entry = %+v", e)
	}
	data, _ := os.ReadFile(filepath.Join(s.root, "entries", "foo", "bar.md"))
	if string(data) != "---\ndescription: desc\ntags: [a, b]\n---\nbody\n" {
		t.Errorf("bytes = %q", data)
	}
	if _, err := s.AddEntry("foo/bar", "x", []string{"a"}, "y"); code(err) != CodeExists {
		t.Errorf("dup add: %v", err)
	}
	if _, err := s.AddEntry("FOO/BAR", "x", []string{"a"}, "y"); code(err) != CodeExists {
		t.Errorf("case-insensitive dup add: %v", err)
	}

	// Unchanged update must not touch the file.
	path := filepath.Join(s.root, "entries", "foo", "bar.md")
	old := []byte("---\r\ntags: [b, a]\r\ndescription: desc\r\n---\r\nbody\r\n")
	os.WriteFile(path, old, 0o644)
	r, err := s.UpdateEntry("foo/bar", "desc", []string{"a", "b"}, "body")
	if err != nil || !r.Changed {
		t.Fatalf("non-canonical file should be rewritten: %+v %v", r, err)
	}
	before, _ := os.Stat(path)
	r, err = s.UpdateEntry("Foo/Bar", "desc", []string{"b", "a"}, "body\n")
	if err != nil || r.Changed {
		t.Fatalf("changed = %+v %v", r, err)
	}
	after, _ := os.Stat(path)
	if !after.ModTime().Equal(before.ModTime()) {
		t.Error("file rewritten although unchanged")
	}
	r, _ = s.UpdateEntry("foo/bar", "desc2", []string{"a"}, "body")
	if !r.Changed || r.Entry.Description != "desc2" {
		t.Errorf("update = %+v", r)
	}
	if _, err := s.UpdateEntry("nope", "d", []string{"a"}, "b"); code(err) != CodeNotFound {
		t.Errorf("update missing: %v", err)
	}
	// Update replaces a broken entry.
	put(t, s, "entries/broken.md", "garbage")
	if r, err := s.UpdateEntry("broken", "fixed", []string{"a"}, "b"); err != nil || !r.Changed {
		t.Errorf("fix broken: %+v %v", r, err)
	}
	g, err := s.Get("BROKEN")
	if err != nil || g.Entry == nil || g.Entry.Description != "fixed" || len(g.Problems) != 0 {
		t.Errorf("get fixed: %+v %v", g, err)
	}
	// Tag updates.
	tr, err := s.UpdateTag("a", "Tag a")
	if err != nil || tr.Changed || tr.Tag.Count != 2 {
		t.Errorf("tag update unchanged: %+v %v", tr, err)
	}
	tr, _ = s.UpdateTag("a", "new")
	if !tr.Changed || *tr.Tag.Description != "new" {
		t.Errorf("tag update: %+v", tr)
	}
	// No temp files left behind.
	des, _ := os.ReadDir(filepath.Join(s.root, "entries"))
	for _, d := range des {
		if strings.HasPrefix(d.Name(), ".tmp") {
			t.Errorf("leftover %s", d.Name())
		}
	}
}

func TestWriteValidationOrder(t *testing.T) {
	s := newStore(t)
	mustTag(t, s, "a")
	long := strings.Repeat("x", 101)
	tests := []struct {
		name       string
		id, d      string
		tags       []string
		body       string
		code, rsn  string
		field      string
		wantFields string
	}{
		{"name first", "Bad Name", long, []string{"ghost"}, "", CodeInvalid, "bad_name", "id", ""},
		{"tags second", "ok", long, []string{"ghost"}, "", CodeUnknownTag, "", "", ""},
		{"desc third", "ok", long, []string{"a"}, "", CodeInvalid, "too_long", "description", ""},
		{"body last", "ok", "d", []string{"a"}, " ", CodeInvalid, "empty", "body", ""},
		{"too many", "ok", "d", []string{"a", "b", "c", "d", "e", "f"}, "b", CodeInvalid, "too_many_tags", "tags", ""},
	}
	for _, tt := range tests {
		_, err := s.AddEntry(tt.id, tt.d, tt.tags, tt.body)
		var p *Problem
		if !errors.As(err, &p) || p.Code != tt.code {
			t.Errorf("%s: %v", tt.name, err)
			continue
		}
		if tt.rsn != "" && (p.Details["reason"] != tt.rsn || p.Details["field"] != tt.field) {
			t.Errorf("%s: %+v", tt.name, p)
		}
	}
	if _, err := s.AddEntry("nul", "d", []string{"a"}, "b"); code(err) != CodeInvalid {
		t.Errorf("reserved: %v", err)
	}
	if _, err := s.AddTag("com1", "d"); code(err) != CodeInvalid {
		t.Errorf("reserved tag: %v", err)
	}
}

func TestBrokenFilesOfEachKind(t *testing.T) {
	s := newStore(t)
	mustTag(t, s, "a")
	put(t, s, "entries/good.md", entryFile("good", "a", "body"))
	put(t, s, "entries/noparse.md", "no frontmatter")
	put(t, s, "entries/longdesc.md", entryFile(strings.Repeat("d", 101), "a", "body"))
	put(t, s, "entries/emptybody.md", "---\ndescription: d\ntags: [a]\n---\n")
	put(t, s, "entries/unknown.md", entryFile("d", "a, ghost", "body"))
	put(t, s, "entries/notags.md", "---\ndescription: d\ntags: []\n---\nb\n")
	put(t, s, "entries/Upper.md", entryFile("d", "a", "body"))
	put(t, s, "entries/readme.txt", "x")
	put(t, s, "entries/a/b/c/too-deep.md", entryFile("d", "a", "body"))
	put(t, s, "entries/a/b/ok.md", entryFile("d", "a", "body"))
	put(t, s, "entries/.hidden.md", "garbage")
	put(t, s, "entries/.dir/x.md", "garbage")
	put(t, s, "entries/nul.md", entryFile("d", "a", "body"))
	put(t, s, "tags/Bad.md", "x")
	put(t, s, "tags/sub/x.md", "x")
	put(t, s, "tags/empty.md", "  \n")
	put(t, s, "tags/.hidden.md", "x")

	res, err := s.Search("", nil, DefaultLimit)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"a/b/ok", "good"}; !reflect.DeepEqual(ids(res), want) {
		t.Errorf("ids = %v", ids(res))
	}
	want := []string{
		"entries/Upper.md|stray_file",
		"entries/a/b/c|stray_file",
		"entries/emptybody.md|invalid",
		"entries/longdesc.md|invalid",
		"entries/noparse.md|parse_error",
		"entries/notags.md|invalid",
		"entries/nul.md|stray_file",
		"entries/readme.txt|stray_file",
		"entries/unknown.md|unknown_tag",
		"tags/Bad.md|stray_file",
		"tags/empty.md|invalid",
		"tags/sub|stray_file",
	}
	if got := problemKeys(res.Problems); !reflect.DeepEqual(got, want) {
		t.Errorf("problems:\n got %v\nwant %v", got, want)
	}
	if res.Problems[3].Details["field"] != "description" {
		t.Errorf("details: %+v", res.Problems[3])
	}

	tl, err := s.ListTags()
	if err != nil {
		t.Fatal(err)
	}
	if len(tl.Tags) != 2 || tl.Tags[0].Name != "a" || tl.Tags[0].Count != 2 || tl.Tags[1].Name != "empty" || tl.Tags[1].Description != nil {
		t.Errorf("tags = %+v", tl.Tags)
	}
	if len(tl.Problems) != len(want) {
		t.Errorf("tag_list problems = %v", problemKeys(tl.Problems))
	}

	// Get: broken returns raw, only this file's problems.
	g, err := s.Get("noparse")
	if err != nil || g.Entry != nil || g.Raw == nil || *g.Raw != "no frontmatter" || len(g.Problems) != 1 {
		t.Errorf("get broken: %+v %v", g, err)
	}
	g, _ = s.Get("unknown")
	if g.Problems[0].Code != CodeUnknownTag || !reflect.DeepEqual(g.Problems[0].Details["tags"], []string{"ghost"}) {
		t.Errorf("get unknown: %+v", g.Problems)
	}
	if _, err := s.Get("missing"); code(err) != CodeNotFound {
		t.Errorf("get missing: %v", err)
	}
	// Broken entries can be dropped.
	if _, err := s.DropEntry("noparse"); err != nil || exists(s, "entries/noparse.md") {
		t.Errorf("drop broken: %v", err)
	}
}

func TestUppercaseFilenameNotServed(t *testing.T) {
	s := newStore(t)
	mustTag(t, s, "a")
	put(t, s, "entries/Foo.md", entryFile("d", "a", "body"))
	g, err := s.Get("foo")
	if err != nil || g.Entry != nil || g.Raw != nil || len(g.Problems) != 1 || g.Problems[0].Code != CodeStrayFile {
		t.Errorf("get foo: %+v %v", g, err)
	}
	if _, err := s.AddEntry("foo", "d", []string{"a"}, "b"); code(err) != CodeStrayFile {
		t.Errorf("add foo over Foo.md: %v", err)
	}
	if _, err := s.DropEntry("foo"); code(err) != CodeStrayFile {
		t.Errorf("drop foo: %v", err)
	}
	if !exists(s, "entries/Foo.md") {
		t.Error("Foo.md must be untouched")
	}
}

func TestSymlinks(t *testing.T) {
	s := newStore(t)
	mustTag(t, s, "a")
	outside := filepath.Join(t.TempDir(), "x.md")
	os.WriteFile(outside, []byte(entryFile("d", "a", "body")), 0o644)
	outDir := t.TempDir()
	os.WriteFile(filepath.Join(outDir, "in.md"), []byte(entryFile("d", "a", "body")), 0o644)
	if err := os.Symlink(outside, filepath.Join(s.root, "entries", "link.md")); err != nil {
		t.Skipf("symlinks unsupported: %v", err)
	}
	os.Symlink(outDir, filepath.Join(s.root, "entries", "dirlink"))
	os.Symlink(outside, filepath.Join(s.root, "tags", "tl.md"))

	res, _ := s.Search("", nil, DefaultLimit)
	if len(res.Entries) != 0 {
		t.Errorf("symlinked entries served: %v", ids(res))
	}
	want := []string{"entries/dirlink|stray_file", "entries/link.md|stray_file", "tags/tl.md|stray_file"}
	if got := problemKeys(res.Problems); !reflect.DeepEqual(got, want) {
		t.Errorf("problems = %v", got)
	}
	g, err := s.Get("link")
	if err != nil || g.Entry != nil || g.Raw != nil || g.Problems[0].Code != CodeStrayFile {
		t.Errorf("get link: %+v %v", g, err)
	}
	if g, _ := s.Get("dirlink/in"); g.Entry != nil || g.Problems[0].Code != CodeStrayFile {
		t.Errorf("get through dir link: %+v", g)
	}
	if _, err := s.AddEntry("dirlink/new", "d", []string{"a"}, "b"); code(err) != CodeStrayFile {
		t.Errorf("add through dir link: %v", err)
	}
	if _, err := os.Stat(filepath.Join(outDir, "new.md")); err == nil {
		t.Error("wrote through symlink")
	}
	if _, err := s.AddEntry("link", "d", []string{"a"}, "b"); code(err) != CodeStrayFile {
		t.Errorf("add over link: %v", err)
	}
	if _, err := s.DropTag("tl"); code(err) != CodeNotFound && code(err) != CodeStrayFile {
		t.Errorf("drop tag link: %v", err)
	}
	if err := os.Symlink(outDir, filepath.Join(s.root, "entries", "dl2")); err == nil {
		// tag via symlink is not an existing tag
		if _, err := s.AddEntry("e", "d", []string{"tl"}, "b"); code(err) != CodeUnknownTag {
			t.Errorf("symlinked tag counted as existing: %v", err)
		}
	}
}

func TestDotfilesIgnored(t *testing.T) {
	s := newStore(t)
	mustTag(t, s, "a")
	put(t, s, "entries/.DS_Store", "x")
	put(t, s, "entries/.tmp-123", "x")
	put(t, s, "entries/sub/.gitkeep", "")
	put(t, s, "tags/.DS_Store", "x")
	res, _ := s.Search("", nil, DefaultLimit)
	if len(res.Problems) != 0 || len(res.Entries) != 0 {
		t.Errorf("%+v", res)
	}
	// A dir holding only a dotfile survives entry_drop.
	s.AddEntry("sub/e", "d", []string{"a"}, "b")
	s.DropEntry("sub/e")
	if !exists(s, "entries/sub/.gitkeep") {
		t.Error("dir with dotfile removed")
	}
}

func TestEmptyDirCleanup(t *testing.T) {
	s := newStore(t)
	mustTag(t, s, "a")
	s.AddEntry("x/y/one", "d", []string{"a"}, "b")
	s.AddEntry("x/two", "d", []string{"a"}, "b")
	s.DropEntry("x/y/one")
	if exists(s, "entries/x/y") || !exists(s, "entries/x") {
		t.Error("expected y removed, x kept")
	}
	s.DropEntry("x/two")
	if exists(s, "entries/x") || !exists(s, "entries") {
		t.Error("expected x removed, entries kept")
	}
	if _, err := s.DropEntry("x/two"); code(err) != CodeNotFound {
		t.Errorf("drop again: %v", err)
	}
}

func TestNestingLimit(t *testing.T) {
	s := newStore(t)
	mustTag(t, s, "a")
	if _, err := s.AddEntry("a/b/c", "d", []string{"a"}, "b"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddEntry("a/b/c/d", "d", []string{"a"}, "b"); code(err) != CodeInvalid {
		t.Errorf("4 segments: %v", err)
	}
	res, _ := s.Search("", nil, DefaultLimit)
	if !reflect.DeepEqual(ids(res), []string{"a/b/c"}) || len(res.Problems) != 0 {
		t.Errorf("%+v", res)
	}
}

func TestTagLifecycle(t *testing.T) {
	s := newStore(t)
	if _, err := s.AddTag("Rel", " Releases \n"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddTag("rel", "x"); code(err) != CodeExists {
		t.Errorf("dup: %v", err)
	}
	if _, err := s.AddTag("bad_name", "x"); code(err) != CodeInvalid {
		t.Errorf("bad: %v", err)
	}
	data, _ := os.ReadFile(filepath.Join(s.root, "tags", "rel.md"))
	if string(data) != "Releases\n" {
		t.Errorf("bytes %q", data)
	}
	s.AddEntry("e1", "d", []string{"rel"}, "b")
	s.AddEntry("e2", "d", []string{"rel"}, "b")
	put(t, s, "entries/e3.md", "garbage") // broken entries do not count
	tl, _ := s.ListTags()
	if tl.Tags[0].Count != 2 {
		t.Errorf("count = %d", tl.Tags[0].Count)
	}
	_, err := s.DropTag("REL")
	var p *Problem
	if !errors.As(err, &p) || p.Code != CodeTagInUse || !reflect.DeepEqual(p.Details["entries"], []string{"e1", "e2"}) {
		t.Fatalf("tag_in_use: %v", err)
	}
	s.DropEntry("e1")
	s.DropEntry("e2")
	if r, err := s.DropTag("rel"); err != nil || r.Name != "rel" || exists(s, "tags/rel.md") {
		t.Errorf("drop: %v", err)
	}
	if _, err := s.DropTag("rel"); code(err) != CodeNotFound {
		t.Errorf("drop again: %v", err)
	}
	if _, err := s.UpdateTag("rel", "x"); code(err) != CodeNotFound {
		t.Errorf("update missing: %v", err)
	}
	// Broken tag: listed with null description, can be updated and dropped.
	s.DropEntry("e3")
	put(t, s, "tags/broken.md", "line1\nline2")
	tl, _ = s.ListTags()
	if len(tl.Tags) != 1 || tl.Tags[0].Description != nil || len(tl.Problems) != 1 {
		t.Errorf("%+v", tl)
	}
	if _, err := s.AddEntry("e", "d", []string{"broken"}, "b"); err != nil {
		t.Errorf("tag with broken description exists: %v", err)
	}
	s.DropEntry("e")
	if r, err := s.UpdateTag("broken", "fixed"); err != nil || !r.Changed {
		t.Errorf("fix: %v", err)
	}
	put(t, s, "tags/broken2.md", "")
	if _, err := s.DropTag("broken2"); err != nil {
		t.Errorf("drop broken tag: %v", err)
	}
}

func TestUnknownTags(t *testing.T) {
	s := newStore(t)
	mustTag(t, s, "a")
	_, err := s.AddEntry("e", "d", []string{"a", "ghost", "Ghost2"}, "b")
	var p *Problem
	if !errors.As(err, &p) || p.Code != CodeUnknownTag || !reflect.DeepEqual(p.Details["tags"], []string{"ghost", "ghost2"}) {
		t.Errorf("add: %v", err)
	}
	if _, err := s.Search("", []string{"ghost"}, DefaultLimit); code(err) != CodeUnknownTag {
		t.Errorf("search: %v", err)
	}
	if exists(s, "entries/e.md") {
		t.Error("entry written despite error")
	}
}

func TestSearch(t *testing.T) {
	s := newStore(t)
	mustTag(t, s, "alpha", "beta")
	add := func(id, d string, tags []string, body string) {
		t.Helper()
		if _, err := s.AddEntry(id, d, tags, body); err != nil {
			t.Fatal(err)
		}
	}
	add("b-entry", "about cats", []string{"alpha"}, "cat cat cat")
	add("a-entry", "about cats", []string{"alpha"}, "one cat")
	add("c-entry", "other", []string{"alpha", "beta"}, "dogs and CATS")
	add("z/dog", "d", []string{"beta"}, "dogs")

	tests := []struct {
		name  string
		q     string
		tags  []string
		limit int
		want  []string
		total int
		trunc bool
	}{
		{"no query by id", "", nil, DefaultLimit, []string{"a-entry", "b-entry", "c-entry", "z/dog"}, 4, false},
		{"score then id", "cat", nil, DefaultLimit, []string{"b-entry", "a-entry", "c-entry"}, 3, false}, // b: 1+1(desc)+3=... highest first
		{"case insensitive", "CATS", nil, DefaultLimit, []string{"a-entry", "b-entry", "c-entry"}, 3, false},
		{"all terms", "cats dogs", nil, DefaultLimit, []string{"c-entry"}, 1, false},
		{"matches id", "z/d", nil, DefaultLimit, []string{"z/dog"}, 1, false},
		{"matches tag name", "beta", nil, DefaultLimit, []string{"c-entry", "z/dog"}, 2, false},
		{"tag filter", "", []string{"Alpha", "beta"}, DefaultLimit, []string{"c-entry"}, 1, false},
		{"limit", "", nil, 2, []string{"a-entry", "b-entry"}, 4, true},
		{"limit exact", "", nil, 4, []string{"a-entry", "b-entry", "c-entry", "z/dog"}, 4, false},
		{"no match", "zzz", nil, DefaultLimit, []string{}, 0, false},
		{"whitespace query", "  \t ", nil, DefaultLimit, []string{"a-entry", "b-entry", "c-entry", "z/dog"}, 4, false},
	}
	for _, tt := range tests {
		r, err := s.Search(tt.q, tt.tags, tt.limit)
		if err != nil {
			t.Errorf("%s: %v", tt.name, err)
			continue
		}
		if !reflect.DeepEqual(ids(r), tt.want) || r.Total != tt.total || r.Truncated != tt.trunc {
			t.Errorf("%s: ids=%v total=%d trunc=%v", tt.name, ids(r), r.Total, r.Truncated)
		}
	}
	// "cat" occurrences: b-entry = desc 1 + body 3 = 4; a-entry = 1+1 = 2; c-entry = body "CATS" 1.
	for _, lim := range []int{-1, 201} {
		if _, err := s.Search("", nil, lim); code(err) != CodeInvalidInput {
			t.Errorf("limit %d: %v", lim, err)
		}
	}
	if r, err := s.Search("", nil, 200); err != nil || r.Problems == nil || r.Entries == nil {
		t.Errorf("limit 200: %v", err)
	}
}

func TestCaseInsensitiveInput(t *testing.T) {
	s := newStore(t)
	if _, err := s.AddTag("  MyTag ", "d"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddEntry("  Mixed/CASE.Id ", "d", []string{"MYTAG"}, "b"); err != nil {
		t.Fatal(err)
	}
	g, err := s.Get("MIXED/case.ID")
	if err != nil || g.Entry == nil || g.Entry.ID != "mixed/case.id" {
		t.Errorf("get: %+v %v", g, err)
	}
	if r, _ := s.Search("", []string{"mytag"}, DefaultLimit); r.Total != 1 {
		t.Error("search by tag")
	}
	if _, err := s.DropEntry("MIXED/CASE.ID"); err != nil {
		t.Error(err)
	}
	if _, err := s.DropTag("MYTAG"); err != nil {
		t.Error(err)
	}
}

func TestReadAcceptsNonCanonicalFiles(t *testing.T) {
	s := newStore(t)
	mustTag(t, s, "a", "b")
	put(t, s, "entries/crlf.md", "\xef\xbb\xbf---\r\ntags: [ B , a ]\r\ndescription:  hi \r\n---\r\n\r\nbody\r\n\r\n")
	g, err := s.Get("crlf")
	if err != nil || g.Entry == nil || g.Entry.Description != "hi" || g.Entry.Body != "body" ||
		!reflect.DeepEqual(g.Entry.Tags, []string{"a", "b"}) {
		t.Errorf("%+v %v", g, err)
	}
}
