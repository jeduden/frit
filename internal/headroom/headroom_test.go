package headroom

import (
	"strings"
	"testing"

	"github.com/jeduden/mdsmith/pkg/mdsmith"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newSession builds a session against a default config — max-file-length
// keeps its built-in cap of 300 lines, which is all the oracle needs.
func newSession(t *testing.T) *mdsmith.Session {
	t.Helper()
	sess, err := mdsmith.NewSession(mdsmith.SessionOptions{
		Workspace: mdsmith.NewMemWorkspace(nil),
		Config:    mdsmith.ConfigYAML(""),
	})
	require.NoError(t, err)

	return sess
}

// plan renders a minimal front-matter block followed by n body lines, so
// the front matter never counts toward ReserveLines but always counts
// toward mdsmith's own line cap.
func plan(bodyLines int) []byte {
	var b strings.Builder
	b.WriteString("---\nid: 1\ntitle: x\n---\n")
	for i := 0; i < bodyLines; i++ {
		b.WriteString("body line\n")
	}

	return []byte(b.String())
}

func TestReserveLinesIsCeilOfPercentOfBodyLinesOnly(t *testing.T) {
	src := plan(50)

	assert.Equal(t, 5, ReserveLines(src, 10))
	assert.Equal(t, 15, ReserveLines(src, 30))
	assert.Equal(t, 0, ReserveLines(src, 0))
}

func TestRoomWithPlentyOfRoomReturnsTheFullReserve(t *testing.T) {
	sess := newSession(t)
	src := plan(50)
	reserve := ReserveLines(src, 10)
	require.Equal(t, 5, reserve)

	room, err := Room(sess, "plan/1_x.md", src, reserve)

	require.NoError(t, err)
	assert.Equal(t, reserve, room, "50-line body plus 5 lines of padding sits well under the 300-line cap")
}

func TestRoomPaddedToTheCapReturnsLessThanTheReserve(t *testing.T) {
	sess := newSession(t)
	src := plan(290) // 4 lines of front matter + 290 body lines = 294 total
	reserve := ReserveLines(src, 10)
	require.Equal(t, 29, reserve)

	room, err := Room(sess, "plan/1_x.md", src, reserve)

	require.NoError(t, err)
	assert.Less(t, room, reserve,
		"294 total lines plus the full 29-line reserve would exceed the 300-line cap")
	assert.Equal(t, 6, room, "300 - 294 lines of headroom actually fit")
}
