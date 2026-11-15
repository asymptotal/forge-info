package server

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/asymptotal/forge-info/internal/store"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func setup(t *testing.T) (*mcp.ClientSession, string) {
	t.Helper()
	dir := t.TempDir()
	if err := store.Init(dir); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(dir, ".info")
	write := func(p, s string) {
		if err := os.WriteFile(filepath.Join(root, p), []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("tags/session.md", "Session handling.\n")
	write("tags/broken.md", "line one\nline two\n")
	st, err := store.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	srv, err := New(st, "test")
	if err != nil {
		t.Fatal(err)
	}
	ct, stt := mcp.NewInMemoryTransports()
	ctx := context.Background()
	if _, err := srv.Connect(ctx, stt, nil); err != nil {
		t.Fatal(err)
	}
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "c", Version: "1"}, nil).Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cs.Close() })
	return cs, root
}

func call(t *testing.T, cs *mcp.ClientSession, name string, args any) *mcp.CallToolResult {
	t.Helper()
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("%s: protocol error: %v", name, err)
	}
	return res
}

func ok(t *testing.T, cs *mcp.ClientSession, name string, args any) map[string]any {
	t.Helper()
	res := call(t, cs, name, args)
	if res.IsError {
		t.Fatalf("%s: unexpected error: %v", name, res.Content)
	}
	m, isMap := res.StructuredContent.(map[string]any)
	if !isMap {
		t.Fatalf("%s: no structured content: %#v", name, res.StructuredContent)
	}
	return m
}

func errCode(t *testing.T, cs *mcp.ClientSession, name string, args any) string {
	t.Helper()
	res := call(t, cs, name, args)
	if !res.IsError {
		t.Fatalf("%s: expected isError", name)
	}
	if res.StructuredContent != nil {
		t.Fatalf("%s: unexpected structured content", name)
	}
	var p struct{ Code, Message string }
	if err := json.Unmarshal([]byte(res.Content[0].(*mcp.TextContent).Text), &p); err != nil {
		t.Fatalf("%s: error text not JSON: %v", name, err)
	}
	return p.Code
}

func TestListAndInstructions(t *testing.T) {
	cs, _ := setup(t)
	tools, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, tl := range tools.Tools {
		names = append(names, tl.Name)
	}
	want := "entry_add entry_drop entry_get entry_search entry_update tag_add tag_drop tag_list tag_update"
	if got := strings.Join(sortStrings(names), " "); got != want {
		t.Fatalf("tools = %s", got)
	}
	instr := cs.InitializeResult().Instructions
	for _, s := range []string{"session: 0, Session handling.", "broken", "snapshot"} {
		if !strings.Contains(instr, s) {
			t.Errorf("instructions missing %q:\n%s", s, instr)
		}
	}
}

func sortStrings(s []string) []string {
	for i := range s {
		for j := i + 1; j < len(s); j++ {
			if s[j] < s[i] {
				s[i], s[j] = s[j], s[i]
			}
		}
	}
	return s
}

func TestHappyPaths(t *testing.T) {
	cs, _ := setup(t)
	e := map[string]any{"id": "a/b", "description": "Desc.", "tags": []string{"session"}, "body": "Hello world"}
	if got := ok(t, cs, "entry_add", e); got["id"] != "a/b" || got["body"] != "Hello world" {
		t.Fatalf("add: %v", got)
	}
	if got := ok(t, cs, "entry_update", e); got["changed"] != false {
		t.Fatalf("update same: %v", got)
	}
	e["body"] = "Changed"
	if got := ok(t, cs, "entry_update", e); got["changed"] != true {
		t.Fatalf("update: %v", got)
	}
	got := ok(t, cs, "entry_search", map[string]any{"query": "changed", "tags": []string{"session"}, "limit": 10})
	if got["total"] != float64(1) || got["truncated"] != false {
		t.Fatalf("search: %v", got)
	}
	ok(t, cs, "entry_search", map[string]any{})
	got = ok(t, cs, "entry_get", map[string]any{"id": "a/b"})
	if got["entry"].(map[string]any)["body"] != "Changed" {
		t.Fatalf("get: %v", got)
	}
	got = ok(t, cs, "tag_add", map[string]any{"name": "new", "description": "New tag."})
	if got["name"] != "new" || got["count"] != float64(0) {
		t.Fatalf("tag_add: %v", got)
	}
	if got = ok(t, cs, "tag_update", map[string]any{"name": "new", "description": "Newer."}); got["changed"] != true {
		t.Fatalf("tag_update: %v", got)
	}
	if got = ok(t, cs, "tag_list", map[string]any{}); len(got["tags"].([]any)) != 3 {
		t.Fatalf("tag_list: %v", got)
	}
	if got = ok(t, cs, "tag_drop", map[string]any{"name": "new"}); got["name"] != "new" {
		t.Fatalf("tag_drop: %v", got)
	}
	if got = ok(t, cs, "entry_drop", map[string]any{"id": "a/b"}); got["id"] != "a/b" {
		t.Fatalf("entry_drop: %v", got)
	}
}

func TestErrors(t *testing.T) {
	cs, _ := setup(t)
	e := map[string]any{"id": "x", "description": "D.", "tags": []string{"nope"}, "body": "b"}
	if c := errCode(t, cs, "entry_add", e); c != "unknown_tag" {
		t.Errorf("got %s", c)
	}
	if c := errCode(t, cs, "entry_get", map[string]any{"id": "missing"}); c != "not_found" {
		t.Errorf("got %s", c)
	}
	e["tags"] = []string{"session"}
	ok(t, cs, "entry_add", e)
	if c := errCode(t, cs, "entry_add", e); c != "exists" {
		t.Errorf("got %s", c)
	}
	if c := errCode(t, cs, "tag_drop", map[string]any{"name": "session"}); c != "tag_in_use" {
		t.Errorf("got %s", c)
	}
	if c := errCode(t, cs, "entry_search", map[string]any{"limit": 500}); c != "invalid_input" {
		t.Errorf("got %s", c)
	}
}

func TestMissingArgument(t *testing.T) {
	cs, _ := setup(t)
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "entry_get", Arguments: map[string]any{}})
	if err == nil && !res.IsError {
		t.Fatal("expected error for missing id")
	}
}

func TestBrokenEntryAndTag(t *testing.T) {
	cs, root := setup(t)
	if err := os.WriteFile(filepath.Join(root, "entries", "bad.md"), []byte("not frontmatter\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := ok(t, cs, "entry_get", map[string]any{"id": "bad"})
	if got["entry"] != nil || got["raw"] != "not frontmatter\n" || len(got["problems"].([]any)) == 0 {
		t.Fatalf("get broken: %v", got)
	}
	tags := ok(t, cs, "tag_list", map[string]any{})
	var sawNull bool
	for _, x := range tags["tags"].([]any) {
		m := x.(map[string]any)
		if m["name"] == "broken" && m["description"] == nil {
			sawNull = true
		}
	}
	if !sawNull || len(tags["problems"].([]any)) == 0 {
		t.Fatalf("tag_list: %v", tags)
	}
	got = ok(t, cs, "entry_search", map[string]any{})
	if len(got["problems"].([]any)) == 0 {
		t.Fatalf("search should report problems: %v", got)
	}
}
