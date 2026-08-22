package herdr

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

// fakeAgentList fakes `herdr agent list` with a canned set of panes,
// the same shape who_test.go's herdrReturning builds.
func fakeAgentList(agents ...map[string]any) Runner {
	body, err := json.Marshal(map[string]any{
		"result": map[string]any{"agents": agents},
	})
	if err != nil {
		panic(err)
	}

	return func(...string) ([]byte, error) { return body, nil }
}

func paneWithSession(session, paneID, status string) map[string]any {
	return map[string]any{
		"agent":        "claude",
		"agent_status": status,
		"pane_id":      paneID,
		"agent_session": map[string]any{
			"value": session,
		},
	}
}

// TestSessionLiveOnlyAPositiveAnswerCounts: F3/S61 — a live bound
// session vetoes, but anything short of a positive answer must not.
func TestSessionLiveOnlyAPositiveAnswerCounts(t *testing.T) {
	cases := []struct {
		name    string
		runner  Runner
		session string
		want    bool
	}{
		{
			name:    "empty session is never live",
			runner:  fakeAgentList(),
			session: "",
			want:    false,
		},
		{
			name:    "the unbound dash is never live",
			runner:  fakeAgentList(paneWithSession("-", "wA:p1", StatusWorking)),
			session: "-",
			want:    false,
		},
		{
			name: "herdr unreachable is no veto",
			runner: func(...string) ([]byte, error) {
				return nil, errors.New("dial unix .herdr.sock: no such file")
			},
			session: "wA:p1-session",
			want:    false,
		},
		{
			name:    "the session is unknown to herdr",
			runner:  fakeAgentList(paneWithSession("other-session", "wB:p1", StatusWorking)),
			session: "missing-session",
			want:    false,
		},
		{
			name:    "a working agent on the session is live",
			runner:  fakeAgentList(paneWithSession("sess-1", "wC:p1", StatusWorking)),
			session: "sess-1",
			want:    true,
		},
		{
			name:    "an idle agent on the session is still live",
			runner:  fakeAgentList(paneWithSession("sess-1", "wC:p1", StatusIdle)),
			session: "sess-1",
			want:    true,
		},
		{
			name:    "an unknown-status agent on the session is still live",
			runner:  fakeAgentList(paneWithSession("sess-1", "wC:p1", StatusUnknown)),
			session: "sess-1",
			want:    true,
			// unknown means frit cannot read what the agent is doing,
			// never that the pane is gone (Presence's own rule) — a
			// veto must err toward not seizing an occupied lane.
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, SessionLive(tc.runner, tc.session))
		})
	}
}

// TestSessionDeadOnlyAReachableHerdrConfirmsIt: the mirror of
// SessionLive (2608212203) — a session is dead only when herdr
// actually answered and shows no live agent under it. An unreachable
// herdr must never read as a death, only as unknown, so it falls back
// to the staleness window rather than skip it (S76).
func TestSessionDeadOnlyAReachableHerdrConfirmsIt(t *testing.T) {
	cases := []struct {
		name    string
		runner  Runner
		session string
		want    bool
	}{
		{
			name:    "empty session is never confirmed dead",
			runner:  fakeAgentList(),
			session: "",
			want:    false,
		},
		{
			name:    "the unbound dash is never confirmed dead",
			runner:  fakeAgentList(),
			session: "-",
			want:    false,
		},
		{
			name: "herdr unreachable is not a death, only unknown",
			runner: func(...string) ([]byte, error) {
				return nil, errors.New("dial unix .herdr.sock: no such file")
			},
			session: "sess-1",
			want:    false,
		},
		{
			name:    "no pane carries the session: confirmed dead",
			runner:  fakeAgentList(paneWithSession("other-session", "wB:p1", StatusWorking)),
			session: "sess-1",
			want:    true,
		},
		{
			name:    "a working agent on the session is not dead",
			runner:  fakeAgentList(paneWithSession("sess-1", "wC:p1", StatusWorking)),
			session: "sess-1",
			want:    false,
		},
		{
			name:    "an idle agent on the session is still not dead",
			runner:  fakeAgentList(paneWithSession("sess-1", "wC:p1", StatusIdle)),
			session: "sess-1",
			want:    false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, SessionDead(tc.runner, tc.session))
		})
	}
}

// TestSessionDeadInChecksManySessionsAgainstOneListCall: the pure half
// SessionDead delegates to, so a caller checking many held plans'
// sessions reads the pane list once rather than once per plan.
func TestSessionDeadInChecksManySessionsAgainstOneListCall(t *testing.T) {
	panes := []Pane{
		{Session: "sess-1", Agent: "claude", Status: StatusWorking},
	}

	assert.False(t, SessionDeadIn(panes, ""), "empty session is never confirmed dead")
	assert.False(t, SessionDeadIn(panes, "-"), "the unbound dash is never confirmed dead")
	assert.False(t, SessionDeadIn(panes, "sess-1"), "a working agent on the session is not dead")
	assert.True(t, SessionDeadIn(panes, "sess-2"), "no pane carries this session: confirmed dead")
	assert.True(t, SessionDeadIn(nil, "sess-1"), "no panes at all: confirmed dead")
}

// TestPaneSessionReadsTheSessionBoundToAPane: start reads a just-opened
// pane's session back this way, since neither worktree.create nor
// agent.start answers with one.
func TestPaneSessionReadsTheSessionBoundToAPane(t *testing.T) {
	runner := fakeAgentList(paneWithSession("sess-1", "wZ:p1", StatusWorking))

	assert.Equal(t, "sess-1", PaneSession(runner, "wZ:p1"))
	assert.Equal(t, "", PaneSession(runner, "wOther:p1"), "no match, no session")
	assert.Equal(t, "", PaneSession(runner, ""), "an empty pane id reads no session")

	broken := func(...string) ([]byte, error) {
		return nil, errors.New("boom")
	}
	assert.Equal(t, "", PaneSession(broken, "wZ:p1"))
}
