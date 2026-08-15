// Package gitobj reads refs, trees and blobs out of a repository's
// object store.
//
// Everything here works on objects rather than on a working tree, so
// a plan file can be read from a branch that was never checked out
// and has no worktree. That is the whole reason frit can index 313
// refs without 313 checkouts.
package gitobj

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
)

// missingMarker is what the batch plumbing prints for an object that
// does not exist. git echoes the request back, so the marker is the
// last field rather than the whole line.
const missingMarker = "missing"

// Ref is one entry of `git for-each-ref`.
type Ref struct {
	// Name is the full ref name, e.g. refs/remotes/dudennl/main.
	Name string
	// OID is the object the ref points at.
	OID string
}

// Short is the ref name without its category prefix, which is how
// people refer to a branch.
func (r Ref) Short() string {
	for _, prefix := range []string{
		"refs/heads/", "refs/remotes/", "refs/tags/",
	} {
		if strings.HasPrefix(r.Name, prefix) {
			return strings.TrimPrefix(r.Name, prefix)
		}
	}

	return r.Name
}

// Branch is the branch name with any remote prefix removed, and
// reports whether this ref names a branch at all.
//
// A local branch and its copy on every remote answer the same name, so
// one hold pattern covers both — which is what a claim pushed to a
// shared forge looks like from here.
//
// A tag answers false. Tags are immutable markers, not lanes: a tag
// that happened to be named like a claim would otherwise read as work
// in progress forever.
func (r Ref) Branch() (string, bool) {
	if name, ok := strings.CutPrefix(r.Name, "refs/heads/"); ok {
		return name, true
	}

	if rest, ok := strings.CutPrefix(r.Name, "refs/remotes/"); ok {
		// refs/remotes/<remote>/<branch> — drop the remote.
		_, branch, found := strings.Cut(rest, "/")
		if !found || branch == "" {
			return "", false
		}
		// refs/remotes/<remote>/HEAD is a symbolic pointer, not work.
		if branch == "HEAD" {
			return "", false
		}

		return branch, true
	}

	return "", false
}

// ParseRefs reads `git for-each-ref --format=%(objectname) %(refname)`.
//
// Malformed lines are skipped rather than failing the walk: one
// unreadable ref must not blind the index to the other 312.
func ParseRefs(data []byte) []Ref {
	lines := strings.Split(string(data), "\n")
	out := make([]Ref, 0, len(lines))

	for _, line := range lines {
		oid, name, ok := strings.Cut(strings.TrimSpace(line), " ")
		if !ok || oid == "" || name == "" {
			continue
		}
		out = append(out, Ref{Name: name, OID: oid})
	}

	return out
}

// TreeEntry is one line of `git ls-tree -r`.
type TreeEntry struct {
	Mode string
	Type string
	OID  string
	Path string
}

// ParseLsTree reads `git ls-tree -r <tree>` output.
//
// The format is `<mode> <type> <oid>\t<path>`: the tab matters,
// because a path may contain spaces and splitting on whitespace
// would truncate it.
func ParseLsTree(data []byte) []TreeEntry {
	lines := strings.Split(string(data), "\n")
	out := make([]TreeEntry, 0, len(lines))

	for _, line := range lines {
		meta, path, ok := strings.Cut(line, "\t")
		if !ok {
			continue
		}
		fields := strings.Fields(meta)
		if len(fields) != 3 {
			continue
		}
		out = append(out, TreeEntry{
			Mode: fields[0],
			Type: fields[1],
			OID:  fields[2],
			Path: path,
		})
	}

	return out
}

// Check is one answer from `git cat-file --batch-check`.
//
// Answers correspond to requests by position, so Missing entries are
// kept rather than dropped: dropping them would silently shift every
// later answer onto the wrong request.
type Check struct {
	OID     string
	Type    string
	Size    int64
	Missing bool
}

// ParseBatchCheck reads `git cat-file --batch-check` output.
func ParseBatchCheck(data []byte) []Check {
	lines := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
	out := make([]Check, 0, len(lines))

	for _, line := range lines {
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[len(fields)-1] == missingMarker {
			out = append(out, Check{Missing: true})
			continue
		}
		if len(fields) != 3 {
			continue
		}
		size, err := strconv.ParseInt(fields[2], 10, 64)
		if err != nil {
			continue
		}
		out = append(out, Check{
			OID: fields[0], Type: fields[1], Size: size,
		})
	}

	return out
}

// Object is one answer from `git cat-file --batch`, payload included.
type Object struct {
	OID     string
	Type    string
	Data    []byte
	Missing bool
}

// ParseBatch reads `git cat-file --batch` output.
//
// Each answer is a header line, then exactly Size bytes of payload,
// then a newline. The payload is sliced by the byte count from the
// header and never by scanning for a delimiter: blob content is
// arbitrary bytes and may contain newlines, so a line-oriented parse
// would corrupt every file longer than one line.
func ParseBatch(data []byte) ([]Object, error) {
	var out []Object

	for len(data) > 0 {
		nl := bytes.IndexByte(data, '\n')
		if nl < 0 {
			return nil, fmt.Errorf("truncated header: %q", data)
		}
		header := string(data[:nl])
		data = data[nl+1:]

		fields := strings.Fields(header)
		if len(fields) >= 2 && fields[len(fields)-1] == missingMarker {
			out = append(out, Object{Missing: true})
			continue
		}
		if len(fields) != 3 {
			return nil, fmt.Errorf("bad header: %q", header)
		}

		size, err := strconv.Atoi(fields[2])
		if err != nil {
			return nil, fmt.Errorf("bad size in %q: %w", header, err)
		}
		if size < 0 || size > len(data) {
			return nil, fmt.Errorf(
				"object %s claims %d bytes, %d remain",
				fields[0], size, len(data))
		}

		out = append(out, Object{
			OID:  fields[0],
			Type: fields[1],
			Data: data[:size],
		})

		// Skip the payload and the newline git writes after it.
		data = data[size:]
		if len(data) > 0 && data[0] == '\n' {
			data = data[1:]
		}
	}

	return out, nil
}
