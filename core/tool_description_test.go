package core

import "testing"

func TestToolCallDescription(t *testing.T) {
	tests := []struct {
		name string
		in   any
		want string
	}{
		{"nil", nil, ""},
		{"map with description", map[string]any{"command": "git log", "description": "List recent commits"}, "List recent commits"},
		{"map without description", map[string]any{"command": "git log"}, ""},
		{"map with non-string description", map[string]any{"description": 42}, ""},
		{"map with blank description", map[string]any{"description": "   "}, ""},
		{"description is trimmed", map[string]any{"description": "  Build the binary\n"}, "Build the binary"},
		{"json arguments", `{"command":"go build ./...","description":"Build the binary"}`, "Build the binary"},
		{"json arguments without description", `{"command":"go build ./..."}`, ""},
		{"plain string argument", "go build ./...", ""},
		{"malformed json", `{"description":`, ""},
		{"json array", `["go","build"]`, ""},
		{"unsupported type", 42, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := ToolCallDescription(tc.in); got != tc.want {
				t.Fatalf("ToolCallDescription(%#v) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
