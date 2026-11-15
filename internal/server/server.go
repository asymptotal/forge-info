// Package server exposes the store as MCP tools. It is a thin wrapper:
// every handler calls the store and returns the store's result.
package server

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/asymptotal/forge-info/internal/store"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// New builds the MCP server for st. Instructions are a snapshot taken now.
func New(st *store.Store, version string) (*mcp.Server, error) {
	instr, err := instructions(st)
	if err != nil {
		return nil, err
	}
	s := mcp.NewServer(&mcp.Implementation{Name: "infomcp", Version: version},
		&mcp.ServerOptions{Instructions: instr})

	mcp.AddTool(s, &mcp.Tool{Name: "entry_search", Description: descSearch}, h(func(in searchIn) (*store.SearchResult, error) {
		limit := store.DefaultLimit
		if in.Limit != nil {
			limit = *in.Limit
		}
		return st.Search(in.Query, in.Tags, limit)
	}))
	mcp.AddTool(s, &mcp.Tool{Name: "entry_get", Description: descGet}, h(func(in idIn) (*store.GetResult, error) {
		return st.Get(in.ID)
	}))
	mcp.AddTool(s, &mcp.Tool{Name: "entry_add", Description: descAdd}, h(func(in entryIn) (*store.Entry, error) {
		return st.AddEntry(in.ID, in.Description, in.Tags, in.Body)
	}))
	mcp.AddTool(s, &mcp.Tool{Name: "entry_update", Description: descUpdate}, h(func(in entryIn) (*store.UpdateEntryResult, error) {
		return st.UpdateEntry(in.ID, in.Description, in.Tags, in.Body)
	}))
	mcp.AddTool(s, &mcp.Tool{Name: "entry_drop", Description: descDrop}, h(func(in idIn) (*store.DropEntryResult, error) {
		return st.DropEntry(in.ID)
	}))
	mcp.AddTool(s, &mcp.Tool{Name: "tag_list", Description: descTagList}, h(func(in struct{}) (*store.ListTagsResult, error) {
		return st.ListTags()
	}))
	mcp.AddTool(s, &mcp.Tool{Name: "tag_add", Description: descTagAdd}, h(func(in tagIn) (*store.Tag, error) {
		return st.AddTag(in.Name, in.Description)
	}))
	mcp.AddTool(s, &mcp.Tool{Name: "tag_update", Description: descTagUpdate}, h(func(in tagIn) (*store.UpdateTagResult, error) {
		return st.UpdateTag(in.Name, in.Description)
	}))
	mcp.AddTool(s, &mcp.Tool{Name: "tag_drop", Description: descTagDrop}, h(func(in nameIn) (*store.DropTagResult, error) {
		return st.DropTag(in.Name)
	}))
	return s, nil
}

type searchIn struct {
	Query string   `json:"query,omitempty" jsonschema:"Whitespace-separated terms; every term must appear (case-insensitive substring) in the id, description, a tag name or the body. Omit to list all."`
	Tags  []string `json:"tags,omitempty" jsonschema:"Only entries having all of these tags. An unknown tag is an error."`
	Limit *int     `json:"limit,omitempty" jsonschema:"Maximum entries returned, 1 to 200. Default 50."`
}

type idIn struct {
	ID string `json:"id" jsonschema:"Entry id: 1 to 3 slash-separated segments of lowercase letters, digits and . _ - (max 100 chars), e.g. session/state-transitions."`
}

type entryIn struct {
	ID          string   `json:"id" jsonschema:"Entry id: 1 to 3 slash-separated segments of lowercase letters, digits and . _ - (max 100 chars), e.g. session/state-transitions."`
	Description string   `json:"description" jsonschema:"One line, 1 to 100 characters, written so an agent can decide from it whether to read the entry."`
	Tags        []string `json:"tags" jsonschema:"1 to 5 distinct existing tags."`
	Body        string   `json:"body" jsonschema:"Markdown, 1 to 2000 characters, no headings. One concept per entry; record why, not what."`
}

type tagIn struct {
	Name        string `json:"name" jsonschema:"Tag name: lowercase letters and digits, hyphen-separated, max 32 chars."`
	Description string `json:"description" jsonschema:"One line, 1 to 200 characters."`
}

type nameIn struct {
	Name string `json:"name" jsonschema:"Tag name."`
}

// h adapts a store call to an SDK tool handler. Errors become a Go error whose
// text is the JSON problem, so the SDK sets isError with that text.
func h[In, Out any](f func(In) (*Out, error)) mcp.ToolHandlerFor[In, *Out] {
	return func(_ context.Context, _ *mcp.CallToolRequest, in In) (*mcp.CallToolResult, *Out, error) {
		out, err := f(in)
		if err != nil {
			return nil, nil, toolError(err)
		}
		return nil, out, nil
	}
}

type jsonError string

func (e jsonError) Error() string { return string(e) }

func toolError(err error) error {
	var p *store.Problem
	if !errors.As(err, &p) {
		p = &store.Problem{Code: store.CodeIOError, Message: err.Error()}
	}
	b, merr := json.Marshal(p)
	if merr != nil {
		return err
	}
	return jsonError(b)
}
