package report

import (
	"bytes"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteJSONIndentsAndEndsWithANewline(t *testing.T) {
	var out bytes.Buffer

	require.NoError(t, WriteJSON(&out, Version("1.2.3")))

	assert.Equal(t, "{\n  \"schema\": 1,\n  \"command\": \"version\","+
		"\n  \"version\": \"1.2.3\"\n}\n", out.String())
}

// TestWriteJSONLeavesTextAsWritten pins the encoder's HTML escaping
// off. A plan title carrying & or < is read by people and agents,
// never embedded in a page, and & in a title is unreadable.
func TestWriteJSONLeavesTextAsWritten(t *testing.T) {
	var out bytes.Buffer

	require.NoError(t, WriteJSON(&out,
		Init([]string{"/fleet/atlas & co/.frit.yml"})))

	assert.Contains(t, out.String(), "/fleet/atlas & co/.frit.yml")
}

func TestWriteJSONReportsAFailedWrite(t *testing.T) {
	err := WriteJSON(failWriter{}, Version("dev"))

	assert.Error(t, err)
}

type failWriter struct{}

func (failWriter) Write([]byte) (int, error) {
	return 0, errors.New("closed")
}

func TestProblemOfCarriesTheRepositoryAndTheMessage(t *testing.T) {
	got := problemOf("atlas", errors.New("not a git repository"))

	assert.Equal(t, Problem{
		Repo: "atlas", Message: "not a git repository",
	}, got)
}

func TestNewHeaderStampsTheSchemaAndTheCommand(t *testing.T) {
	assert.Equal(t, header{Schema: Schema, Command: "repos"},
		newHeader("repos"))
}
