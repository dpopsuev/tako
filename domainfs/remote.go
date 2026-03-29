// Package domainfs provides MCPRemoteFS, an fs.FS implementation that
// fetches files from a domain data MCP server via domain_read and
// domain_list tool calls.
package domainfs

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"strings"
	"time"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/dpopsuev/origami/subprocess"
)

// MCPRemoteFS implements fs.FS by delegating to MCP domain_read and
// domain_list tools. It is the sole fs.FS implementation for remote
// domain data in the container architecture.
type MCPRemoteFS struct {
	caller  subprocess.ToolCaller
	timeout time.Duration
}

// New creates an MCPRemoteFS backed by the given ToolCaller.
func New(caller subprocess.ToolCaller) *MCPRemoteFS {
	return &MCPRemoteFS{caller: caller, timeout: 10 * time.Second}
}

// WithTimeout returns a copy with the given per-call timeout.
func (r *MCPRemoteFS) WithTimeout(d time.Duration) *MCPRemoteFS {
	return &MCPRemoteFS{caller: r.caller, timeout: d}
}

// Open implements fs.FS. It calls domain_read for files and
// domain_list for directories.
func (r *MCPRemoteFS) Open(name string) (fs.File, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrInvalid}
	}

	ctx, cancel := context.WithTimeout(context.Background(), r.timeout)
	defer cancel()

	result, err := r.caller.CallTool(ctx, "domain_read", map[string]any{"path": name})
	if err != nil {
		return nil, &fs.PathError{Op: "open", Path: name, Err: err}
	}

	text := firstText(result)

	if result.IsError {
		// The path may be a directory -- try domain_list as fallback.
		dirFile, dirErr := r.openDir(ctx, name)
		if dirErr == nil {
			return dirFile, nil
		}
		// Both failed. Return the most informative error.
		if isNotFoundError(text) {
			return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
		}
		return nil, &fs.PathError{Op: "open", Path: name, Err: fmt.Errorf("%w: %s", ErrRemoteCall, text)}
	}

	return newMemFile(name, []byte(text)), nil
}

func (r *MCPRemoteFS) openDir(ctx context.Context, name string) (fs.File, error) {
	result, err := r.caller.CallTool(ctx, "domain_list", map[string]any{"path": name})
	if err != nil {
		return nil, &fs.PathError{Op: "open", Path: name, Err: err}
	}
	if result.IsError {
		text := firstText(result)
		if isNotFoundError(text) {
			return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
		}
		return nil, &fs.PathError{Op: "open", Path: name, Err: fmt.Errorf("%w: %s", ErrRemoteCall, text)}
	}

	var entries []dirEntryJSON
	if err := json.Unmarshal([]byte(firstText(result)), &entries); err != nil {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fmt.Errorf("unmarshal dir: %w", err)}
	}

	fsDirEntries := make([]fs.DirEntry, len(entries))
	for i, e := range entries {
		fsDirEntries[i] = &staticDirEntry{name: e.Name, isDir: e.IsDir}
	}
	return &memDir{name: name, entries: fsDirEntries}, nil
}

type dirEntryJSON struct {
	Name  string `json:"name"`
	IsDir bool   `json:"is_dir"`
}

func isNotFoundError(text string) bool {
	lower := strings.ToLower(text)
	return strings.Contains(lower, "not found") ||
		strings.Contains(lower, "does not exist") ||
		strings.Contains(lower, "no such file") ||
		strings.Contains(lower, "file does not exist")
}

func firstText(result *sdkmcp.CallToolResult) string {
	for _, c := range result.Content {
		if tc, ok := c.(*sdkmcp.TextContent); ok {
			return tc.Text
		}
	}
	return ""
}

// --- in-memory file ---

type memFile struct {
	name   string
	data   []byte
	reader *bytes.Reader
}

func newMemFile(name string, data []byte) *memFile {
	return &memFile{name: name, data: data, reader: bytes.NewReader(data)}
}

func (f *memFile) Stat() (fs.FileInfo, error) {
	return &staticFileInfo{name: baseName(f.name), size: int64(len(f.data))}, nil
}

func (f *memFile) Read(b []byte) (int, error) { return f.reader.Read(b) }
func (f *memFile) Close() error               { return nil }

// --- in-memory directory ---

type memDir struct {
	name    string
	entries []fs.DirEntry
	offset  int
}

func (d *memDir) Stat() (fs.FileInfo, error) {
	return &staticFileInfo{name: baseName(d.name), isDir: true}, nil
}

func (d *memDir) Read([]byte) (int, error) {
	return 0, &fs.PathError{Op: "read", Path: d.name, Err: ErrIsADirectory}
}
func (d *memDir) Close() error { return nil }

func (d *memDir) ReadDir(n int) ([]fs.DirEntry, error) {
	if n <= 0 {
		entries := d.entries[d.offset:]
		d.offset = len(d.entries)
		return entries, nil
	}
	if d.offset >= len(d.entries) {
		return nil, io.EOF
	}
	end := d.offset + n
	if end > len(d.entries) {
		end = len(d.entries)
	}
	entries := d.entries[d.offset:end]
	d.offset = end
	if d.offset >= len(d.entries) {
		return entries, io.EOF
	}
	return entries, nil
}

// --- static fs.FileInfo ---

type staticFileInfo struct {
	name  string
	size  int64
	isDir bool
}

func (fi *staticFileInfo) Name() string { return fi.name }
func (fi *staticFileInfo) Size() int64  { return fi.size }
func (fi *staticFileInfo) Mode() fs.FileMode {
	if fi.isDir {
		return fs.ModeDir | 0o555
	}
	return 0o444
}
func (fi *staticFileInfo) ModTime() time.Time { return time.Time{} }
func (fi *staticFileInfo) IsDir() bool        { return fi.isDir }
func (fi *staticFileInfo) Sys() any           { return nil }

// --- static fs.DirEntry ---

type staticDirEntry struct {
	name  string
	isDir bool
}

func (e *staticDirEntry) Name() string { return e.name }
func (e *staticDirEntry) IsDir() bool  { return e.isDir }
func (e *staticDirEntry) Type() fs.FileMode {
	if e.isDir {
		return fs.ModeDir
	}
	return 0
}
func (e *staticDirEntry) Info() (fs.FileInfo, error) {
	return &staticFileInfo{name: e.name, isDir: e.isDir}, nil
}

func baseName(path string) string {
	if path == "." {
		return "."
	}
	i := strings.LastIndex(path, "/")
	if i >= 0 {
		return path[i+1:]
	}
	return path
}

// Compile-time interface checks.
var (
	_ fs.FS          = (*MCPRemoteFS)(nil)
	_ fs.File        = (*memFile)(nil)
	_ fs.ReadDirFile = (*memDir)(nil)
)
