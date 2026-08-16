package herdr

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fixture reads a recorded `herdr agent list` response.
func fixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	require.NoError(t, err)

	return data
}

// TestParseAgentListReadsEveryRecordShape is the whole risk surface of
// this package: three records that look different on the wire — an
// integrated agent, a bare pane with none, and one whose status frit
// cannot read — must all come back as typed panes.
func TestParseAgentListReadsEveryRecordShape(t *testing.T) {
	panes, err := ParseAgentList(fixture(t, "agent_list.json"))
	require.NoError(t, err)
	require.Len(t, panes, 3)

	integrated := panes[0]
	assert.Equal(t, "claude", integrated.Agent)
	assert.Equal(t, StatusWorking, integrated.Status)
	assert.Equal(t, "/home/jeduden/git/jeduden/duden.nl", integrated.CWD)
	assert.Equal(t,
		"8e6a81ff-63e8-410c-ac6c-0036b2654549", integrated.Session)
	assert.Equal(t, "wC:p1", integrated.PaneID)
	assert.Equal(t, "wC", integrated.Workspace)
	assert.Equal(t, "Resume batch handoff documentation", integrated.Title)
	assert.True(t, integrated.HasAgent())

	bare := panes[1]
	assert.Empty(t, bare.Agent)
	assert.Empty(t, bare.Session)
	assert.False(t, bare.HasAgent(),
		"a pane with no agent must not read as staffed")

	unknown := panes[2]
	assert.Equal(t, "claude", unknown.Agent)
	assert.Equal(t, StatusUnknown, unknown.Status)
	assert.True(t, unknown.HasAgent())
}

// TestParseAgentListPrefersTheStrippedTitle keeps the marker glyph a
// terminal paints for animation out of the label a board shows.
func TestParseAgentListPrefersTheStrippedTitle(t *testing.T) {
	panes, err := ParseAgentList([]byte(`{"result":{"agents":[
		{"pane_id":"w1:p1","terminal_title":"◐ Busy",
		 "terminal_title_stripped":"Busy"}]}}`))
	require.NoError(t, err)
	require.Len(t, panes, 1)
	assert.Equal(t, "Busy", panes[0].Title)
}

// TestParseAgentListFallsBackToTheRawTitle covers a record that
// carries no stripped form: the raw title is better than none.
func TestParseAgentListFallsBackToTheRawTitle(t *testing.T) {
	panes, err := ParseAgentList([]byte(`{"result":{"agents":[
		{"pane_id":"w1:p1","terminal_title":"plain"}]}}`))
	require.NoError(t, err)
	require.Len(t, panes, 1)
	assert.Equal(t, "plain", panes[0].Title)
}

// TestParseAgentListHandlesNoAgents is the empty-but-valid socket: a
// running server with no panes is not an error.
func TestParseAgentListHandlesNoAgents(t *testing.T) {
	panes, err := ParseAgentList([]byte(`{"result":{"agents":[]}}`))
	require.NoError(t, err)
	assert.Empty(t, panes)
}

// TestParseAgentListRejectsInvalidJSON: garbage on the socket is a
// failure the caller must be able to tell apart from an empty list.
func TestParseAgentListRejectsInvalidJSON(t *testing.T) {
	_, err := ParseAgentList([]byte("not json"))
	assert.Error(t, err)
}
