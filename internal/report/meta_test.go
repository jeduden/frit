package report

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestVersionCarriesTheBuildVersion(t *testing.T) {
	doc := Version("1.2.3")

	assert.Equal(t, Schema, doc.Schema)
	assert.Equal(t, "version", doc.Command)
	assert.Equal(t, "1.2.3", doc.Version)
}

func TestInitCarriesTheFileItWrote(t *testing.T) {
	doc := Init("/fleet/atlas/.frit.yml")

	assert.Equal(t, Schema, doc.Schema)
	assert.Equal(t, "init", doc.Command)
	assert.Equal(t, "/fleet/atlas/.frit.yml", doc.Path)
}

func TestSkillsCarriesTheFilesItWrote(t *testing.T) {
	doc := Skills([]string{"/fleet/atlas/.claude/skills/plan-pick/SKILL.md"})

	assert.Equal(t, Schema, doc.Schema)
	assert.Equal(t, "skills", doc.Command)
	assert.Equal(t, []string{"/fleet/atlas/.claude/skills/plan-pick/SKILL.md"},
		doc.Paths)
}

// TestSkillsNeverNull pins the list-is-[]-never-null rule: a Skills
// document that wrote nothing still carries an empty list a consumer
// can range over without a nil test.
func TestSkillsNeverNull(t *testing.T) {
	doc := Skills(nil)

	assert.NotNil(t, doc.Paths)
	assert.Empty(t, doc.Paths)
}
