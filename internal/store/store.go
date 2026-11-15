// Package store implements the file-backed knowledge store (TDD §1-§5).
package store

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	entriesDir   = "entries"
	tagsDir      = "tags"
	defaultLimit = 50
	maxLimit     = 200
)

// Result and data types. JSON tags follow TDD §5.

type Summary struct {
	ID          string   `json:"id"`
	Description string   `json:"description"`
	Tags        []string `json:"tags"`
}

type Entry struct {
	Summary
	Body string `json:"body"`
}

// Tag.Description is nil (JSON null) when the tag file is broken.
type Tag struct {
	Name        string  `json:"name"`
	Description *string `json:"description"`
	Count       int     `json:"count"`
}

type SearchResult struct {
	Total     int       `json:"total"`
	Truncated bool      `json:"truncated"`
	Entries   []Summary `json:"entries"`
	Problems  []Problem `json:"problems"`
}

type GetResult struct {
	Entry    *Entry    `json:"entry"`
	Problems []Problem `json:"problems"`
	Raw      *string   `json:"raw,omitempty"`
}

type UpdateEntryResult struct {
	Changed bool  `json:"changed"`
	Entry   Entry `json:"entry"`
}

type DropEntryResult struct {
	ID string `json:"id"`
}

type ListTagsResult struct {
	Tags     []Tag     `json:"tags"`
	Problems []Problem `json:"problems"`
}

type UpdateTagResult struct {
	Changed bool `json:"changed"`
	Tag     Tag  `json:"tag"`
}

type DropTagResult struct {
	Name string `json:"name"`
}

// Store is a store rooted at a .info directory. It holds no state besides the path.
type Store struct{ root string }

// Find walks up from startDir to the first directory containing .info and
// returns the path of that .info directory.
func Find(startDir string) (string, error) {
	dir, err := filepath.Abs(startDir)
	if err != nil {
		return "", err
	}
	for {
		root := filepath.Join(dir, ".info")
		if fi, err := os.Stat(root); err == nil && fi.IsDir() {
			return root, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("no .info directory found from %s upwards", startDir)
		}
		dir = parent
	}
}

// Init creates <dir>/.info with empty entries/ and tags/. It fails if .info exists.
func Init(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	root := filepath.Join(dir, ".info")
	if err := os.Mkdir(root, 0o755); err != nil {
		return err
	}
	for _, sub := range []string{entriesDir, tagsDir} {
		if err := os.Mkdir(filepath.Join(root, sub), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(root, sub, ".gitkeep"), nil, 0o644); err != nil {
			return err
		}
	}
	return nil
}

// Open opens the store at root (the .info directory).
func Open(root string) (*Store, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	for _, sub := range []string{entriesDir, tagsDir} {
		fi, err := os.Lstat(filepath.Join(root, sub))
		if err != nil || !fi.IsDir() {
			return nil, fmt.Errorf("%s is not a store: missing %s/ directory", root, sub)
		}
	}
	return &Store{root: root}, nil
}

// Root returns the .info directory.
func (s *Store) Root() string { return s.root }

// ---- paths -----------------------------------------------------------------

func relPath(area, name string) string { return area + "/" + name + ".md" }

// resolve maps a validated name to a file path under area, never following
// symlinks and confirming exact on-disk names via directory listings. It
// reports whether the file exists as a regular file. Anything else on the way
// (symlink, wrong type, differently-cased name) is a stray_file problem.
func (s *Store) resolve(area, name string) (string, bool, *Problem) {
	segs := strings.Split(name, "/")
	dir := filepath.Join(s.root, area)
	rel := area
	for i, seg := range segs {
		last := i == len(segs)-1
		want := seg
		if last {
			want += ".md"
		}
		rel += "/" + want
		des, err := os.ReadDir(dir)
		if errors.Is(err, fs.ErrNotExist) {
			break
		}
		if err != nil {
			return "", false, ioProblem(rel, err)
		}
		found := ""
		for _, de := range des {
			if de.Name() == want {
				found = want
				break
			}
			if found == "" && strings.EqualFold(de.Name(), want) {
				found = de.Name()
			}
		}
		if found == "" {
			break
		}
		if found != want {
			return "", false, newProblem(CodeStrayFile, strings.TrimSuffix(rel, want)+found,
				"name differs from %q only in case", want)
		}
		dir = filepath.Join(dir, want)
		fi, err := os.Lstat(dir)
		if err != nil {
			return "", false, ioProblem(rel, err)
		}
		switch {
		case last && fi.Mode().IsRegular():
			return dir, true, nil
		case last, !fi.IsDir():
			return "", false, newProblem(CodeStrayFile, rel, "not a regular file or directory (symlinks are not followed)")
		}
	}
	return filepath.Join(append([]string{s.root, area}, segs...)...) + ".md", false, nil
}

// writeFile writes data atomically: temp file in the target dir, then rename.
func writeFile(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	_, err = f.Write(data)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Chmod(tmp, 0o644)
	}
	if err == nil {
		err = os.Rename(tmp, path)
	}
	if err != nil {
		os.Remove(tmp)
	}
	return err
}

// ---- scanning ----------------------------------------------------------------

func stray(rel string) Problem {
	return *newProblem(CodeStrayFile, rel, "not a regular .md file with a valid name, or nested too deeply")
}

// listTags returns the names of existing tags (regular files with valid names)
// and a stray_file problem for everything else.
func (s *Store) listTags() ([]string, []Problem, error) {
	des, err := os.ReadDir(filepath.Join(s.root, tagsDir))
	if err != nil {
		return nil, nil, ioProblem(tagsDir, err)
	}
	var names []string
	var probs []Problem
	for _, de := range des {
		n := de.Name()
		if strings.HasPrefix(n, ".") {
			continue
		}
		if stem, ok := strings.CutSuffix(n, ".md"); ok && de.Type().IsRegular() {
			if name, p := checkName(tagName, "name", stem); p == nil && name == stem {
				names = append(names, name)
				continue
			}
		}
		probs = append(probs, stray(tagsDir+"/"+n))
	}
	return names, probs, nil
}

func (s *Store) tagSet() (map[string]bool, error) {
	names, _, err := s.listTags()
	if err != nil {
		return nil, err
	}
	set := make(map[string]bool, len(names))
	for _, n := range names {
		set[n] = true
	}
	return set, nil
}

// listEntryIDs walks entries/ and returns the ids of candidate files plus
// stray_file problems.
func (s *Store) listEntryIDs(segs []string, ids *[]string, probs *[]Problem) error {
	rel := strings.Join(append([]string{entriesDir}, segs...), "/")
	des, err := os.ReadDir(filepath.Join(append([]string{s.root, entriesDir}, segs...)...))
	if err != nil {
		return ioProblem(rel, err)
	}
	for _, de := range des {
		n := de.Name()
		if strings.HasPrefix(n, ".") {
			continue
		}
		sub := append(append([]string{}, segs...), n)
		switch {
		case de.Type().IsDir():
			// A directory this deep could not hold a valid id.
			if id, p := checkName(entryID, "id", strings.Join(sub, "/")); p != nil || id != strings.Join(sub, "/") || len(sub) >= maxSegments {
				*probs = append(*probs, stray(rel+"/"+n))
				continue
			}
			if err := s.listEntryIDs(sub, ids, probs); err != nil {
				return err
			}
		case de.Type().IsRegular() && strings.HasSuffix(n, ".md"):
			sub[len(sub)-1] = strings.TrimSuffix(n, ".md")
			want := strings.Join(sub, "/")
			if id, p := checkName(entryID, "id", want); p != nil || id != want {
				*probs = append(*probs, stray(rel+"/"+n))
				continue
			}
			*ids = append(*ids, want)
		default:
			*probs = append(*probs, stray(rel+"/"+n))
		}
	}
	return nil
}

// validateEntry parses and validates file contents; it reports every problem.
func validateEntry(id string, data []byte, tags map[string]bool) (Entry, []Problem) {
	path := relPath(entriesDir, id)
	fm, body, p := parseEntry(data)
	if p != nil {
		return Entry{}, withPath(path, p)
	}
	var probs []Problem
	desc, p := checkText(entryDesc, fm.description)
	if p != nil {
		probs = append(probs, withPath(path, p)...)
	}
	names, tp := checkTags(fm.tags, func(n string) bool { return tags[n] })
	for _, q := range tp {
		q.Path = path
		probs = append(probs, q)
	}
	body, p = checkText(entryBody, body)
	if p != nil {
		probs = append(probs, withPath(path, p)...)
	}
	if len(probs) > 0 {
		return Entry{}, probs
	}
	return Entry{Summary{id, desc, names}, body}, nil
}

func sortProblems(ps []Problem) []Problem {
	if ps == nil {
		ps = []Problem{}
	}
	sort.SliceStable(ps, func(i, j int) bool {
		if ps[i].Path != ps[j].Path {
			return ps[i].Path < ps[j].Path
		}
		return ps[i].Code < ps[j].Code
	})
	return ps
}

// scanEntries returns all valid entries (sorted by id) and the problems of the rest.
func (s *Store) scanEntries(tags map[string]bool) ([]Entry, []Problem, error) {
	var ids []string
	var probs []Problem
	if err := s.listEntryIDs(nil, &ids, &probs); err != nil {
		return nil, nil, err
	}
	sort.Strings(ids)
	var entries []Entry
	for _, id := range ids {
		path := relPath(entriesDir, id)
		data, err := os.ReadFile(filepath.Join(s.root, filepath.FromSlash(path)))
		if err != nil {
			probs = append(probs, *ioProblem(path, err))
			continue
		}
		e, ps := validateEntry(id, data, tags)
		if ps != nil {
			probs = append(probs, ps...)
			continue
		}
		entries = append(entries, e)
	}
	return entries, probs, nil
}

func (s *Store) readTag(name string) (*string, []Problem, error) {
	path := relPath(tagsDir, name)
	data, err := os.ReadFile(filepath.Join(s.root, tagsDir, name+".md"))
	if err != nil {
		return nil, nil, ioProblem(path, err)
	}
	raw, p := parseTag(data)
	if p == nil {
		var desc string
		if desc, p = checkText(tagDesc, raw); p == nil {
			return &desc, nil, nil
		}
	}
	return nil, withPath(path, p), nil
}

func (s *Store) tagCount(name string) (int, error) {
	set, err := s.tagSet()
	if err != nil {
		return 0, err
	}
	entries, _, err := s.scanEntries(set)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, e := range entries {
		if hasTag(e.Tags, name) {
			n++
		}
	}
	return n, nil
}

func hasTag(tags []string, name string) bool {
	for _, t := range tags {
		if t == name {
			return true
		}
	}
	return false
}

// ---- entry tools ---------------------------------------------------------------

// Search implements entry_search. limit 0 means the default (50).
func (s *Store) Search(query string, tags []string, limit int) (*SearchResult, error) {
	if limit == 0 {
		limit = defaultLimit
	}
	if limit < 1 || limit > maxLimit {
		return nil, newProblem(CodeInvalidInput, "", "limit must be between 1 and %d", maxLimit)
	}
	set, err := s.tagSet()
	if err != nil {
		return nil, err
	}
	var want []string
	if len(tags) > 0 {
		var ps []Problem
		if want, ps = checkTags(tags, func(n string) bool { return set[n] }); len(ps) > 0 {
			return nil, &ps[0]
		}
	}
	entries, probs, err := s.scanEntries(set)
	if err != nil {
		return nil, err
	}
	_, tagProbs, err := s.listTags()
	if err != nil {
		return nil, err
	}
	probs = append(probs, tagProbs...)
	probs = append(probs, s.tagFileProblems(set)...)

	terms := strings.Fields(strings.ToLower(query))
	type hit struct {
		sum   Summary
		score int
	}
	var hits []hit
	for _, e := range entries {
		ok := true
		for _, t := range want {
			ok = ok && hasTag(e.Tags, t)
		}
		if !ok {
			continue
		}
		score := 0
		fields := append([]string{e.ID, e.Description, e.Body}, e.Tags...)
		for _, t := range terms {
			n := 0
			for _, f := range fields {
				n += strings.Count(strings.ToLower(f), t)
			}
			if n == 0 {
				ok = false
				break
			}
			score += n
		}
		if ok {
			hits = append(hits, hit{e.Summary, score})
		}
	}
	sort.SliceStable(hits, func(i, j int) bool {
		if hits[i].score != hits[j].score {
			return hits[i].score > hits[j].score
		}
		return hits[i].sum.ID < hits[j].sum.ID
	})
	res := &SearchResult{Total: len(hits), Entries: []Summary{}, Problems: sortProblems(probs)}
	if len(hits) > limit {
		hits, res.Truncated = hits[:limit], true
	}
	for _, h := range hits {
		res.Entries = append(res.Entries, h.sum)
	}
	return res, nil
}

// tagFileProblems reports tag files whose description is broken.
func (s *Store) tagFileProblems(set map[string]bool) []Problem {
	var out []Problem
	for name := range set {
		if _, ps, err := s.readTag(name); err != nil {
			out = append(out, *ioProblemFrom(err))
		} else {
			out = append(out, ps...)
		}
	}
	return out
}

func ioProblemFrom(err error) *Problem {
	var p *Problem
	if errors.As(err, &p) {
		return p
	}
	return ioProblem("", err)
}

// Get implements entry_get. It checks only the requested file.
func (s *Store) Get(id string) (*GetResult, error) {
	id, p := checkName(entryID, "id", id)
	if p != nil {
		return nil, p
	}
	path, exists, p := s.resolve(entriesDir, id)
	if p != nil && p.Code == CodeStrayFile {
		return &GetResult{Problems: []Problem{*p}}, nil
	} else if p != nil {
		return nil, p
	}
	rel := relPath(entriesDir, id)
	if !exists {
		return nil, newProblem(CodeNotFound, rel, "entry %q not found", id)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, ioProblem(rel, err)
	}
	set, err := s.tagSet()
	if err != nil {
		return nil, err
	}
	e, probs := validateEntry(id, data, set)
	if probs != nil {
		raw := string(data)
		return &GetResult{Problems: sortProblems(probs), Raw: &raw}, nil
	}
	return &GetResult{Entry: &e, Problems: []Problem{}}, nil
}

// prepEntry validates in the order name, tags, description, body and renders.
func (s *Store) prepEntry(id, description string, tags []string, body string) (Entry, []byte, error) {
	id, p := checkName(entryID, "id", id)
	if p != nil {
		return Entry{}, nil, p
	}
	set, err := s.tagSet()
	if err != nil {
		return Entry{}, nil, err
	}
	names, ps := checkTags(tags, func(n string) bool { return set[n] })
	if len(ps) > 0 {
		return Entry{}, nil, &ps[0]
	}
	if description, p = checkText(entryDesc, description); p != nil {
		return Entry{}, nil, p
	}
	if body, p = checkText(entryBody, body); p != nil {
		return Entry{}, nil, p
	}
	return Entry{Summary{id, description, names}, body}, renderEntry(description, names, body), nil
}

// AddEntry implements entry_add.
func (s *Store) AddEntry(id, description string, tags []string, body string) (*Entry, error) {
	e, data, err := s.prepEntry(id, description, tags, body)
	if err != nil {
		return nil, err
	}
	path, exists, p := s.resolve(entriesDir, e.ID)
	if p != nil {
		return nil, p
	}
	if exists {
		return nil, newProblem(CodeExists, relPath(entriesDir, e.ID), "entry %q already exists", e.ID)
	}
	if err := writeFile(path, data); err != nil {
		return nil, ioProblem(relPath(entriesDir, e.ID), err)
	}
	return &e, nil
}

// UpdateEntry implements entry_update.
func (s *Store) UpdateEntry(id, description string, tags []string, body string) (*UpdateEntryResult, error) {
	e, data, err := s.prepEntry(id, description, tags, body)
	if err != nil {
		return nil, err
	}
	rel := relPath(entriesDir, e.ID)
	path, exists, p := s.resolve(entriesDir, e.ID)
	if p != nil {
		return nil, p
	}
	if !exists {
		return nil, newProblem(CodeNotFound, rel, "entry %q not found", e.ID)
	}
	old, err := os.ReadFile(path)
	if err != nil {
		return nil, ioProblem(rel, err)
	}
	if string(old) == string(data) {
		return &UpdateEntryResult{Changed: false, Entry: e}, nil
	}
	if err := writeFile(path, data); err != nil {
		return nil, ioProblem(rel, err)
	}
	return &UpdateEntryResult{Changed: true, Entry: e}, nil
}

// DropEntry implements entry_drop and removes directories left empty.
func (s *Store) DropEntry(id string) (*DropEntryResult, error) {
	id, p := checkName(entryID, "id", id)
	if p != nil {
		return nil, p
	}
	rel := relPath(entriesDir, id)
	path, exists, p := s.resolve(entriesDir, id)
	if p != nil {
		return nil, p
	}
	if !exists {
		return nil, newProblem(CodeNotFound, rel, "entry %q not found", id)
	}
	if err := os.Remove(path); err != nil {
		return nil, ioProblem(rel, err)
	}
	stop := filepath.Join(s.root, entriesDir)
	for dir := filepath.Dir(path); dir != stop && strings.HasPrefix(dir, stop); dir = filepath.Dir(dir) {
		if os.Remove(dir) != nil { // not empty (dotfiles count as content)
			break
		}
	}
	return &DropEntryResult{ID: id}, nil
}

// ---- tag tools -----------------------------------------------------------------

// ListTags implements tag_list.
func (s *Store) ListTags() (*ListTagsResult, error) {
	names, probs, err := s.listTags()
	if err != nil {
		return nil, err
	}
	set := make(map[string]bool, len(names))
	for _, n := range names {
		set[n] = true
	}
	entries, eprobs, err := s.scanEntries(set)
	if err != nil {
		return nil, err
	}
	probs = append(probs, eprobs...)
	counts := map[string]int{}
	for _, e := range entries {
		for _, t := range e.Tags {
			counts[t]++
		}
	}
	sort.Strings(names)
	res := &ListTagsResult{Tags: []Tag{}}
	for _, n := range names {
		desc, ps, err := s.readTag(n)
		if err != nil {
			return nil, err
		}
		probs = append(probs, ps...)
		res.Tags = append(res.Tags, Tag{n, desc, counts[n]})
	}
	res.Problems = sortProblems(probs)
	return res, nil
}

func (s *Store) prepTag(name, description string) (string, string, error) {
	name, p := checkName(tagName, "name", name)
	if p != nil {
		return "", "", p
	}
	if description, p = checkText(tagDesc, description); p != nil {
		return "", "", p
	}
	return name, description, nil
}

// AddTag implements tag_add.
func (s *Store) AddTag(name, description string) (*Tag, error) {
	name, description, err := s.prepTag(name, description)
	if err != nil {
		return nil, err
	}
	path, exists, p := s.resolve(tagsDir, name)
	if p != nil {
		return nil, p
	}
	if exists {
		return nil, newProblem(CodeExists, relPath(tagsDir, name), "tag %q already exists", name)
	}
	if err := writeFile(path, renderTag(description)); err != nil {
		return nil, ioProblem(relPath(tagsDir, name), err)
	}
	n, err := s.tagCount(name)
	if err != nil {
		return nil, err
	}
	return &Tag{name, &description, n}, nil
}

// UpdateTag implements tag_update.
func (s *Store) UpdateTag(name, description string) (*UpdateTagResult, error) {
	name, description, err := s.prepTag(name, description)
	if err != nil {
		return nil, err
	}
	rel := relPath(tagsDir, name)
	path, exists, p := s.resolve(tagsDir, name)
	if p != nil {
		return nil, p
	}
	if !exists {
		return nil, newProblem(CodeNotFound, rel, "tag %q not found", name)
	}
	old, err := os.ReadFile(path)
	if err != nil {
		return nil, ioProblem(rel, err)
	}
	data := renderTag(description)
	changed := string(old) != string(data)
	if changed {
		if err := writeFile(path, data); err != nil {
			return nil, ioProblem(rel, err)
		}
	}
	n, err := s.tagCount(name)
	if err != nil {
		return nil, err
	}
	return &UpdateTagResult{changed, Tag{name, &description, n}}, nil
}

// DropTag implements tag_drop.
func (s *Store) DropTag(name string) (*DropTagResult, error) {
	name, p := checkName(tagName, "name", name)
	if p != nil {
		return nil, p
	}
	rel := relPath(tagsDir, name)
	path, exists, p := s.resolve(tagsDir, name)
	if p != nil {
		return nil, p
	}
	if !exists {
		return nil, newProblem(CodeNotFound, rel, "tag %q not found", name)
	}
	set, err := s.tagSet()
	if err != nil {
		return nil, err
	}
	entries, _, err := s.scanEntries(set)
	if err != nil {
		return nil, err
	}
	var users []string
	for _, e := range entries {
		if hasTag(e.Tags, name) {
			users = append(users, e.ID)
		}
	}
	if len(users) > 0 {
		return nil, &Problem{Code: CodeTagInUse, Path: rel,
			Message: fmt.Sprintf("tag %q is used by %d entries", name, len(users)),
			Details: map[string]any{"entries": users}}
	}
	if err := os.Remove(path); err != nil {
		return nil, ioProblem(rel, err)
	}
	return &DropTagResult{Name: name}, nil
}
