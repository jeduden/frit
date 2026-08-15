// Package index groups plan files into one entry per plan, across
// every ref that carries it.
package index

import (
	"fmt"
	"sort"

	"github.com/jeduden/frit/internal/planmeta"
	"github.com/jeduden/frit/internal/plans"
)

// Key identifies a plan across the whole fleet.
//
// The host and repository are part of the key because plan ids are
// only unique within a repository: atlas allocates timestamps
// (2608142306) while mdsmith allocates counters (100), so an id
// alone would collide the moment both are indexed.
type Key struct {
	Host string
	Repo string
	ID   int64
}

// String renders the key in its canonical host:repo:id form.
func (k Key) String() string {
	return fmt.Sprintf("%s:%s:%d", k.Host, k.Repo, k.ID)
}

// Version is one distinct content of a plan, and the refs carrying
// it. A plan edited on a branch has two versions: the branch's and
// everyone else's.
type Version struct {
	OID  string
	Plan planmeta.Plan
	Path string
	Refs []string
}

// Entry is one plan, however many refs and versions it has.
type Entry struct {
	Key      Key
	Versions []Version
}

// Primary is the authoritative version of the plan.
//
// Build orders Versions so the first is the one to trust: the copy
// on the default branch when there is one, then the copy the most
// refs carry, then the lowest object id for a stable tie-break.
//
// Preferring the default branch over the majority is not arbitrary.
// Measured on this machine, atlas's plans appear on 391 refs, most
// of them old lanes that were branched before the work finished and
// never updated again. Counting refs therefore reports a plan as
// unstarted long after it merged — 98 done where the default branch
// says 106. The status is flipped by the commit that lands the work,
// so the branch that work lands on is the one telling the truth.
func (e Entry) Primary() Version {
	return e.Versions[0]
}

// RefCount is how many refs carry this plan in any version.
func (e Entry) RefCount() int {
	n := 0
	for _, v := range e.Versions {
		n += len(v.Refs)
	}

	return n
}

// Build groups collected files into one entry per plan id.
//
// Each distinct blob is parsed exactly once, however many refs carry
// it. On a repository with 987 refs sharing a few hundred plan files
// that is the difference between a few hundred parses and a few
// hundred thousand.
//
// Files that are not plans — the proto.md schema template, anything
// without front matter — are skipped, and the reasons are returned
// alongside so a caller can report them without the walk failing.
func Build(
	host, repo, preferred string, files []plans.File,
) ([]Entry, []error) {
	parsed := map[string]planmeta.Plan{}
	failed := map[string]bool{}
	var problems []error

	byID := map[int64]map[string]*Version{}

	for _, f := range files {
		if planmeta.IsProto(f.Path) || failed[f.OID] {
			continue
		}

		plan, ok := parsed[f.OID]
		if !ok {
			var err error
			plan, err = planmeta.Parse(f.Content)
			if err != nil {
				failed[f.OID] = true
				problems = append(problems,
					fmt.Errorf("%s: %w", f.Path, err))
				continue
			}
			parsed[f.OID] = plan
		}

		addVersion(byID, plan, f)
	}

	return collect(host, repo, preferred, byID), problems
}

// addVersion records that one ref carries one content of one plan.
func addVersion(
	byID map[int64]map[string]*Version, plan planmeta.Plan, f plans.File,
) {
	versions, ok := byID[plan.ID]
	if !ok {
		versions = map[string]*Version{}
		byID[plan.ID] = versions
	}

	v, ok := versions[f.OID]
	if !ok {
		v = &Version{OID: f.OID, Plan: plan, Path: f.Path}
		versions[f.OID] = v
	}
	v.Refs = append(v.Refs, f.Ref)
}

// collect flattens the grouping into sorted entries.
func collect(
	host, repo, preferred string, byID map[int64]map[string]*Version,
) []Entry {
	out := make([]Entry, 0, len(byID))

	for id, versions := range byID {
		entry := Entry{Key: Key{Host: host, Repo: repo, ID: id}}
		for _, v := range versions {
			sort.Strings(v.Refs)
			entry.Versions = append(entry.Versions, *v)
		}
		rank(entry.Versions, preferred)
		out = append(out, entry)
	}

	sort.Slice(out, func(i, j int) bool {
		return out[i].Key.ID < out[j].Key.ID
	})

	return out
}

// rank orders versions so the authoritative one comes first: the
// copy on the preferred ref, then the copy the most refs carry, then
// the lowest object id so the order is stable between runs.
func rank(versions []Version, preferred string) {
	sort.Slice(versions, func(i, j int) bool {
		a, b := versions[i], versions[j]
		if onRef(a, preferred) != onRef(b, preferred) {
			return onRef(a, preferred)
		}
		if len(a.Refs) != len(b.Refs) {
			return len(a.Refs) > len(b.Refs)
		}

		return a.OID < b.OID
	})
}

// onRef reports whether a version is carried by the named ref.
func onRef(v Version, ref string) bool {
	if ref == "" {
		return false
	}
	for _, r := range v.Refs {
		if r == ref {
			return true
		}
	}

	return false
}
