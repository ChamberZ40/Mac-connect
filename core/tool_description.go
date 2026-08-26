package core

import (
	"encoding/json"
	"strings"
)

// ToolCallDescription extracts the agent-authored natural-language intent of a
// tool call from its raw input — "List recent commits" rather than
// "cd /tmp/work\ngit log --oneline".
//
// Tools that carry one put it in a "description" field alongside the machine
// argument (Claude Code's Bash does; MCP tools commonly do). The argument alone
// makes a poor label, because a multi-line script's first line is usually a
// `cd`, so renderers prefer this when it is present.
//
// input may be the decoded argument map or the JSON text of one, since agents
// differ in which of the two they hand over. Anything else, absent field, or
// unparseable JSON yields "", leaving callers on their raw argument summary.
func ToolCallDescription(input any) string {
	switch value := input.(type) {
	case map[string]any:
		return descriptionFromMap(value)
	case string:
		trimmed := strings.TrimSpace(value)
		if !strings.HasPrefix(trimmed, "{") {
			return ""
		}
		var decoded map[string]any
		if err := json.Unmarshal([]byte(trimmed), &decoded); err != nil {
			return ""
		}
		return descriptionFromMap(decoded)
	default:
		return ""
	}
}

func descriptionFromMap(input map[string]any) string {
	description, _ := input["description"].(string)
	return strings.TrimSpace(description)
}
