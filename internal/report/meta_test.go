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

func TestInitCarriesTheFilesItWrote(t *testing.T) {
	doc := Init([]string{
		"/fleet/atlas/.frit.yml", "/fleet/atlas/plan/proto.md"})

	assert.Equal(t, Schema, doc.Schema)
	assert.Equal(t, "init", doc.Command)
	assert.Equal(t, []string{
		"/fleet/atlas/.frit.yml", "/fleet/atlas/plan/proto.md"}, doc.Paths)
}

// TestInitNeverNull pins the list-is-[]-never-null rule for init too: a
// document that wrote nothing still carries an empty list, not null.
func TestInitNeverNull(t *testing.T) {
	doc := Init(nil)

	assert.NotNil(t, doc.Paths)
	assert.Empty(t, doc.Paths)
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
