// Package report is the shape of what a frit command found.
//
// Every command builds a report and then renders it twice over: as an
// aligned table for a person, or as JSON for an agent. Both readings
// come from this one model, so the two cannot drift apart — a fact
// added for the table is in the JSON by construction.
//
// The JSON is a contract, and three rules keep it one:
//
//   - Every key is always present. A consumer indexes a field without
//     first testing for it.
//   - A list is empty rather than null, because iterating null is an
//     error in most languages and iterating nothing is not.
//   - A repository frit could not read is carried in the document, not
//     only written to stderr. A consumer reading stdout alone must
//     still be able to tell a clean fleet from one it never opened.
package report

import (
	"encoding/json"
	"io"

	"github.com/jeduden/frit/internal/gitwt"
)

// Schema is the version of the JSON contract. It rises when a field
// changes meaning or leaves. Adding one does not move it: a consumer
// reading by name is unaffected by a key it does not know.
const Schema = 1

// header opens every document, so a consumer handed one out of context
// knows what it is holding.
type header struct {
	Schema  int    `json:"schema"`
	Command string `json:"command"`
}

// newHeader stamps a document with the contract version and the
// command that produced it.
func newHeader(command string) header {
	return header{Schema: Schema, Command: command}
}

// Problem is one repository frit could not read. The walk continues
// past it — a single broken checkout must not blind the whole board —
// so the failure travels with the report instead of ending it.
type Problem struct {
	Repo    string `json:"repo"`
	Message string `json:"message"`
}

// problemOf records a failure against the repository it happened in.
func problemOf(repo string, err error) Problem {
	return Problem{Repo: repo, Message: err.Error()}
}

// Worktree is one checkout, as both renderings see it.
//
// Name and HasCommit are computed here rather than left to each
// renderer: the basename is how a lane is referred to conversationally,
// and whether a checkout has a commit is the question the whole orphan
// report turns on. Neither should be re-derived by a consumer.
type Worktree struct {
	Path        string `json:"path"`
	Name        string `json:"name"`
	Head        string `json:"head"`
	Branch      string `json:"branch"`
	Detached    bool   `json:"detached"`
	Bare        bool   `json:"bare"`
	Locked      bool   `json:"locked"`
	LockReason  string `json:"lock_reason"`
	Prunable    bool   `json:"prunable"`
	PruneReason string `json:"prune_reason"`
	HasCommit   bool   `json:"has_commit"`
}

// worktreeOf carries every porcelain fact across unchanged.
func worktreeOf(w gitwt.Worktree) Worktree {
	return Worktree{
		Path:        w.Path,
		Name:        w.Name(),
		Head:        w.Head,
		Branch:      w.Branch,
		Detached:    w.Detached,
		Bare:        w.Bare,
		Locked:      w.Locked,
		LockReason:  w.LockReason,
		Prunable:    w.Prunable,
		PruneReason: w.PruneReason,
		HasCommit:   w.HasCommit(),
	}
}

// worktreesOf converts a list, and returns an empty one rather than
// nil so the encoded form is [] instead of null.
func worktreesOf(ws []gitwt.Worktree) []Worktree {
	out := make([]Worktree, 0, len(ws))
	for _, w := range ws {
		out = append(out, worktreeOf(w))
	}

	return out
}

// WriteJSON encodes a document for a consumer.
//
// Indented, because a person reads this output too — piping frit
// through jq to make it legible should not be the price of --json.
// HTML escaping is off: a plan title carrying & or < is read, never
// embedded in a page, and & in a title is unreadable.
func WriteJSON(w io.Writer, doc any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)

	return enc.Encode(doc)
}
