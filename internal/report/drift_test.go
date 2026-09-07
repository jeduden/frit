package report

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestDriftAddRowRendersNilCommitsAsAnEmptyList: a plan with no
// commits naming it still reports Commits as [] rather than null,
// the same rule every other list in the JSON contract follows.
func TestDriftAddRowRendersNilCommitsAsAnEmptyList(t *testing.T) {
	doc := NewDrift("/fleet")
	doc.AddRow("atlas", 7, false, false, nil)

	assert.Equal(t, []DriftCommit{}, doc.Rows[0].Commits)
}
