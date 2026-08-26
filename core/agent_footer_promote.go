package core

import (
	"regexp"
	"strings"
)

// Promotion of agent-authored footer lines.
//
// Agents have no channel for contributing their own metadata to the reply
// footer: statusFooter is composed entirely by the engine from session/agent
// state (see buildReplyFooter / composeRichStatusFooter), so an agent that
// wants to report something the engine cannot know — elapsed wall-clock as it
// measured it, tool-call counts, files it touched — can only write it into the
// response body, where it renders above the card divider instead of in the
// small/dim footer block.
//
// Promotion closes that gap: when enabled, a trailing footer-marker line is
// lifted out of the body and appended to statusFooter, so it reaches the same
// StatusFooterSender / RichCardSupporter rendering path as engine-owned lines.
//
// This is the inverse of stripAgentFooterLines: that one deletes agent-emitted
// model/token lines that duplicate the engine footer, this one relocates a line
// the engine could not have produced.

// promotedAgentFooterRe matches a footer-marker line: a horizontal-bar glyph
// ("─" U+2500, or an em dash) followed by whitespace and at least one
// non-space character.
//
// Both glyphs are accepted because agents (and prompt templates) use them
// interchangeably as a visual separator.
var promotedAgentFooterRe = regexp.MustCompile(`^[ \t]*[─—][ \t]+\S[^\n]*$`)

// extractPromotedAgentFooter splits a trailing footer-marker line off the end
// of text. It returns the body with that line removed and the footer line
// itself, both with surrounding whitespace trimmed.
//
// Only the LAST line is considered: a marker line in the middle of a response
// is ordinary prose or a horizontal rule and is left untouched. When the last
// line does not match, body is returned unchanged and footer is "".
//
// The footer line is preserved verbatim (marker glyph included) — platforms
// that do not implement StatusFooterSender inline it via appendReplyFooter,
// where the marker is still doing useful visual work.
func extractPromotedAgentFooter(text string) (body, footer string) {
	trimmed := strings.TrimRight(text, "\n \t")
	if trimmed == "" {
		return text, ""
	}
	idx := strings.LastIndex(trimmed, "\n")
	last := trimmed[idx+1:] // idx == -1 yields the whole string
	if !promotedAgentFooterRe.MatchString(last) {
		return text, ""
	}
	if idx < 0 {
		return "", strings.TrimSpace(last)
	}
	return strings.TrimRight(trimmed[:idx], "\n \t"), strings.TrimSpace(last)
}

// joinStatusFooterBlocks concatenates status-footer blocks with '\n', skipping
// empty ones. Platforms split statusFooter on '\n' and render one dim line per
// entry, so a stray empty block would render as a blank footer row.
func joinStatusFooterBlocks(blocks ...string) string {
	kept := make([]string, 0, len(blocks))
	for _, b := range blocks {
		if strings.TrimSpace(b) != "" {
			kept = append(kept, b)
		}
	}
	return strings.Join(kept, "\n")
}
