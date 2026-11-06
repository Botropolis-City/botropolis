package ui

import (
	"time"

	"botropolis.city/botropolis/pkg/events"
)

// NoticeFor is how long a notice stays on its plate before it reads as
// nothing; it stays in the stream.
const NoticeFor = 4 * time.Second

// Message is one notice: what it says, the tone of its dot, the key
// that acts on it, shown as a cap, and the session it is about, which a
// click on its stream line flies to. It never asks a question (item 352).
type Message struct {
	Text      string
	Tone      Tone
	Key       string
	SessionID string
}

// Notice is the one passing message: the newest, for NoticeFor after it
// is set, then nothing. A second message replaces the first.
type Notice struct {
	msg Message
	at  time.Time
}

func (n *Notice) Set(m Message, now time.Time) {
	n.msg, n.at = m, now
}

func (n *Notice) Clear() {
	n.msg = Message{}
}

// Current is the message while it is fresh.
func (n *Notice) Current(now time.Time) (Message, bool) {
	if n.msg.Text == "" || !now.Before(n.at.Add(NoticeFor)) {
		return Message{}, false
	}
	return n.msg, true
}

// Text is the message's text while it is fresh, else "".
func (n *Notice) Text(now time.Time) string {
	m, _ := n.Current(now)
	return m.Text
}

// NoticeEvent is the message as the stream's line: a client-side event of
// kind Notice, which the daemon never logs, carrying the tone's name as
// its detail so the line is drawn and muted as a line of that tone is.
func NoticeEvent(m Message, at time.Time) events.Event {
	detail := ""
	if m.Tone != ToneNone {
		detail = m.Tone.String()
	}
	return events.Event{At: at, Kind: events.Notice, SessionID: m.SessionID, Title: m.Text, Detail: detail}
}

// NoticeTone is the tone a notice's line carries, ToneNone for any other
// event.
func NoticeTone(e events.Event) Tone {
	if e.Kind != events.Notice {
		return ToneNone
	}
	for t := ToneNeedsYou; t <= ToneMerged; t++ {
		if t.String() == e.Detail {
			return t
		}
	}
	return ToneNone
}

// noticeKind is the daemon's kind a notice's line is muted as: its tone's
// kind where the stream flags one, so a notice in the error tone stands
// out as an error does, and any other reads as routine.
func noticeKind(e events.Event) events.Kind {
	if e.Kind == events.Notice {
		return events.Kind(e.Detail)
	}
	return e.Kind
}
