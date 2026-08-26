package core

import (
	"encoding/json"
	"strings"
	"unicode"
)

// Tracking of files an agent wrote during a turn, for the 🔧 footer line.
//
// The engine cannot ask an agent "what did you change?" — no interface reports
// it — but every agent streams an EventToolUse per invocation, and for
// file-writing tools the human-readable ToolInput summary is (or contains) the
// target path. Recognizing that subset of tools is enough to report changed
// files without any agent-specific plumbing.
//
// Tool names, not agent names, are matched here: core stays agnostic about which
// agent produced the event (see also the ToolName == "AskUserQuestion" handling
// in the permission path).

const (
	// maxTrackedMutatedFiles bounds the per-turn list. A turn that writes more
	// files than this still reports the count of what was tracked; the footer
	// only ever spells out a handful of names anyway.
	maxTrackedMutatedFiles = 64

	// maxMutatedPathLen rejects tool inputs too long to plausibly be a single
	// path (patch bodies, serialized diffs).
	maxMutatedPathLen = 512
)

// fileMutatingTools holds the normalized names (see normalizeToolName) of tools
// that write to disk, across the agents cc-connect supports: Claude Code
// (Edit/Write/MultiEdit/NotebookEdit), Codex (apply_patch), Gemini CLI
// (write_file/replace), and the edit/create/update variants used by
// Cursor/opencode/ACP backends.
//
// Read-only tools are absent by design: only writes belong on the footer line.
var fileMutatingTools = map[string]bool{
	"edit":                    true,
	"multiedit":               true,
	"write":                   true,
	"writefile":               true,
	"notebookedit":            true,
	"applypatch":              true,
	"patch":                   true,
	"replace":                 true,
	"createfile":              true,
	"editfile":                true,
	"updatefile":              true,
	"writetextfile":           true,
	"strreplaceeditor":        true,
	"strreplacebasededittool": true,
}

// mutatedPathKeys lists the JSON fields agents use for the target path when the
// tool-input summary is a serialized object rather than a bare path.
var mutatedPathKeys = []string{
	"file_path", "notebook_path", "path", "target_file", "absolute_path", "filename", "file",
}

// normalizeToolName reduces a tool name to a comparable key: the last
// namespace segment ("fs/write_text_file" → "write_text_file", as ACP-style
// backends emit), lowercased with separators dropped so that "apply_patch",
// "applyPatch" and "Apply-Patch" all compare equal.
func normalizeToolName(name string) string {
	if idx := strings.LastIndexAny(name, "/:."); idx >= 0 {
		name = name[idx+1:]
	}
	var b strings.Builder
	for _, r := range name {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(unicode.ToLower(r))
		}
	}
	return b.String()
}

// isFileMutatingTool reports whether a tool invocation writes to disk.
func isFileMutatingTool(name string) bool {
	return fileMutatingTools[normalizeToolName(name)]
}

// mutatedFilePath extracts the written path from a tool-input summary.
//
// Two shapes are accepted, matching what agents actually emit: a bare path
// ("/repo/core/engine.go", as claudecode's summarizeInput produces for
// Edit/Write), or a JSON object carrying one of mutatedPathKeys. Anything else
// — multi-line patch bodies, prose, oversized blobs — yields "" so the footer
// reports the tool count without inventing a filename.
func mutatedFilePath(toolInput string) string {
	s := strings.TrimSpace(toolInput)
	if s == "" || len(s) > maxMutatedPathLen {
		return ""
	}
	if strings.HasPrefix(s, "{") {
		var m map[string]any
		if err := json.Unmarshal([]byte(s), &m); err != nil {
			return ""
		}
		for _, key := range mutatedPathKeys {
			if v, ok := m[key].(string); ok {
				if v = strings.TrimSpace(v); v != "" {
					return v
				}
			}
		}
		return ""
	}
	if strings.ContainsAny(s, "\n\r") {
		return ""
	}
	return s
}

// trackMutatedFile returns files plus the path written by this tool event.
//
// Returns the input slice unchanged when the tool does not write, when no path
// could be extracted, when the path is already tracked, or when the cap is
// reached. Otherwise a new slice is returned — the caller's backing array is
// never mutated, so a snapshot handed to the footer builder cannot change
// underneath it.
func trackMutatedFile(files []string, toolName, toolInput string) []string {
	if !isFileMutatingTool(toolName) {
		return files
	}
	path := mutatedFilePath(toolInput)
	if path == "" {
		return files
	}
	for _, f := range files {
		if f == path {
			return files
		}
	}
	if len(files) >= maxTrackedMutatedFiles {
		return files
	}
	out := make([]string, len(files), len(files)+1)
	copy(out, files)
	return append(out, path)
}
