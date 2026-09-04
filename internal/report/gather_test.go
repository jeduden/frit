package report

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGatherJSONCarriesEveryKey pins the wire shape: every count is
// present, so a consumer indexes the block without first testing for a
// key. Adding this block does not move the schema version.
func TestGatherJSONCarriesEveryKey(t *testing.T) {
	var buf bytes.Buffer
	require.NoError(t, WriteJSON(&buf, Gather{
		Discovered: 3, Read: 2, Fetched: 1, Problems: 1, ElapsedMS: 20,
	}))

	for _, key := range []string{
		"discovered", "read", "fetched", "problems", "elapsed_ms",
	} {
		assert.Contains(t, buf.String(), `"`+key+`"`,
			"gather JSON must always carry %q", key)
	}
	assert.Equal(t, 1, Schema, "adding the gather block must not move the schema")
}

// TestGatherStatusLineShowsPartialWalk asserts the one line the table
// shows names the reduced counts of a partial walk — read of
// discovered, fetched, and problems — from the same struct the JSON
// carries, so a person and a consumer cannot see different coverage.
func TestGatherStatusLineShowsPartialWalk(t *testing.T) {
	line := Gather{
		Discovered: 3, Read: 2, Fetched: 1, Problems: 1, ElapsedMS: 20,
	}.StatusLine()

	assert.Contains(t, line, "2/3", "must show read of discovered")
	assert.Contains(t, line, "1 fetched", "must show the fetched count")
	assert.Contains(t, line, "1 problem", "must show the problem count")
	assert.Contains(t, line, "20ms", "must show the elapsed time")
}
