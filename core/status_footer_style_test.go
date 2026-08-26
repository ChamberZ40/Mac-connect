package core

import (
	"strings"
	"testing"
	"time"
)

func TestCtxGauge(t *testing.T) {
	cases := map[int]string{
		-5:  "▱▱▱▱▱▱▱▱▱▱",
		0:   "▱▱▱▱▱▱▱▱▱▱",
		1:   "▰▱▱▱▱▱▱▱▱▱", // non-zero always fills at least one cell
		9:   "▰▱▱▱▱▱▱▱▱▱",
		43:  "▰▰▰▰▱▱▱▱▱▱",
		50:  "▰▰▰▰▰▱▱▱▱▱",
		100: "▰▰▰▰▰▰▰▰▰▰",
		120: "▰▰▰▰▰▰▰▰▰▰",
	}
	for pct, want := range cases {
		if got := ctxGauge(pct); got != want {
			t.Errorf("ctxGauge(%d) = %q, want %q", pct, got, want)
		}
	}
}

func TestContextUsedPercent(t *testing.T) {
	tests := []struct {
		name    string
		usage   *ContextUsage
		wantPct int
		wantOK  bool
	}{
		{name: "nil usage", usage: nil},
		{name: "no window", usage: &ContextUsage{UsedTokens: 100}},
		{name: "no usage numbers", usage: &ContextUsage{ContextWindow: 1000}},
		{
			name:    "used tokens",
			usage:   &ContextUsage{UsedTokens: 430, ContextWindow: 1000},
			wantPct: 43, wantOK: true,
		},
		{
			name:    "falls back to total tokens",
			usage:   &ContextUsage{TotalTokens: 430, ContextWindow: 1000},
			wantPct: 43, wantOK: true,
		},
		{
			name:    "rounds to nearest",
			usage:   &ContextUsage{UsedTokens: 41772, ContextWindow: 1_000_000},
			wantPct: 4, wantOK: true,
		},
		{
			name:    "clamps over-full window",
			usage:   &ContextUsage{UsedTokens: 2000, ContextWindow: 1000},
			wantPct: 100, wantOK: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pct, ok := contextUsedPercent(tt.usage)
			if ok != tt.wantOK || pct != tt.wantPct {
				t.Errorf("contextUsedPercent() = (%d, %v), want (%d, %v)", pct, ok, tt.wantPct, tt.wantOK)
			}
		})
	}
}

func TestFooterModelLine(t *testing.T) {
	usage := &ContextUsage{UsedTokens: 430, ContextWindow: 1000}
	tests := []struct {
		name  string
		model string
		usage *ContextUsage
		want  string
	}{
		{
			name:  "model and gauge",
			model: "claude-opus-5",
			usage: usage,
			want:  "🧠 claude-opus-5 · ctx ▰▰▰▰▱▱▱▱▱▱43%",
		},
		{
			name:  "no usage keeps model only",
			model: "codex",
			want:  "🧠 codex",
		},
		{
			name:  "gauge only when model unknown",
			usage: usage,
			want:  "🧠 ctx ▰▰▰▰▱▱▱▱▱▱43%",
		},
		{name: "nothing known"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := footerModelLine(tt.model, tt.usage); got != tt.want {
				t.Errorf("footerModelLine() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestFooterModelLine_ModelAndGaugeOnly pins the decision that neither raw token
// counts nor reasoning effort belong on the rich footer's model line.
func TestFooterModelLine_ModelAndGaugeOnly(t *testing.T) {
	got := footerModelLine("claude-opus-5", &ContextUsage{
		InputTokens:              1,
		OutputTokens:             168,
		CacheCreationInputTokens: 971,
		CachedInputTokens:        40800,
		UsedTokens:               41772,
		ContextWindow:            1_000_000,
	})
	for _, banned := range []string{"out ", "in 1", "cw ", "cr "} {
		if strings.Contains(got, banned) {
			t.Errorf("footerModelLine() = %q, must not contain token segment %q", got, banned)
		}
	}
	if want := "🧠 claude-opus-5 · ctx ▰▱▱▱▱▱▱▱▱▱4%"; got != want {
		t.Errorf("footerModelLine() = %q, want %q", got, want)
	}
}

// TestComposeRichStatusFooter_OmitsReasoningEffort keeps the effort segment out
// of the rich footer even when the session reports one.
func TestComposeRichStatusFooter_OmitsReasoningEffort(t *testing.T) {
	e := newClaudeFooterEngine()
	e.i18n = NewI18n(LangEnglish)
	session := &controllableAgentSession{
		model:           "claude-opus-5",
		workDir:         "/tmp/ws",
		reasoningEffort: "xhigh",
		contextUsage:    &ContextUsage{UsedTokens: 430, ContextWindow: 1000},
	}
	got := e.composeRichStatusFooter(false, time.Now(), nil, session, "/tmp/ws", turnFooterStats{})
	if strings.Contains(got, "xhigh") {
		t.Errorf("footer = %q, want no reasoning-effort segment", got)
	}
}

func TestFooterWorkdirLine(t *testing.T) {
	if got, want := footerWorkdirLine("~/code/cc-connect"), "📁 ~/code/cc-connect"; got != want {
		t.Errorf("footerWorkdirLine() = %q, want %q", got, want)
	}
	if got := footerWorkdirLine("   "); got != "" {
		t.Errorf("footerWorkdirLine(blank) = %q, want empty", got)
	}
}

func TestFooterToolsLine(t *testing.T) {
	i18nEN := NewI18n(LangEnglish)
	i18nZH := NewI18n(LangChinese)
	tests := []struct {
		name  string
		i18n  *I18n
		stats turnFooterStats
		want  string
	}{
		{
			name:  "no tools means no line",
			i18n:  i18nEN,
			stats: turnFooterStats{ToolCount: 0, Files: []string{"a.go"}},
			want:  "",
		},
		{
			name:  "tools without writes",
			i18n:  i18nEN,
			stats: turnFooterStats{ToolCount: 4},
			want:  "🔧 4 tools",
		},
		{
			name:  "tools with writes (en)",
			i18n:  i18nEN,
			stats: turnFooterStats{ToolCount: 4, Files: []string{"/repo/core/engine.go", "/repo/config/config.go"}},
			want:  "🔧 4 tools · 2 changed: engine.go, config.go",
		},
		{
			name:  "tools with writes (zh)",
			i18n:  i18nZH,
			stats: turnFooterStats{ToolCount: 4, Files: []string{"/repo/core/engine.go", "/repo/config/config.go"}},
			want:  "🔧 工具 4 次 · 改动 2: engine.go, config.go",
		},
		{
			name:  "nil i18n is safe",
			stats: turnFooterStats{ToolCount: 4},
			want:  "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := footerToolsLine(tt.i18n, tt.stats); got != tt.want {
				t.Errorf("footerToolsLine() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFormatFooterFileNames(t *testing.T) {
	tests := []struct {
		name  string
		files []string
		max   int
		want  string
	}{
		{name: "empty", max: 3},
		{name: "zero max", files: []string{"a.go"}, max: 0},
		{name: "base names only", files: []string{"a/b.go", "c/d.go"}, max: 3, want: "b.go, d.go"},
		{
			name:  "collapses overflow",
			files: []string{"1.go", "2.go", "3.go", "4.go", "5.go"},
			max:   3,
			want:  "1.go, 2.go, 3.go, +2",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatFooterFileNames(tt.files, tt.max); got != tt.want {
				t.Errorf("formatFooterFileNames() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestComposeRichStatusFooter_FullLayout locks the two-line layout end to end:
// elapsed + model on line 1, workdir + tools on line 2.
func TestComposeRichStatusFooter_FullLayout(t *testing.T) {
	e := newClaudeFooterEngine()
	e.i18n = NewI18n(LangChinese)
	session := &controllableAgentSession{
		model:   "claude-opus-5",
		workDir: "/tmp/ws",
		contextUsage: &ContextUsage{
			UsedTokens:    430,
			ContextWindow: 1000,
		},
	}
	stats := turnFooterStats{ToolCount: 4, Files: []string{"/tmp/ws/core/engine.go", "/tmp/ws/daemon/launchd.go"}}

	got := e.composeRichStatusFooter(false, time.Now().Add(-290*time.Second), nil, session, "/tmp/ws", stats)
	lines := strings.Split(got, "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 footer lines, got %d: %q", len(lines), got)
	}
	if !strings.HasPrefix(lines[0], "⏱ 用时 4 分 5") {
		t.Errorf("line 1 = %q, want elapsed prefix", lines[0])
	}
	if want := " · 🧠 claude-opus-5 · ctx ▰▰▰▰▱▱▱▱▱▱43%"; !strings.HasSuffix(lines[0], want) {
		t.Errorf("line 1 = %q, want model segment suffix %q", lines[0], want)
	}
	if !strings.HasPrefix(lines[1], "📁 ") || !strings.Contains(lines[1], "ws") {
		t.Errorf("line 2 = %q, want workdir prefix", lines[1])
	}
	if want := " · 🔧 工具 4 次 · 改动 2: engine.go, launchd.go"; !strings.HasSuffix(lines[1], want) {
		t.Errorf("line 2 = %q, want tools segment suffix %q", lines[1], want)
	}
}

// TestJoinFooterSegments covers the pairing helpers, including the collapse that
// keeps a half-empty pair from rendering a dangling separator.
func TestJoinFooterSegments(t *testing.T) {
	tests := []struct {
		name     string
		segments []string
		want     string
	}{
		{name: "both present", segments: []string{"⏱ 用时 1 秒", "🧠 codex"}, want: "⏱ 用时 1 秒 · 🧠 codex"},
		{name: "first only", segments: []string{"⏱ 用时 1 秒", ""}, want: "⏱ 用时 1 秒"},
		{name: "second only", segments: []string{"", "🔧 4 tools"}, want: "🔧 4 tools"},
		{name: "neither", segments: []string{"", ""}, want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := joinFooterSegments(tt.segments...); got != tt.want {
				t.Errorf("joinFooterSegments(%q) = %q, want %q", tt.segments, got, tt.want)
			}
		})
	}

	if got, want := joinFooterLines("a", "", "b"), "a\nb"; got != want {
		t.Errorf("joinFooterLines() = %q, want %q — an empty line must not leave a blank row", got, want)
	}
	if got := joinFooterLines("", ""); got != "" {
		t.Errorf("joinFooterLines(empty) = %q, want empty", got)
	}
}

// TestComposeRichStatusFooter_Toggles verifies each display toggle still gates
// its own segment, and that dropping both segments of a pair drops the line.
func TestComposeRichStatusFooter_Toggles(t *testing.T) {
	newSession := func() *controllableAgentSession {
		return &controllableAgentSession{
			model:        "claude-opus-5",
			workDir:      "/tmp/ws",
			contextUsage: &ContextUsage{UsedTokens: 430, ContextWindow: 1000},
		}
	}
	stats := turnFooterStats{ToolCount: 2, Files: []string{"/tmp/ws/a.go"}}

	t.Run("master toggle off", func(t *testing.T) {
		e := newClaudeFooterEngine()
		e.i18n = NewI18n(LangEnglish)
		e.SetReplyFooterEnabled(false)
		if got := e.composeRichStatusFooter(false, time.Now(), nil, newSession(), "/tmp/ws", stats); got != "" {
			t.Errorf("footer = %q, want empty when reply_footer is off", got)
		}
	})

	t.Run("streaming yields nothing", func(t *testing.T) {
		e := newClaudeFooterEngine()
		e.i18n = NewI18n(LangEnglish)
		if got := e.composeRichStatusFooter(true, time.Now(), nil, newSession(), "/tmp/ws", stats); got != "" {
			t.Errorf("footer = %q, want empty while streaming", got)
		}
	})

	t.Run("context indicator off leaves elapsed alone on line 1", func(t *testing.T) {
		e := newClaudeFooterEngine()
		e.i18n = NewI18n(LangEnglish)
		e.SetShowContextIndicator(false)
		got := e.composeRichStatusFooter(false, time.Now(), nil, newSession(), "/tmp/ws", stats)
		lines := strings.Split(got, "\n")
		if len(lines) != 2 {
			t.Fatalf("footer = %q, want 2 lines", got)
		}
		if strings.Contains(got, footerModelGlyph) || strings.Contains(lines[0], footerSegmentSep) {
			t.Errorf("line 1 = %q, want elapsed only with no trailing separator", lines[0])
		}
		if !strings.Contains(lines[1], footerWorkdirGlyph) || !strings.Contains(lines[1], footerToolsGlyph) {
			t.Errorf("line 2 = %q, want workdir and tools retained", lines[1])
		}
	})

	t.Run("workdir indicator off leaves tools alone on line 2", func(t *testing.T) {
		e := newClaudeFooterEngine()
		e.i18n = NewI18n(LangEnglish)
		e.SetShowWorkdirIndicator(false)
		got := e.composeRichStatusFooter(false, time.Now(), nil, newSession(), "/tmp/ws", stats)
		lines := strings.Split(got, "\n")
		if len(lines) != 2 {
			t.Fatalf("footer = %q, want 2 lines", got)
		}
		if strings.Contains(got, footerWorkdirGlyph) {
			t.Errorf("footer = %q, want no workdir segment", got)
		}
		if !strings.HasPrefix(lines[1], footerToolsGlyph) {
			t.Errorf("line 2 = %q, want tools segment at line start", lines[1])
		}
	})

	t.Run("toolless turn keeps workdir on line 2", func(t *testing.T) {
		e := newClaudeFooterEngine()
		e.i18n = NewI18n(LangEnglish)
		got := e.composeRichStatusFooter(false, time.Now(), nil, newSession(), "/tmp/ws", turnFooterStats{})
		lines := strings.Split(got, "\n")
		if len(lines) != 2 {
			t.Fatalf("footer = %q, want 2 lines", got)
		}
		if strings.Contains(got, footerToolsGlyph) {
			t.Errorf("footer = %q, want no tools segment", got)
		}
		if !strings.HasPrefix(lines[1], footerWorkdirGlyph) || strings.Contains(lines[1], footerSegmentSep) {
			t.Errorf("line 2 = %q, want workdir only with no trailing separator", lines[1])
		}
	})

	t.Run("empty pair drops the line entirely", func(t *testing.T) {
		e := newClaudeFooterEngine()
		e.i18n = NewI18n(LangEnglish)
		e.SetShowWorkdirIndicator(false)
		got := e.composeRichStatusFooter(false, time.Now(), nil, newSession(), "/tmp/ws", turnFooterStats{})
		if strings.Contains(got, "\n") {
			t.Errorf("footer = %q, want a single line when both line-2 segments are gone", got)
		}
	})
}
