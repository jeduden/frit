package herdr

// The dispatch surface: the mutating herdr calls frit composes but does
// not own. Each one hands a pane back to herdr and steps away — frit
// focuses, starts and prompts, but never reads an agent back. There is
// deliberately no wrapper here for `agent read`: the one call that would
// turn a board into a chat client has no home in this package.

// Focus raises the pane a lane is already running in. It sends no text
// and starts no agent — the read-only handoff, rung one of the ladder.
func Focus(runner Runner, target string) error {
	_, err := runner("agent", "focus", target)

	return err
}
