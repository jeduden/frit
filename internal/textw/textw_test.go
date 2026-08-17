package textw

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWidthCountsColumnsNotRunes(t *testing.T) {
	assert.Equal(t, 5, Width("hello"))
	assert.Equal(t, 2, Width("🔳"), "a status glyph paints two columns")
	assert.Equal(t, 3, Width("a—b"), "an em dash paints one")
	assert.Equal(t, 0, Width(""))
}

func TestTruncateMarksACut(t *testing.T) {
	assert.Equal(t, "hello", Truncate("hello", 10))
	assert.Equal(t, "hello", Truncate("hello", 5))
	assert.Equal(t, "", Truncate("hello", 0))
	assert.Equal(t, "", Truncate("hello", -3))

	got := Truncate("hello world", 6)
	assert.LessOrEqual(t, Width(got), 6, "the result fits the cap")
	assert.Contains(t, got, "…", "a cut is marked")
}

// TestTruncateKeepsAWideGlyphWhole: a two-column glyph is dropped
// entirely rather than sliced to a half-column when it would not fit.
func TestTruncateKeepsAWideGlyphWhole(t *testing.T) {
	got := Truncate("ab🔳cd", 3)

	assert.LessOrEqual(t, Width(got), 3)
}
