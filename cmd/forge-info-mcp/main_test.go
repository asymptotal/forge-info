package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func exec(args ...string) (int, string, string) {
	var out, errb bytes.Buffer
	code := run(args, strings.NewReader(""), &out, &errb)
	return code, out.String(), errb.String()
}

func TestRun(t *testing.T) {
	dir := t.TempDir()
	if c, _, e := exec(); c != 2 || e == "" {
		t.Errorf("no args: %d %q", c, e)
	}
	if c, out, _ := exec("version"); c != 0 || strings.TrimSpace(out) == "" {
		t.Errorf("version: %d %q", c, out)
	}
	if c, _, _ := exec("version", "x"); c != 2 {
		t.Errorf("version with args: %d", c)
	}
	if c, _, _ := exec("bogus"); c != 2 {
		t.Errorf("bogus: %d", c)
	}
	if c, _, e := exec("init", dir); c != 0 {
		t.Errorf("init: %d %q", c, e)
	}
	if _, err := os.Stat(filepath.Join(dir, ".info", "entries", ".gitkeep")); err != nil {
		t.Error(err)
	}
	if c, _, _ := exec("init", dir); c != 2 {
		t.Errorf("second init: %d", c)
	}
	if c, _, _ := exec("serve", "--root", filepath.Join(dir, "nope")); c != 2 {
		t.Errorf("serve bad root: %d", c)
	}
	if c, out, _ := exec("serve", "--root", filepath.Join(dir, ".info")); c != 0 || out != "" {
		t.Errorf("serve on EOF: %d %q", c, out)
	}
}
