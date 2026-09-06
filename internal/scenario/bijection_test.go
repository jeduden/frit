package scenario

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const featuresPath = "../../features"

// matrixPaths are the catalog documents kept in bijection with
// features/: the lease protocol's cross-host matrix, and the command
// catalog for a behavior that is not one.
var matrixPaths = []string{
	"../../docs/research/lease-protocol.md",
	"../../docs/research/command-scenarios.md",
}

// TestMatrixAndFeaturesAreInBijection keeps every matrix document and
// the godog feature tags the only sources for a scenario's existence:
// a matrix row documented without a tagged scenario, or a tag naming
// no matrix row in any document, fails the build rather than passing
// silently, and the failure lists the ids in a stable order.
func TestMatrixAndFeaturesAreInBijection(t *testing.T) {
	matrix, err := MatrixIDsAll(matrixPaths...)
	require.NoError(t, err)

	features, err := FeatureTagIDs(featuresPath)
	require.NoError(t, err)

	assert.Empty(t, onlyIn(matrix, features), "matrix scenarios with no tagged feature")
	assert.Empty(t, onlyIn(features, matrix), "feature tags naming no matrix scenario")
}

// onlyIn lists the ids in a that b lacks, sorted so a failure reads
// the same on every run.
func onlyIn(a, b map[string]bool) []string {
	var out []string
	for id := range a {
		if !b[id] {
			out = append(out, id)
		}
	}
	slices.Sort(out)

	return out
}

func TestOnlyInListsTheDifferenceSorted(t *testing.T) {
	a := map[string]bool{"S9": true, "S2": true, "S5": true}
	b := map[string]bool{"S5": true}
	assert.Equal(t, []string{"S2", "S9"}, onlyIn(a, b))
	assert.Empty(t, onlyIn(b, a))
}
