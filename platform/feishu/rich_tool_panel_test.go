package feishu

import (
	"strings"
	"testing"
	"time"

	"github.com/chenhg5/cc-connect/core"
)

func TestCommandClassificationTarget(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"empty", "  ", ""},
		{"single command", "git status", "git status"},
		{"skips cd prefix", "cd /tmp/fdework\ngit log --oneline", "git log --oneline"},
		{"skips chained cd", "cd /tmp/x && go build ./...", "go build ./..."},
		{"skips env assignment", "CGO_ENABLED=0 go build ./...", "go build ./..."},
		{"skips export and source", "export PATH=/bin; source ~/.zshrc; make test", "make test"},
		{"keeps pipes intact", "cat notes.md | head -20", "cat notes.md | head -20"},
		{"all setup falls back to whole command", "cd /tmp/x && cd /tmp/y", "cd /tmp/x && cd /tmp/y"},
		{"strips wrapping quotes", `"git push origin main"`, "git push origin main"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := commandClassificationTarget(tc.in); got != tc.want {
				t.Fatalf("commandClassificationTarget(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// A multi-line script must be labelled and shown by its first meaningful
// segment: labelling it by `cd /tmp/fdework` is what made every row in the
// card read "Run command / cd /tmp/fdework".
func TestBuildToolDisplayClassifiesPastSetupPrefixes(t *testing.T) {
	display := buildToolDisplay("Bash", "cd /tmp/fdework\ngit log --oneline | head -20")
	if display.Title != "Git" {
		t.Fatalf("title = %q, want %q", display.Title, "Git")
	}
	if !strings.Contains(display.Detail, "git log --oneline") {
		t.Fatalf("detail should keep the whole script, got %q", display.Detail)
	}
}

func TestFormatToolCallDuration(t *testing.T) {
	tests := []struct {
		name string
		in   time.Duration
		want string
	}{
		{"unknown", 0, ""},
		{"negative", -time.Second, ""},
		{"sub-second", 710 * time.Millisecond, "710 ms"},
		{"just-under-a-second", 999 * time.Millisecond, "999 ms"},
		{"seconds", 2800 * time.Millisecond, "2.8 s"},
		{"round-seconds", 3 * time.Second, "3.0 s"},
		{"minutes", 72 * time.Second, "1m 12s"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatToolCallDuration(tt.in); got != tt.want {
				t.Fatalf("formatToolCallDuration(%v) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestFormatToolLaneSpan(t *testing.T) {
	tests := []struct {
		in   time.Duration
		want string
	}{
		{0, ""},
		{53 * time.Second, "53s"},
		{9*time.Minute + 53*time.Second, "9m 53s"},
		{time.Hour + 4*time.Minute, "1h 04m"},
	}
	for _, tt := range tests {
		if got := formatToolLaneSpan(tt.in); got != tt.want {
			t.Fatalf("formatToolLaneSpan(%v) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestToolStepStatusWord(t *testing.T) {
	ok, failed := true, false
	zero, one := 0, 1
	tests := []struct {
		name string
		step core.ToolStep
		want string
	}{
		{"running", core.ToolStep{Name: "Bash"}, "Running"},
		{"success-flag", core.ToolStep{Name: "Bash", Done: true, Success: &ok}, "Succeeded"},
		{"failure-flag", core.ToolStep{Name: "Bash", Done: true, Success: &failed}, "Failed"},
		{"exit-zero", core.ToolStep{Name: "Bash", Done: true, ExitCode: &zero}, "Succeeded"},
		{"exit-nonzero", core.ToolStep{Name: "Bash", Done: true, ExitCode: &one}, "Failed"},
		{"status-completed", core.ToolStep{Name: "Bash", Done: true, Status: "completed"}, "Succeeded"},
		{"status-failed", core.ToolStep{Name: "Bash", Done: true, Status: "failed"}, "Failed"},
		{"status-error", core.ToolStep{Name: "Bash", Done: true, Status: "error"}, "Failed"},
		{"done-without-signal", core.ToolStep{Name: "Bash", Done: true}, "Succeeded"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := toolStepStatusWord(tt.step); got != tt.want {
				t.Fatalf("toolStepStatusWord(%+v) = %q, want %q", tt.step, got, tt.want)
			}
		})
	}
}

func TestRichToolStepHeadline(t *testing.T) {
	ok := true
	one := 1

	got := richToolStepHeadline(core.ToolStep{
		Kind:     core.ToolStepKindTool,
		Name:     "Bash",
		Summary:  "echo hi",
		Duration: 710 * time.Millisecond,
		Success:  &ok,
		Done:     true,
	})
	if want := "Run command (710 ms) · Succeeded"; got != want {
		t.Fatalf("headline = %q, want %q", got, want)
	}

	got = richToolStepHeadline(core.ToolStep{
		Kind:     core.ToolStepKindTool,
		Name:     "Bash",
		Summary:  "go build ./...",
		Duration: 2800 * time.Millisecond,
		ExitCode: &one,
		Done:     true,
	})
	if want := "Build (2.8 s) · Failed · exit 1"; got != want {
		t.Fatalf("failed headline = %q, want %q", got, want)
	}

	// No timing yet (still running): the duration segment disappears rather
	// than rendering a misleading "0 ms".
	got = richToolStepHeadline(core.ToolStep{Kind: core.ToolStepKindTool, Name: "Bash", Summary: "sleep 5"})
	if want := "Wait · Running"; got != want {
		t.Fatalf("running headline = %q, want %q", got, want)
	}
}

func TestRichToolsLaneTitle(t *testing.T) {
	base := time.Date(2026, 8, 25, 10, 0, 0, 0, time.UTC)
	steps := []core.ToolStep{
		{Kind: core.ToolStepKindTool, Name: "Bash", StartedAt: base, Duration: time.Second, Done: true},
		{Kind: core.ToolStepKindTool, Name: "Read", StartedAt: base.Add(9*time.Minute + 50*time.Second), Duration: 3 * time.Second, Done: true},
	}

	if got, want := richToolsLaneTitle(steps, "zh"), "执行时间 9m 53s · 使用 2 个工具"; got != want {
		t.Fatalf("zh title = %q, want %q", got, want)
	}
	if got, want := richToolsLaneTitle(steps, "en"), "Elapsed 9m 53s · 2 tools"; got != want {
		t.Fatalf("en title = %q, want %q", got, want)
	}

	// Singular reads as "1 tool", not "1 tools".
	single := []core.ToolStep{{Kind: core.ToolStepKindTool, Name: "Bash", StartedAt: base, Duration: 2 * time.Second, Done: true}}
	if got, want := richToolsLaneTitle(single, "en"), "Elapsed 2s · 1 tool"; got != want {
		t.Fatalf("singular title = %q, want %q", got, want)
	}

	// Without timing data the title falls back to the plain counted label.
	untimed := []core.ToolStep{{Kind: core.ToolStepKindTool, Name: "Bash"}, {Kind: core.ToolStepKindTool, Name: "Read"}}
	if got, want := richToolsLaneTitle(untimed, "en"), "Tools (2)"; got != want {
		t.Fatalf("untimed title = %q, want %q", got, want)
	}
	if got, want := richToolsLaneTitle(untimed, "zh"), "工具 (2)"; got != want {
		t.Fatalf("untimed zh title = %q, want %q", got, want)
	}
}

func TestRichStepRowContentPrefersAgentDescription(t *testing.T) {
	ok := true
	content := richStepRowContent(core.ToolStep{
		Kind:        core.ToolStepKindTool,
		Name:        "Bash",
		Summary:     "cd /tmp/fdework\ngit log --oneline | head -20",
		Description: "List recent commits",
		Result:      "abc1234 fix: something",
		Duration:    438 * time.Millisecond,
		Success:     &ok,
		Done:        true,
	}, "en")

	want := "Git (438 ms) · Succeeded\nList recent commits\nResult\nabc1234 fix: something"
	if content != want {
		t.Fatalf("row content = %q, want %q", content, want)
	}
}

func TestRichStepRowContentCollapsesMultilineCommand(t *testing.T) {
	content := richStepRowContent(core.ToolStep{
		Kind:    core.ToolStepKindTool,
		Name:    "Bash",
		Summary: "cd /tmp/fdework\ncat notes.md | head -20",
	}, "en")

	lines := strings.Split(content, "\n")
	if len(lines) != 2 {
		t.Fatalf("multi-line command should collapse to one detail line, got %q", content)
	}
	if !strings.Contains(lines[1], "cat notes.md | head -20") {
		t.Fatalf("detail line should keep the informative tail, got %q", lines[1])
	}
	if strings.Contains(lines[1], "\n") {
		t.Fatalf("detail line should not contain newlines, got %q", lines[1])
	}
}

func TestRichStepRowContentKeepsHeadOfMultilineScript(t *testing.T) {
	script := "cd /tmp/biz_logs\npython3 - <<'EOF'\nimport pandas as pd\ndf = pd.read_csv('/tmp/biz_logs/skills.csv')\nprint(df.head())\nEOF"
	ok := true

	content := richStepRowContent(core.ToolStep{
		Kind:    core.ToolStepKindTool,
		Name:    "Bash",
		Summary: script,
		Success: &ok,
		Done:    true,
	}, "en")

	lines := strings.Split(content, "\n")
	if len(lines) != 2 {
		t.Fatalf("script row should render headline + one detail line, got %q", content)
	}
	// The head must be the line that says what ran, not the `cd` that opens it,
	// and the reader must be told the script continues.
	if !strings.Contains(lines[1], "python3 - <<'EOF'") {
		t.Fatalf("detail should lead with the meaningful line, got %q", lines[1])
	}
	if strings.Contains(lines[1], "import pandas") {
		t.Fatalf("detail should not flatten the script body onto one line, got %q", lines[1])
	}
	if !strings.Contains(lines[1], "(+5 lines)") {
		t.Fatalf("detail should report the hidden line count, got %q", lines[1])
	}

	zh := richStepRowContent(core.ToolStep{
		Kind:    core.ToolStepKindTool,
		Name:    "Bash",
		Summary: script,
		Success: &ok,
		Done:    true,
	}, "zh")
	if !strings.Contains(zh, "(+5 行)") {
		t.Fatalf("zh detail should localize the hidden line count, got %q", zh)
	}
}

func TestRichStepRowContentTruncatesResult(t *testing.T) {
	ok := true
	content := richStepRowContent(core.ToolStep{
		Kind:        core.ToolStepKindTool,
		Name:        "Bash",
		Summary:     "cat big.json",
		Description: "Dump a large file",
		Result:      strings.Repeat("x", richToolResultMaxLen+200),
		Duration:    3 * time.Second,
		Success:     &ok,
		Done:        true,
	}, "en")

	if !strings.Contains(content, richToolResultLabel) {
		t.Fatalf("row content should carry a result label, got %q", content)
	}
	if !strings.Contains(content, "…") {
		t.Fatalf("oversize result should be marked as truncated, got %q", content)
	}
	if len(content) > richToolResultMaxLen+200 {
		t.Fatalf("oversize result should be truncated, got %d chars", len(content))
	}
}

func TestRichStepRowContentThinkingUnchanged(t *testing.T) {
	content := richStepRowContent(core.ToolStep{
		Kind:    core.ToolStepKindThinking,
		Name:    "Thinking",
		Summary: "Inspecting event routing",
	}, "en")
	if content != "Inspecting event routing" {
		t.Fatalf("thinking row = %q, want the reasoning text only", content)
	}
}

func TestBuildRichCardToolPanelUsesNewStyle(t *testing.T) {
	ok := true
	base := time.Date(2026, 8, 25, 10, 0, 0, 0, time.UTC)
	steps := []core.ToolStep{
		{
			Kind:        core.ToolStepKindTool,
			Name:        "Bash",
			Summary:     "ls -la",
			Description: "List files in current directory",
			Result:      "total 48",
			StartedAt:   base,
			Duration:    710 * time.Millisecond,
			Success:     &ok,
			Done:        true,
		},
	}

	panels := collectCardPanels(t, buildRichCard(core.CardStatusDone, "zh", steps, "done", false, ""))
	if len(panels) != 1 {
		t.Fatalf("panel count = %d, want 1 tools panel: %#v", len(panels), panels)
	}
	if got, want := cardPanelTitle(panels[0]), "执行时间 1s · 使用 1 个工具"; got != want {
		t.Fatalf("panel title = %q, want %q", got, want)
	}
	for _, want := range []string{"710 ms", "Succeeded", "List files in current directory", richToolResultLabel, "total 48"} {
		if !panelContains(t, panels[0], want) {
			t.Fatalf("tools panel should contain %q: %#v", want, panels[0])
		}
	}
}

// Compaction drops rows to fit Feishu's payload limit. The panel header must
// still report the turn's real totals, or it disagrees with the engine-composed
// footer count on exactly the long turns where the reader needs it.
func TestBuildRichCardHeaderKeepsRealToolCountAfterCompaction(t *testing.T) {
	ok := true
	base := time.Date(2026, 8, 26, 10, 0, 0, 0, time.UTC)
	steps := make([]core.ToolStep, 0, 120)
	for i := range 120 {
		steps = append(steps, core.ToolStep{
			Kind:        core.ToolStepKindTool,
			Name:        "Bash",
			Summary:     strings.Repeat("x", 900),
			Description: strings.Repeat("y", 900),
			Result:      strings.Repeat("z", 900),
			StartedAt:   base.Add(time.Duration(i) * 10 * time.Second),
			Duration:    9 * time.Second,
			Success:     &ok,
			Done:        true,
		})
	}

	// A large body is what forces step compaction in practice: the rows are
	// already capped, so it is the markdown that pushes the payload over.
	body := strings.Repeat("lorem ipsum dolor sit amet. ", 800)
	card := buildRichCard(core.CardStatusDone, "zh", steps, body, false, "🔧 工具 120 次")

	if len(card) > maxRichCardJSONBytes {
		t.Fatalf("card should have been compacted under the size limit, got %d bytes", len(card))
	}
	if !strings.Contains(card, "使用 120 个工具") {
		t.Fatalf("compacted card header should still report 120 tools, got %q", firstPanelTitle(card))
	}
	// 119 gaps of 10s plus the last call's 9s.
	if !strings.Contains(card, "执行时间 19m 59s") {
		t.Fatalf("compacted card header should keep the full lane span, got %q", firstPanelTitle(card))
	}
}

func firstPanelTitle(card string) string {
	idx := strings.Index(card, "执行时间")
	if idx == -1 {
		return "<no tools panel title>"
	}
	end := strings.Index(card[idx:], `"`)
	if end == -1 {
		return card[idx:]
	}
	return card[idx : idx+end]
}
