package gitobj

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseRefsReadsLocalRemoteAndTagRefs(t *testing.T) {
	in := "88559bff refs/heads/ci/runner-speed\n" +
		"4afe275e refs/remotes/dudennl/main\n" +
		"d301e4ce refs/tags/v1.0.0\n"

	got := ParseRefs([]byte(in))

	require.Len(t, got, 3)
	assert.Equal(t, "refs/heads/ci/runner-speed", got[0].Name)
	assert.Equal(t, "88559bff", got[0].OID)
	assert.Equal(t, "ci/runner-speed", got[0].Short())
	assert.Equal(t, "dudennl/main", got[1].Short())
	assert.Equal(t, "v1.0.0", got[2].Short())
}

func TestRefShortLeavesAnUnrecognisedNamespaceAlone(t *testing.T) {
	r := Ref{Name: "refs/notes/commits"}

	assert.Equal(t, "refs/notes/commits", r.Short())
}

func TestParseRefsSkipsMalformedLines(t *testing.T) {
	in := "\nnot-a-ref\n88559bff refs/heads/main\n\n"

	got := ParseRefs([]byte(in))

	require.Len(t, got, 1)
	assert.Equal(t, "refs/heads/main", got[0].Name)
}

func TestParseLsTreeSplitsOnTabSoPathsMayHoldSpaces(t *testing.T) {
	in := "100644 blob aaaa\tplan/2608142306_fleet index.md\n" +
		"100644 blob bbbb\tplan/proto.md\n"

	got := ParseLsTree([]byte(in))

	require.Len(t, got, 2)
	assert.Equal(t, "plan/2608142306_fleet index.md", got[0].Path)
	assert.Equal(t, "aaaa", got[0].OID)
	assert.Equal(t, "blob", got[0].Type)
	assert.Equal(t, "100644", got[0].Mode)
}

func TestParseLsTreeSkipsLinesWithoutATab(t *testing.T) {
	got := ParseLsTree([]byte("garbage\n100644 blob cc\tplan/a.md\n"))

	require.Len(t, got, 1)
	assert.Equal(t, "plan/a.md", got[0].Path)
}

func TestParseBatchCheckKeepsMissingEntriesInPosition(t *testing.T) {
	in := "aaaa tree 128\n" +
		"refs/heads/no-plans:plan missing\n" +
		"bbbb tree 64\n"

	got := ParseBatchCheck([]byte(in))

	require.Len(t, got, 3, "positions must line up with requests")
	assert.Equal(t, "aaaa", got[0].OID)
	assert.Equal(t, int64(128), got[0].Size)
	assert.True(t, got[1].Missing)
	assert.Equal(t, "bbbb", got[2].OID)
}

func TestParseBatchReadsPayloadsByByteCount(t *testing.T) {
	// Content deliberately holds newlines: a line-oriented parse
	// would truncate it, which is the bug this pins.
	body := "---\nid: 1\n---\n# Plan\n"
	in := "aaaa blob " + itoa(len(body)) + "\n" + body + "\n" +
		"bbbb blob 2\nhi\n"

	got, err := ParseBatch([]byte(in))

	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, body, string(got[0].Data))
	assert.Equal(t, "aaaa", got[0].OID)
	assert.Equal(t, "hi", string(got[1].Data))
}

func TestParseBatchHandlesEmptyBlobs(t *testing.T) {
	got, err := ParseBatch([]byte("aaaa blob 0\n\n"))

	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Empty(t, got[0].Data)
}

func TestParseBatchKeepsMissingEntriesInPosition(t *testing.T) {
	in := "deadbeef missing\naaaa blob 2\nhi\n"

	got, err := ParseBatch([]byte(in))

	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.True(t, got[0].Missing)
	assert.Equal(t, "hi", string(got[1].Data))
}

func TestParseBatchRejectsATruncatedPayload(t *testing.T) {
	_, err := ParseBatch([]byte("aaaa blob 99\nshort\n"))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "claims 99 bytes")
}

func TestParseBatchRejectsAMalformedHeader(t *testing.T) {
	_, err := ParseBatch([]byte("aaaa blob\n"))
	require.Error(t, err)

	_, err = ParseBatch([]byte("aaaa blob x\nhi\n"))
	require.Error(t, err)

	_, err = ParseBatch([]byte("no-newline"))
	require.Error(t, err)
}

func TestParseBatchOnEmptyInput(t *testing.T) {
	got, err := ParseBatch(nil)

	require.NoError(t, err)
	assert.Empty(t, got)
}

// itoa keeps the fixture readable without importing strconv into
// every test.
func itoa(n int) string {
	return strings.TrimSpace(sprintInt(n))
}

func sprintInt(n int) string {
	if n == 0 {
		return "0"
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}

	return string(digits)
}

func TestBranchStripsTheRemoteSoOnePatternCoversBoth(t *testing.T) {
	local, ok := Ref{Name: "refs/heads/plan/42-x"}.Branch()
	require.True(t, ok)
	assert.Equal(t, "plan/42-x", local)

	remote, ok := Ref{Name: "refs/remotes/origin/plan/42-x"}.Branch()
	require.True(t, ok)
	assert.Equal(t, "plan/42-x", remote,
		"a claim pushed to a shared forge is the same claim")
	assert.Equal(t, local, remote)
}

func TestBranchRejectsWhatIsNotALane(t *testing.T) {
	for _, name := range []string{
		"refs/tags/v1.0.0",
		"refs/remotes/origin/HEAD",
		"refs/notes/commits",
		"refs/stash",
		"refs/remotes/origin",
	} {
		_, ok := Ref{Name: name}.Branch()
		assert.False(t, ok, name)
	}
}
