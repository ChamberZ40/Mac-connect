package core

import (
	"strings"
	"testing"
)

func TestIsFileMutatingTool(t *testing.T) {
	mutating := []string{
		"Edit", "edit", "MultiEdit", "Write", "write_file", "NotebookEdit",
		"apply_patch", "applyPatch", "Apply-Patch", "replace", "create_file",
		"update_file", "fs/write_text_file", "str_replace_editor",
	}
	for _, name := range mutating {
		if !isFileMutatingTool(name) {
			t.Errorf("isFileMutatingTool(%q) = false, want true", name)
		}
	}
	readOnly := []string{"Read", "Bash", "Grep", "Glob", "WebFetch", "TodoWrite", "", "Task"}
	for _, name := range readOnly {
		if isFileMutatingTool(name) {
			t.Errorf("isFileMutatingTool(%q) = true, want false", name)
		}
	}
}

func TestMutatedFilePath(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "empty", input: "  "},
		{name: "bare path", input: "/repo/core/engine.go", want: "/repo/core/engine.go"},
		{name: "path with spaces", input: "/repo/my docs/a.md", want: "/repo/my docs/a.md"},
		{name: "json file_path", input: `{"file_path":"/repo/a.go","old_string":"x"}`, want: "/repo/a.go"},
		{name: "json notebook_path", input: `{"notebook_path":"/repo/n.ipynb"}`, want: "/repo/n.ipynb"},
		{name: "json target_file", input: `{"target_file":"/repo/b.go"}`, want: "/repo/b.go"},
		{name: "json without a path key", input: `{"content":"hello"}`},
		{name: "malformed json", input: `{"file_path":`},
		{name: "multi-line patch body", input: "--- a/x.go\n+++ b/x.go\n@@"},
		{name: "oversized blob", input: strings.Repeat("a", maxMutatedPathLen+1)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := mutatedFilePath(tt.input); got != tt.want {
				t.Errorf("mutatedFilePath(%.40q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestTrackMutatedFile(t *testing.T) {
	var files []string
	files = trackMutatedFile(files, "Read", "/repo/a.go") // read-only: ignored
	files = trackMutatedFile(files, "Edit", "/repo/a.go")
	files = trackMutatedFile(files, "Bash", "go test ./...") // ignored
	files = trackMutatedFile(files, "Write", `{"file_path":"/repo/b.go"}`)
	files = trackMutatedFile(files, "Edit", "/repo/a.go") // duplicate: ignored
	files = trackMutatedFile(files, "Edit", "  ")         // no path: ignored

	want := []string{"/repo/a.go", "/repo/b.go"}
	if len(files) != len(want) {
		t.Fatalf("files = %v, want %v", files, want)
	}
	for i := range want {
		if files[i] != want[i] {
			t.Errorf("files[%d] = %q, want %q", i, files[i], want[i])
		}
	}
}

// TestTrackMutatedFile_DoesNotMutateCaller guards the immutability contract: a
// snapshot already handed to a footer build must not gain entries afterwards.
func TestTrackMutatedFile_DoesNotMutateCaller(t *testing.T) {
	original := trackMutatedFile(nil, "Edit", "/repo/a.go")
	snapshot := original
	updated := trackMutatedFile(original, "Edit", "/repo/b.go")

	if len(snapshot) != 1 || snapshot[0] != "/repo/a.go" {
		t.Errorf("snapshot = %v, want unchanged single entry", snapshot)
	}
	if len(updated) != 2 {
		t.Fatalf("updated = %v, want 2 entries", updated)
	}
	if &snapshot[0] == &updated[0] {
		t.Error("updated shares backing array with the caller's slice")
	}
}

func TestTrackMutatedFile_CapsTrackedFiles(t *testing.T) {
	var files []string
	for i := 0; i < maxTrackedMutatedFiles+10; i++ {
		files = trackMutatedFile(files, "Edit", "/repo/f"+string(rune('a'+i%26))+string(rune('a'+i/26))+".go")
	}
	if len(files) != maxTrackedMutatedFiles {
		t.Errorf("len(files) = %d, want cap %d", len(files), maxTrackedMutatedFiles)
	}
}
