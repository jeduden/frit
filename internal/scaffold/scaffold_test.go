package scaffold

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jeduden/frit/internal/planmeta"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteProtoWritesTheSchema(t *testing.T) {
	planDir := filepath.Join(t.TempDir(), "plan")

	path, err := WriteProto(planDir, false)

	require.NoError(t, err)
	assert.Equal(t, filepath.Join(planDir, planmeta.ProtoName), path)
	got, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, protoSchema, got)
}

// TestWriteProtoCreatesThePlanDir pins that a fresh repo, whose plan
// directory does not exist yet, gets it made rather than an error: the
// whole point is to scaffold machinery a repo has not got.
func TestWriteProtoCreatesThePlanDir(t *testing.T) {
	planDir := filepath.Join(t.TempDir(), "nested", "plan")

	_, err := WriteProto(planDir, false)

	require.NoError(t, err)
	_, statErr := os.Stat(filepath.Join(planDir, planmeta.ProtoName))
	require.NoError(t, statErr)
}

func TestWriteProtoRefusesToClobber(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, planmeta.ProtoName)
	mine := []byte("# my edited proto\n")
	require.NoError(t, os.WriteFile(target, mine, 0o644))

	_, err := WriteProto(dir, false)

	require.ErrorIs(t, err, ErrExists)
	after, readErr := os.ReadFile(target)
	require.NoError(t, readErr)
	assert.Equal(t, mine, after, "the edit survives")
}

func TestWriteProtoForceOverwrites(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, planmeta.ProtoName)
	require.NoError(t, os.WriteFile(target, []byte("old\n"), 0o644))

	_, err := WriteProto(dir, true)

	require.NoError(t, err)
	after, readErr := os.ReadFile(target)
	require.NoError(t, readErr)
	assert.Equal(t, protoSchema, after)
}

// TestShippedProtoMatchesRepo pins the embedded schema equal to this
// repo's plan/proto.md, so the copy frit ships from init cannot drift
// from the schema frit itself lints plans against.
func TestShippedProtoMatchesRepo(t *testing.T) {
	repo, err := os.ReadFile(
		filepath.Join("..", "..", "plan", planmeta.ProtoName))
	require.NoError(t, err)
	assert.Equal(t, repo, protoSchema)
}
