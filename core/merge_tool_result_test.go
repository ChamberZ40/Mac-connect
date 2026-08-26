package core

import (
	"testing"
	"time"
)

// Claude Code's tool_result carries no tool name, only a tool_use_id. Matching
// such a result by name found nothing and appended a phantom step, so every
// call rendered twice: a real row stuck on "Running" plus a "Tool" row holding
// the result. That is what made the panel's tool count disagree with the
// footer's.
func TestMergeRichToolResult_NamelessResultResolvesPendingStep(t *testing.T) {
	now := time.Now()
	steps := []ToolStep{
		{Kind: ToolStepKindTool, Name: "Bash", CallID: "toolu_1", Summary: "ls", StartedAt: now.Add(-time.Second)},
	}

	got := mergeRichToolResult(steps, Event{Type: EventToolResult, ToolID: "toolu_1"}, "total 48", 500, now)

	if len(got) != 1 {
		t.Fatalf("result should resolve the pending step, not append one: %+v", got)
	}
	if got[0].Name != "Bash" {
		t.Fatalf("step name = %q, want Bash", got[0].Name)
	}
	if !got[0].Done || got[0].Result != "total 48" {
		t.Fatalf("step should be resolved with its result, got %+v", got[0])
	}
	if got[0].Duration <= 0 {
		t.Fatalf("resolved step should carry a duration, got %v", got[0].Duration)
	}
}

// With no ids at all (older agents), a nameless result still belongs to the
// pending call rather than to a new phantom step.
func TestMergeRichToolResult_NamelessResultWithoutIDsUsesPendingStep(t *testing.T) {
	now := time.Now()
	steps := []ToolStep{
		{Kind: ToolStepKindTool, Name: "Read", Summary: "main.go", StartedAt: now.Add(-time.Second)},
	}

	got := mergeRichToolResult(steps, Event{Type: EventToolResult}, "package main", 500, now)

	if len(got) != 1 || got[0].Name != "Read" || !got[0].Done {
		t.Fatalf("nameless result should resolve the pending Read step, got %+v", got)
	}
}

// Parallel calls: ids keep results with their own call even when they land out
// of order. Without ids the panel would credit the wrong row.
func TestMergeRichToolResult_MatchesByCallIDOutOfOrder(t *testing.T) {
	now := time.Now()
	steps := []ToolStep{
		{Kind: ToolStepKindTool, Name: "Bash", CallID: "a", Summary: "sleep 3", StartedAt: now.Add(-3 * time.Second)},
		{Kind: ToolStepKindTool, Name: "Bash", CallID: "b", Summary: "echo hi", StartedAt: now.Add(-time.Second)},
	}

	got := mergeRichToolResult(steps, Event{Type: EventToolResult, ToolID: "a"}, "slept", 500, now)

	if len(got) != 2 {
		t.Fatalf("no step should be appended, got %+v", got)
	}
	if got[0].Result != "slept" || !got[0].Done {
		t.Fatalf("result should land on call a, got %+v", got[0])
	}
	if got[1].Done || got[1].Result != "" {
		t.Fatalf("call b must stay pending, got %+v", got[1])
	}
}

// Sequential same-name calls: the first result must resolve the first call, not
// overwrite the newest one, or the earlier row never leaves "Running".
func TestMergeRichToolResult_SequentialSameNameCallsResolveInOrder(t *testing.T) {
	now := time.Now()
	steps := []ToolStep{
		{Kind: ToolStepKindTool, Name: "Bash", Summary: "step one", StartedAt: now.Add(-2 * time.Second)},
		{Kind: ToolStepKindTool, Name: "Bash", Summary: "step two", StartedAt: now.Add(-time.Second)},
	}

	got := mergeRichToolResult(steps, Event{Type: EventToolResult, ToolName: "Bash"}, "one done", 500, now)
	if len(got) != 2 {
		t.Fatalf("no step should be appended, got %+v", got)
	}
	if got[0].Result != "one done" {
		t.Fatalf("first result should resolve the oldest pending call, got %+v", got[0])
	}

	got = mergeRichToolResult(got, Event{Type: EventToolResult, ToolName: "Bash"}, "two done", 500, now)
	if len(got) != 2 {
		t.Fatalf("second result should still not append, got %+v", got)
	}
	if got[1].Result != "two done" {
		t.Fatalf("second result should resolve the second call, got %+v", got[1])
	}
}

// A genuinely orphan result (no pending call anywhere) still shows up, because
// dropping tool output silently is worse than an extra row.
func TestMergeRichToolResult_OrphanResultStillAppends(t *testing.T) {
	now := time.Now()
	steps := []ToolStep{
		{Kind: ToolStepKindTool, Name: "Bash", Summary: "ls", Done: true, Result: "already"},
	}

	got := mergeRichToolResult(steps, Event{Type: EventToolResult, ToolName: "Grep"}, "found", 500, now)

	if len(got) != 2 {
		t.Fatalf("orphan result should append a step, got %+v", got)
	}
	if got[1].Name != "Grep" || got[1].Result != "found" {
		t.Fatalf("appended step = %+v", got[1])
	}
	if got[0].Result != "already" {
		t.Fatalf("resolved step must not be overwritten, got %+v", got[0])
	}
}

// An agent that streams several result chunks for one call must keep updating
// that call's row rather than growing a new row per chunk.
func TestMergeRichToolResult_RepeatChunksUpdateSameStep(t *testing.T) {
	now := time.Now()
	steps := []ToolStep{
		{Kind: ToolStepKindTool, Name: "Bash", Summary: "tail -f log", StartedAt: now.Add(-time.Second)},
	}

	steps = mergeRichToolResult(steps, Event{Type: EventToolResult, ToolName: "Bash"}, "chunk one", 500, now)
	steps = mergeRichToolResult(steps, Event{Type: EventToolResult, ToolName: "Bash"}, "chunk two", 500, now)

	if len(steps) != 1 {
		t.Fatalf("repeat chunks should not append steps, got %+v", steps)
	}
	if steps[0].Result != "chunk two" {
		t.Fatalf("latest chunk should win, got %q", steps[0].Result)
	}
}

// Thinking rows are not tool calls and must never absorb a tool result.
func TestMergeRichToolResult_IgnoresThinkingSteps(t *testing.T) {
	now := time.Now()
	steps := []ToolStep{{Kind: ToolStepKindThinking, Name: "Thinking", Summary: "planning"}}

	got := mergeRichToolResult(steps, Event{Type: EventToolResult}, "output", 500, now)

	if len(got) != 2 {
		t.Fatalf("result should append its own tool step, got %+v", got)
	}
	if got[0].Kind != ToolStepKindThinking || got[0].Result != "" {
		t.Fatalf("thinking row must stay untouched, got %+v", got[0])
	}
}
