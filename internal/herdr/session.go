package herdr

// SessionLive reports whether herdr positively confirms a session is
// live right now — the takeover veto's whole decision (F3, S61): a
// live bound session vetoes takeover, but only a positive answer
// counts, so an unreachable herdr or an unknown session must read the
// same as no session at all, never the other way round.
//
// Status is deliberately not consulted. Presence already draws the
// line between "doing something" and "cannot tell" — unknown means
// frit could not read what the agent is doing, never that the pane is
// gone — and a veto must err toward not seizing an occupied lane.
func SessionLive(runner Runner, session string) bool {
	if session == "" || session == "-" {
		return false
	}
	panes, err := List(runner)
	if err != nil {
		return false
	}
	for _, p := range panes {
		if p.Session == session && p.HasAgent() {
			return true
		}
	}

	return false
}

// SessionDead reports whether herdr positively confirms a bound
// session is gone — the mirror of SessionLive, and the signal that
// frees a held plan at once rather than waiting out the whole
// takeover window (S76). Only a herdr that actually answered and
// shows no live agent under the session counts: an unreachable herdr
// can never read as a death, only as unknown, so it falls back to the
// window exactly like SessionLive fails safe toward not vetoing.
func SessionDead(runner Runner, session string) bool {
	panes, err := List(runner)
	if err != nil {
		return false
	}

	return SessionDeadIn(panes, session)
}

// SessionDeadIn is SessionDead's pure half: given panes already read
// by one List call, reports whether the session is confirmed gone.
// Shared by SessionDead and by a caller checking many sessions against
// the same List call — observeHolds reads the pane list once per
// fleet gather rather than once per held plan.
func SessionDeadIn(panes []Pane, session string) bool {
	if session == "" || session == "-" {
		return false
	}
	for _, p := range panes {
		if p.Session == session && p.HasAgent() {
			return false
		}
	}

	return true
}

// PaneSession reads the agent session bound to a pane id, "" when it
// cannot be read — herdr unreachable, or no pane with that id. `start`
// uses it to learn the session a just-started agent was given: neither
// worktree.create nor agent.start answers with one, so it is read back
// from the same agent list every other read uses.
func PaneSession(runner Runner, paneID string) string {
	if paneID == "" {
		return ""
	}
	panes, err := List(runner)
	if err != nil {
		return ""
	}
	for _, p := range panes {
		if p.PaneID == paneID {
			return p.Session
		}
	}

	return ""
}
