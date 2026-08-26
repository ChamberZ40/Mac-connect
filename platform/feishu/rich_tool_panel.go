package feishu

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/chenhg5/cc-connect/core"
)

// Rendering of the rich-card tool lane.
//
// One tool call renders as up to three stacked lines inside the collapsible
// "Tools" panel:
//
//	Run command (710 ms) · Succeeded
//	List files in current directory
//	Result
//	total 48 ...
//
// The headline answers "what ran, how long, did it work" without the reader
// having to parse the arguments; the second line is the call itself; the result
// block is what the tool reported back, truncated hard because a Feishu card
// carries a whole turn's worth of these under a single JSON size limit.
//
// The panel header replaces a bare count with the lane's wall-clock span:
//
//	执行时间 9m 53s · 使用 13 个工具
//
// which is the one number a reader wants from a collapsed panel.

const (
	// richToolResultMaxLen caps a single rendered tool result. Feishu rejects
	// cards over maxRichCardJSONBytes, and a turn can hold a dozen tool calls,
	// so per-step generosity here costs the whole card.
	richToolResultMaxLen = 300

	// richToolDetailMaxLen caps the argument line. Anything longer is a script
	// body, not a label.
	richToolDetailMaxLen = 200

	richToolResultLabel = "Result"

	richToolStatusSucceeded = "Succeeded"
	richToolStatusFailed    = "Failed"
	richToolStatusRunning   = "Running"

	richToolSegmentSep = " · "
	richToolTruncMark  = "…"
)

// formatToolCallDuration renders a single call's runtime:
//
//	710ms -> "710 ms"
//	2.8s  -> "2.8 s"
//	72s   -> "1m 12s"
//
// Returns "" for zero or negative input: ToolStep.Duration is zero both while a
// call is running and when its timing was never observed, and "0 ms" would read
// as a measurement in either case.
func formatToolCallDuration(d time.Duration) string {
	if d <= 0 {
		return ""
	}
	switch {
	case d < time.Second:
		return fmt.Sprintf("%d ms", d.Milliseconds())
	case d < time.Minute:
		return fmt.Sprintf("%.1f s", d.Seconds())
	default:
		total := int64(d / time.Second)
		return fmt.Sprintf("%dm %ds", total/60, total%60)
	}
}

// formatToolLaneSpan renders the whole lane's span for the panel header:
//
//	710ms -> "1s"    (sub-second spans round up so the header never reads "0s")
//	53s   -> "53s"
//	9m53s -> "9m 53s"
//	1h4m  -> "1h 04m"
//
// Returns "" when the span is unknown, so the caller can fall back to a count.
func formatToolLaneSpan(d time.Duration) string {
	if d <= 0 {
		return ""
	}
	total := int64(math.Round(d.Seconds()))
	if total < 1 {
		total = 1
	}
	switch {
	case total < 60:
		return fmt.Sprintf("%ds", total)
	case total < 3600:
		return fmt.Sprintf("%dm %ds", total/60, total%60)
	default:
		return fmt.Sprintf("%dh %02dm", total/3600, (total%3600)/60)
	}
}

// toolStepStatusWord condenses the several ways an agent can report an outcome
// (success flag, exit code, status string) into one word.
//
// A step that is Done but reported nothing counts as succeeded: agents omit the
// status fields on the happy path far more often than on failure.
func toolStepStatusWord(step core.ToolStep) string {
	if !step.Done {
		return richToolStatusRunning
	}
	if step.Success != nil {
		if *step.Success {
			return richToolStatusSucceeded
		}
		return richToolStatusFailed
	}
	if step.ExitCode != nil {
		if *step.ExitCode == 0 {
			return richToolStatusSucceeded
		}
		return richToolStatusFailed
	}
	switch strings.ToLower(strings.TrimSpace(step.Status)) {
	case "failed", "error", "failure", "cancelled", "canceled", "aborted", "timeout":
		return richToolStatusFailed
	case "":
		return richToolStatusSucceeded
	default:
		return richToolStatusSucceeded
	}
}

// richToolStepHeadline renders the first line of a tool row:
//
//	Run command (710 ms) · Succeeded
//	Build (2.8 s) · Failed · exit 1
//
// The exit code only appears on failure, where it is diagnostic; on the happy
// path "exit 0" is noise.
func richToolStepHeadline(step core.ToolStep) string {
	name := richStepDisplayName(step)
	if dur := formatToolCallDuration(step.Duration); dur != "" {
		name += " (" + dur + ")"
	}
	status := toolStepStatusWord(step)
	segments := []string{name, status}
	if status == richToolStatusFailed && step.ExitCode != nil {
		segments = append(segments, fmt.Sprintf("exit %d", *step.ExitCode))
	}
	return strings.Join(segments, richToolSegmentSep)
}

// toolLaneSpan measures the lane as wall-clock: earliest observed start to
// latest observed end. Returns 0 when no step carries timing, and falls back to
// the sum of durations when starts were never stamped (older agents), which is
// the closest honest answer available.
func toolLaneSpan(steps []core.ToolStep) time.Duration {
	var (
		earliest, latest time.Time
		sum              time.Duration
	)
	for _, step := range steps {
		sum += max(step.Duration, 0)
		if step.StartedAt.IsZero() {
			continue
		}
		end := step.StartedAt.Add(max(step.Duration, 0))
		if earliest.IsZero() || step.StartedAt.Before(earliest) {
			earliest = step.StartedAt
		}
		if latest.IsZero() || end.After(latest) {
			latest = end
		}
	}
	if earliest.IsZero() || !latest.After(earliest) {
		return sum
	}
	return latest.Sub(earliest)
}

// richLaneTotals carries a turn's real lane sizes. Panel headers are rendered
// from these rather than from the steps actually drawn, because both the row cap
// and payload-size compaction shrink the drawn set — which used to make the
// header's tool count disagree with the engine-composed footer count on exactly
// the long turns where the reader relies on it.
type richLaneTotals struct {
	reasoning int
	tools     int
	toolSpan  time.Duration
}

func richLaneTotalsFor(steps []core.ToolStep) richLaneTotals {
	reasoning, tools := splitRichStepsByLane(steps)
	return richLaneTotals{
		reasoning: len(reasoning),
		tools:     len(tools),
		toolSpan:  toolLaneSpan(tools),
	}
}

// richToolsLaneTitle renders the tools panel header from the lane's own steps.
func richToolsLaneTitle(steps []core.ToolStep, lang string) string {
	return richToolsLaneTitleFor(len(steps), toolLaneSpan(steps), lang)
}

// richToolsLaneTitleFor renders the tools panel header. Falls back to the plain
// counted label ("Tools (13)" / "工具 (13)") while no timing is known, so a
// just-started turn does not show a fabricated elapsed time.
func richToolsLaneTitleFor(count int, laneSpan time.Duration, lang string) string {
	span := formatToolLaneSpan(laneSpan)
	if span == "" {
		return progressPanelTitle("Tools", count, lang)
	}
	if isZhLikeProgressLang(lang) {
		return fmt.Sprintf("执行时间 %s · 使用 %d 个工具", span, count)
	}
	noun := "tools"
	if count == 1 {
		noun = "tool"
	}
	return fmt.Sprintf("Elapsed %s · %d %s", span, count, noun)
}

// richToolStepDetail renders the argument line: the agent's own description
// when it wrote one, otherwise the sanitized argument.
//
// A multi-line script is reduced to its first meaningful line plus a count of
// what is hidden, rather than flattened: flattening a heredoc produces an
// unreadable ribbon of Python, and showing only the head hides that the call
// did more than that head says.
func richToolStepDetail(step core.ToolStep, lang string) string {
	if described := strings.TrimSpace(step.Description); described != "" {
		return truncateToolText(collapseToolTextLines(described), richToolDetailMaxLen)
	}
	detail := buildToolDisplay(step.Name, step.Summary).Detail
	if head, hidden := splitToolDetailHead(detail); hidden > 0 {
		return truncateToolText(head, richToolDetailMaxLen) + " " + hiddenLinesMarker(hidden, lang)
	}
	return truncateToolText(collapseToolTextLines(detail), richToolDetailMaxLen)
}

// splitToolDetailHead picks the line worth showing from a multi-line value and
// reports how many other lines it stands for. A script's opening line is
// routinely `cd /tmp/work`, so the head is the first line that carries intent.
//
// Returns hidden == 0 for single-line values and for JSON-ish payloads, whose
// line breaks are formatting rather than steps and which read better collapsed.
func splitToolDetailHead(detail string) (head string, hidden int) {
	normalized := strings.ReplaceAll(detail, "\r\n", "\n")
	if !strings.Contains(normalized, "\n") {
		return "", 0
	}
	lines := make([]string, 0, 8)
	for _, line := range strings.Split(normalized, "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			lines = append(lines, trimmed)
		}
	}
	if len(lines) < 2 {
		return "", 0
	}
	if strings.HasPrefix(lines[0], "{") || strings.HasPrefix(lines[0], "[") {
		return "", 0
	}
	head = lines[0]
	for _, line := range lines {
		if commandSegmentIntent(line) != "" {
			head = line
			break
		}
	}
	return head, len(lines) - 1
}

// hiddenLinesMarker labels the lines a row could not show, so a one-line
// rendering is never mistaken for the whole script.
func hiddenLinesMarker(hidden int, lang string) string {
	if isZhLikeProgressLang(lang) {
		return fmt.Sprintf("(+%d 行)", hidden)
	}
	noun := "lines"
	if hidden == 1 {
		noun = "line"
	}
	return fmt.Sprintf("(+%d %s)", hidden, noun)
}

// collapseToolTextLines folds a multi-line value onto one line, squeezing runs
// of whitespace so the result stays scannable.
func collapseToolTextLines(text string) string {
	return strings.Join(strings.Fields(strings.ReplaceAll(text, "\r\n", "\n")), " ")
}

// truncateToolText cuts text to max runes, marking the cut so a clipped value
// is never mistaken for the whole one.
func truncateToolText(text string, max int) string {
	if max <= 0 {
		return ""
	}
	runes := []rune(text)
	if len(runes) <= max {
		return text
	}
	return strings.TrimRight(string(runes[:max]), " \t") + richToolTruncMark
}

// richToolResultBlock renders the trailing result block, or "" when the tool
// reported nothing.
func richToolResultBlock(step core.ToolStep) string {
	result := strings.TrimSpace(step.Result)
	if result == "" {
		return ""
	}
	return richToolResultLabel + "\n" + truncateToolText(result, richToolResultMaxLen)
}
