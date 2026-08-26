package core

import (
	"fmt"
	"math"
	"path/filepath"
	"strings"
)

// Rendering of the two-line rich-card status footer.
//
// Layout (composeRichStatusFooter assembles these; every segment is skipped when
// its data is missing or its display toggle is off, and a line with no surviving
// segment disappears entirely):
//
//	⏱ 用时 4 分 50 秒 · 🧠 claude-opus-5 · ctx ▰▰▰▰▱▱▱▱▱▱43%
//	📁 ~/code/cc-connect · 🔧 工具 4 次 · 改动 2: engine.go, config.go
//
// Two lines, not four: this block renders as small/dim text under the reply, so
// four stacked lines took more vertical space than the reply itself in short
// turns. Pairing keeps "what the turn cost" on line 1 and "where it happened /
// what it touched" on line 2.
//
// Every segment carries a leading glyph because platforms render this block with
// no other visual structure — the glyph is what keeps two segments on one line
// readable as two things. Raw token counts (out/in/cw/cr) and reasoning effort
// are deliberately absent: the gauge answers "how much room is left" at a
// glance, which is the only token question worth asking from a chat client. The
// full detail remains available in the CCD-style footer
// (Engine.buildClaudeStatusLineFooter) that non-rich platforms fall back to.

const (
	footerModelGlyph   = "🧠"
	footerWorkdirGlyph = "📁"
	footerToolsGlyph   = "🔧"

	// footerSegmentSep separates two segments sharing a line. Same separator the
	// segments use internally (model · ctx), which is fine: the glyphs, not the
	// separator, are what delimit segments.
	footerSegmentSep = " · "

	// ctxGaugeCells is the gauge width; each cell is therefore worth 10%.
	ctxGaugeCells       = 10
	ctxGaugeFilledGlyph = "▰"
	ctxGaugeEmptyGlyph  = "▱"

	// footerMaxFileNames caps how many changed-file names are spelled out
	// before the remainder collapses into a "+N" tail, so a turn that touched
	// twenty files does not produce a footer line that wraps five times.
	footerMaxFileNames = 3
)

// joinFooterSegments places the given segments on one line, dropping empty ones.
// Returns "" when nothing survives, so the caller can drop the whole line.
func joinFooterSegments(segments ...string) string {
	return joinNonEmpty(footerSegmentSep, segments)
}

// joinFooterLines stacks the given lines, dropping empty ones so a suppressed
// line leaves no blank row in the rendered card.
func joinFooterLines(lines ...string) string {
	return joinNonEmpty("\n", lines)
}

func joinNonEmpty(sep string, parts []string) string {
	kept := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			kept = append(kept, p)
		}
	}
	return strings.Join(kept, sep)
}

// turnFooterStats carries the per-turn activity counters that only the event
// loop can observe. Passed by value into composeRichStatusFooter so the footer
// builder stays a pure function of its inputs.
type turnFooterStats struct {
	ToolCount int      // tool invocations seen this turn
	Files     []string // paths written this turn, in first-touch order
}

// ctxGauge renders a used-context percentage as a fixed-width bar:
//
//	0   -> "▱▱▱▱▱▱▱▱▱▱"
//	43  -> "▰▰▰▰▱▱▱▱▱▱"
//	100 -> "▰▰▰▰▰▰▰▰▰▰"
//
// Out-of-range input is clamped. Any non-zero percentage fills at least one
// cell so "context in use" never reads as an empty bar.
func ctxGauge(pct int) string {
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}
	filled := pct * ctxGaugeCells / 100
	if filled == 0 && pct > 0 {
		filled = 1
	}
	return strings.Repeat(ctxGaugeFilledGlyph, filled) +
		strings.Repeat(ctxGaugeEmptyGlyph, ctxGaugeCells-filled)
}

// contextUsedPercent computes the share of the context window in use, rounded
// to the nearest percent and clamped to 0..100.
//
// Reports ok=false when the session exposes no usable window/usage numbers, so
// callers can drop the gauge instead of rendering a misleading 0%.
func contextUsedPercent(usage *ContextUsage) (int, bool) {
	if usage == nil || usage.ContextWindow <= 0 {
		return 0, false
	}
	used := usage.UsedTokens
	if used <= 0 && usage.TotalTokens > 0 {
		used = usage.TotalTokens
	}
	if used <= 0 {
		return 0, false
	}
	pct := int(math.Round(float64(used) * 100 / float64(usage.ContextWindow)))
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}
	return pct, true
}

// footerModelLine renders the model line:
//
//	🧠 claude-opus-5 · ctx ▰▰▰▰▱▱▱▱▱▱43%
//
// Model and gauge are the whole line: neither token counts nor reasoning effort
// appear here. Both remain available in the CCD-style footer
// (Engine.buildClaudeStatusLineFooter). The gauge segment drops out when the
// session reports no usable context numbers; the result is "" when nothing at
// all is known.
func footerModelLine(model string, usage *ContextUsage) string {
	var parts []string
	if model = strings.TrimSpace(model); model != "" {
		parts = append(parts, model)
	}
	if pct, ok := contextUsedPercent(usage); ok {
		parts = append(parts, fmt.Sprintf("ctx %s%d%%", ctxGauge(pct), pct))
	}
	if len(parts) == 0 {
		return ""
	}
	return footerModelGlyph + " " + strings.Join(parts, " · ")
}

// footerWorkdirLine renders the workspace line: "📁 ~/code/cc-connect".
// The path is expected to be pre-normalized by replyFooterWorkDir.
func footerWorkdirLine(dir string) string {
	if dir = strings.TrimSpace(dir); dir == "" {
		return ""
	}
	return footerWorkdirGlyph + " " + dir
}

// footerToolsLine renders the activity line:
//
//	🔧 工具 4 次 · 改动 2: engine.go, config.go
//
// The changed-files segment is omitted when no write was observed, and the
// whole line is omitted when the turn ran no tools at all (a plain
// question-and-answer turn has nothing worth reporting).
func footerToolsLine(i18n *I18n, stats turnFooterStats) string {
	if i18n == nil || stats.ToolCount <= 0 {
		return ""
	}
	line := footerToolsGlyph + " " + i18n.Tf(MsgFooterToolCount, stats.ToolCount)
	if names := formatFooterFileNames(stats.Files, footerMaxFileNames); names != "" {
		line += " · " + i18n.Tf(MsgFooterFilesChanged, len(stats.Files), names)
	}
	return line
}

// formatFooterFileNames renders changed paths as a comma-separated list of base
// names, keeping at most max entries and collapsing the rest into "+N":
//
//	["a/b.go", "c/d.go"]                   -> "b.go, d.go"
//	["1.go", "2.go", "3.go", "4.go"] (3)   -> "1.go, 2.go, 3.go, +1"
//
// Base names only: the directory is already implied by the 📁 workdir line, and
// full paths would dominate the footer.
func formatFooterFileNames(files []string, max int) string {
	if len(files) == 0 || max <= 0 {
		return ""
	}
	names := make([]string, 0, max+1)
	for _, f := range files {
		if len(names) == max {
			names = append(names, fmt.Sprintf("+%d", len(files)-max))
			break
		}
		if base := filepath.Base(strings.TrimSpace(f)); base != "" && base != "." {
			names = append(names, base)
		}
	}
	return strings.Join(names, ", ")
}
