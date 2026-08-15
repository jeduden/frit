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
