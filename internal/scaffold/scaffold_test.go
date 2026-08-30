package scaffold

import (
	"bytes"
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

func TestWriteMdsmithConfigWritesTheDefault(t *testing.T) {
	dir := t.TempDir()

	path, err := WriteMdsmithConfig(dir, false)

	require.NoError(t, err)
	assert.Equal(t, filepath.Join(dir, ".mdsmith.yml"), path)
	got, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, mdsmithConfig, got)
}

func TestWriteMdsmithConfigRefusesToClobber(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, ".mdsmith.yml")
	mine := []byte("front-matter: false\n")
	require.NoError(t, os.WriteFile(target, mine, 0o644))

	_, err := WriteMdsmithConfig(dir, false)

	require.ErrorIs(t, err, ErrExists)
	after, readErr := os.ReadFile(target)
	require.NoError(t, readErr)
	assert.Equal(t, mine, after, "the edit survives")
}

func TestWritePlanIndexWritesTheSeed(t *testing.T) {
	dir := t.TempDir()

	path, err := WritePlanIndex(dir, false)

	require.NoError(t, err)
	assert.Equal(t, filepath.Join(dir, "PLAN.md"), path)
	got, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, planIndex, got)
}

// TestPlanIndexCatalogsFolderPlans pins that both catalog blocks in the
// seed glob folder plans (plan/<id>_slug/plan.md), not only flat plans,
// so a repo on the folder shape does not silently drop plans from its
// generated index. The proto exclusion must survive in each block.
func TestPlanIndexCatalogsFolderPlans(t *testing.T) {
	assert.Equal(t, 2, bytes.Count(planIndex, []byte(`"plan/*/plan.md"`)),
		"both catalog blocks glob folder plans")
	assert.Equal(t, 2, bytes.Count(planIndex, []byte(`"!plan/proto.md"`)),
		"both catalog blocks still exclude the proto")
}

func TestWritePlanIndexRefusesToClobber(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "PLAN.md")
	mine := []byte("# my plans\n")
	require.NoError(t, os.WriteFile(target, mine, 0o644))

	_, err := WritePlanIndex(dir, false)

	require.ErrorIs(t, err, ErrExists)
	after, readErr := os.ReadFile(target)
	require.NoError(t, readErr)
	assert.Equal(t, mine, after, "the edit survives")
}
