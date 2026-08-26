package core

import (
	"strings"
	"testing"
	"time"
)

func TestExtractPromotedAgentFooter(t *testing.T) {
	tests := []struct {
		name       string
		text       string
		wantBody   string
		wantFooter string
	}{
		{
			name:       "trailing marker line is promoted",
			text:       "answer body\n\n─ ~51s · 工具 6 次 · 成功 6/6",
			wantBody:   "answer body",
			wantFooter: "─ ~51s · 工具 6 次 · 成功 6/6",
		},
		{
			name:       "em dash marker also promoted",
			text:       "answer body\n— 12s · tools 3",
			wantBody:   "answer body",
			wantFooter: "— 12s · tools 3",
		},
		{
			name:       "footer only, no body",
			text:       "─ ~4s · 工具 1 次 · 成功 1/1",
			wantBody:   "",
			wantFooter: "─ ~4s · 工具 1 次 · 成功 1/1",
		},
		{
			name:       "trailing newlines tolerated",
			text:       "body\n─ footer line\n\n",
			wantBody:   "body",
			wantFooter: "─ footer line",
		},
		{
			name:       "leading indent tolerated",
			text:       "body\n  ─ footer line",
			wantBody:   "body",
			wantFooter: "─ footer line",
		},
		{
			name:       "marker not on last line is left alone",
			text:       "─ this is prose\nand this is the real last line",
			wantBody:   "─ this is prose\nand this is the real last line",
			wantFooter: "",
		},
		{
			name:       "bare marker without content is not a footer",
			text:       "body\n─",
			wantBody:   "body\n─",
			wantFooter: "",
		},
		{
			name:       "markdown horizontal rule is not a footer",
			text:       "body\n---",
			wantBody:   "body\n---",
			wantFooter: "",
		},
		{
			name:       "no marker at all",
			text:       "just a plain answer",
			wantBody:   "just a plain answer",
			wantFooter: "",
		},
		{
			name:       "empty input",
			text:       "",
			wantBody:   "",
			wantFooter: "",
		},
		{
			name:       "code fence content is not mistaken for a footer",
			text:       "```\n─ inside fence\n```",
			wantBody:   "```\n─ inside fence\n```",
			wantFooter: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, footer := extractPromotedAgentFooter(tt.text)
			if body != tt.wantBody {
				t.Errorf("body\n  got  = %q\n  want = %q", body, tt.wantBody)
			}
			if footer != tt.wantFooter {
				t.Errorf("footer\n  got  = %q\n  want = %q", footer, tt.wantFooter)
			}
		})
	}
}

func TestJoinStatusFooterBlocks(t *testing.T) {
	tests := []struct {
		name   string
		blocks []string
		want   string
	}{
		{
			name:   "both present",
			blocks: []string{"⏱ 用时 5 秒\nmodel · ctx 4%", "─ 工具 6 次"},
			want:   "⏱ 用时 5 秒\nmodel · ctx 4%\n─ 工具 6 次",
		},
		{
			name:   "engine footer empty",
			blocks: []string{"", "─ 工具 6 次"},
			want:   "─ 工具 6 次",
		},
		{
			name:   "promoted footer empty",
			blocks: []string{"model · ctx 4%", ""},
			want:   "model · ctx 4%",
		},
		{
			name:   "whitespace-only block skipped",
			blocks: []string{"model · ctx 4%", "   "},
			want:   "model · ctx 4%",
		},
		{
			name:   "all empty",
			blocks: []string{"", ""},
			want:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := joinStatusFooterBlocks(tt.blocks...); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

// ── engine integration ────────────────────────────────────────────────────────

func promoteFooterDisplay(promote bool) DisplayCfg {
	return DisplayCfg{
		Mode:               "full",
		ThinkingMessages:   true,
		ThinkingMaxLen:     300,
		ToolMaxLen:         500,
		ToolMessages:       true,
		PromoteAgentFooter: promote,
	}
}

// With promotion on and a footer-aware platform, the trailing line must arrive
// as the structured footer argument, never as part of the body.
func TestProcessInteractiveEvents_PromotesAgentFooterIntoStatusFooter(t *testing.T) {
	p := &stubFooterSendingPlatform{stubPlatformEngine: stubPlatformEngine{n: "telegram"}}
	e := NewEngine("test", &stubAgent{}, []Platform{p}, "", LangEnglish)
	e.SetDisplayConfig(promoteFooterDisplay(true))

	sessionKey := "telegram:user-promote-footer"
	session := e.sessions.GetOrCreateActive(sessionKey)
	agentSession := newControllableSession("s-promote-footer")
	state := &interactiveState{
		agentSession: agentSession,
		platform:     p,
		replyCtx:     "ctx-promote-footer",
	}
	e.interactiveStates[sessionKey] = state

	agentSession.events <- Event{Type: EventText, Content: "answer\n\n─ ~5s · tools 3 · ok 3/3"}
	agentSession.events <- Event{Type: EventResult, Done: true}
	e.processInteractiveEvents(state, session, e.sessions, sessionKey, "m-promote-footer", time.Now(), nil, nil, state.replyCtx)

	if len(p.footerCalls) != 1 {
		t.Fatalf("footerCalls = %#v, want exactly one footer-aware send", p.footerCalls)
	}
	if want := "answer|FOOTER|─ ~5s · tools 3 · ok 3/3"; p.footerCalls[0] != want {
		t.Fatalf("footer call =\n  %q\nwant\n  %q", p.footerCalls[0], want)
	}
	if got := p.getSent(); len(got) != 0 {
		t.Fatalf("plain Send = %#v, want empty when footer routing succeeded", got)
	}
	// The promoted line is metadata, so history keeps only the prose body.
	if got := session.GetHistory(0); len(got) != 1 || got[0].Content != "answer" {
		t.Fatalf("history = %#v, want body without the footer line", got)
	}
}

// Default (promotion off) must leave the agent's output byte-for-byte alone.
func TestProcessInteractiveEvents_KeepsAgentFooterInBodyByDefault(t *testing.T) {
	p := &stubFooterSendingPlatform{stubPlatformEngine: stubPlatformEngine{n: "telegram"}}
	e := NewEngine("test", &stubAgent{}, []Platform{p}, "", LangEnglish)
	e.SetDisplayConfig(promoteFooterDisplay(false))

	sessionKey := "telegram:user-promote-off"
	session := e.sessions.GetOrCreateActive(sessionKey)
	agentSession := newControllableSession("s-promote-off")
	state := &interactiveState{
		agentSession: agentSession,
		platform:     p,
		replyCtx:     "ctx-promote-off",
	}
	e.interactiveStates[sessionKey] = state

	body := "answer\n\n─ ~5s · tools 3 · ok 3/3"
	agentSession.events <- Event{Type: EventText, Content: body}
	agentSession.events <- Event{Type: EventResult, Done: true}
	e.processInteractiveEvents(state, session, e.sessions, sessionKey, "m-promote-off", time.Now(), nil, nil, state.replyCtx)

	if len(p.footerCalls) != 0 {
		t.Fatalf("footerCalls = %#v, want none when promotion is disabled", p.footerCalls)
	}
	sent := p.getSent()
	if len(sent) != 1 || sent[0] != body {
		t.Fatalf("sent = %#v, want the untouched body %q", sent, body)
	}
}

// Platforms without StatusFooterSender still show the line — inlined by
// appendReplyFooter in the engine's dim-italic shape.
func TestProcessInteractiveEvents_PromotedFooterInlinedWithoutFooterSender(t *testing.T) {
	p := &stubPlatformEngine{n: "telegram"}
	e := NewEngine("test", &stubAgent{}, []Platform{p}, "", LangEnglish)
	e.SetDisplayConfig(promoteFooterDisplay(true))

	sessionKey := "telegram:user-promote-inline"
	session := e.sessions.GetOrCreateActive(sessionKey)
	agentSession := newControllableSession("s-promote-inline")
	state := &interactiveState{
		agentSession: agentSession,
		platform:     p,
		replyCtx:     "ctx-promote-inline",
	}
	e.interactiveStates[sessionKey] = state

	agentSession.events <- Event{Type: EventText, Content: "answer\n\n─ ~5s · tools 3 · ok 3/3"}
	agentSession.events <- Event{Type: EventResult, Done: true}
	e.processInteractiveEvents(state, session, e.sessions, sessionKey, "m-promote-inline", time.Now(), nil, nil, state.replyCtx)

	sent := p.getSent()
	if len(sent) != 1 {
		t.Fatalf("sent = %#v, want one final reply", sent)
	}
	if !strings.HasPrefix(sent[0], "answer") {
		t.Fatalf("reply = %q, want it to start with the body", sent[0])
	}
	if !strings.Contains(sent[0], "*─ ~5s · tools 3 · ok 3/3*") {
		t.Fatalf("reply = %q, want the promoted line inlined as *…*", sent[0])
	}
}

// A reply that is nothing but the footer line must be delivered as-is rather
// than promoted into an empty message.
func TestProcessInteractiveEvents_FooterOnlyReplyIsNotPromoted(t *testing.T) {
	p := &stubFooterSendingPlatform{stubPlatformEngine: stubPlatformEngine{n: "telegram"}}
	e := NewEngine("test", &stubAgent{}, []Platform{p}, "", LangEnglish)
	e.SetDisplayConfig(promoteFooterDisplay(true))

	sessionKey := "telegram:user-promote-only"
	session := e.sessions.GetOrCreateActive(sessionKey)
	agentSession := newControllableSession("s-promote-only")
	state := &interactiveState{
		agentSession: agentSession,
		platform:     p,
		replyCtx:     "ctx-promote-only",
	}
	e.interactiveStates[sessionKey] = state

	body := "─ ~5s · tools 3 · ok 3/3"
	agentSession.events <- Event{Type: EventText, Content: body}
	agentSession.events <- Event{Type: EventResult, Done: true}
	e.processInteractiveEvents(state, session, e.sessions, sessionKey, "m-promote-only", time.Now(), nil, nil, state.replyCtx)

	if len(p.footerCalls) != 0 {
		t.Fatalf("footerCalls = %#v, want none for a footer-only reply", p.footerCalls)
	}
	if sent := p.getSent(); len(sent) != 1 || sent[0] != body {
		t.Fatalf("sent = %#v, want the untouched body %q", sent, body)
	}
}
