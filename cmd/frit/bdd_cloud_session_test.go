package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cucumber/godog"
)

// C26 runs the built frit as two Claude Code cloud sessions. Every such
// container is called "vm", so a holder named by hostname would read
// the second session's lost race as its own hold. It registers as its
// own section file, per docs/development.md, and runs through C15's
// herdr-less harness.
func init() {
	registrars = append(registrars, (*world).registerCloudSession)
}

func (w *world) registerCloudSession(sc *godog.ScenarioContext) {
	sc.Step(`^this host runs as Claude cloud session "([^"]+)"$`,
		w.thisHostRunsAsCloudSession)
	sc.Step(`^origin's claim marker for plan (\d+) names holder "([^"]+)"$`,
		w.originsClaimMarkerNamesHolder)
	sc.Step(`^the built frit runs "([^"]+)" from the lane, expecting it to fail$`,
		w.theBuiltFritRunsFromTheLaneExpectingFailure)
	sc.Step(`^it refuses the lane as held by "([^"]+)"$`, w.itRefusesTheLaneAsHeldBy)
	sc.Step(`^origin refuses refs outside refs/heads and every delete, as a Claude cloud session's proxy does$`,
		w.originRefusesWhatTheCloudProxyRefuses)
	sc.Step(`^origin carries plan (\d+)'s rescue as a branch under frit-rescue/$`,
		w.originCarriesTheRescueAsABranch)
}

// thisHostRunsAsCloudSession sets the session id a Claude Code cloud
// container carries in its environment, replacing any set before.
func (w *world) thisHostRunsAsCloudSession(id string) error {
	section[missingHerdrState](w).env = []string{cloudSessionEnv + "=" + id}

	return nil
}

// originsClaimMarkerNamesHolder reads the bare remote's plan branch:
// the claim marker's holder line carries the session, not the host.
func (w *world) originsClaimMarkerNamesHolder(id, holder string) error {
	body, err := gitCapture(w.t, originPath(w), "log", "-1", "--format=%B",
		"refs/heads/plan/"+id)
	if err != nil {
		return err
	}
	if !markerNamesHolder(body, holder) {
		return fmt.Errorf("origin's claim marker does not name holder %q: %s",
			holder, body)
	}

	return nil
}

// theBuiltFritRunsFromTheLaneExpectingFailure runs from C15's lane
// and lets the run exit non-zero, for a step that reads its refusal.
func (w *world) theBuiltFritRunsFromTheLaneExpectingFailure(args string) error {
	st := section[missingHerdrState](w)
	st.expectFail = true
	defer func() { st.expectFail = false }()

	return w.theBuiltFritRunsFromTheLane(args)
}

// itRefusesTheLaneAsHeldBy reads the run's stderr: the preflight that
// keeps a checkout from handing out another host's lane named the
// session holding it.
func (w *world) itRefusesTheLaneAsHeldBy(holder string) error {
	want := "the current worktree stands on a claim held by " + holder
	if got := section[commandState](w).errb.String(); !strings.Contains(got, want) {
		return fmt.Errorf("expected %q on stderr, got: %s", want, got)
	}

	return nil
}

// markerNamesHolder reports whether a marker body's holder line, its
// value padded to align with the other keys, carries exactly holder.
func markerNamesHolder(body, holder string) bool {
	for _, line := range strings.Split(body, "\n") {
		if f := strings.Fields(line); len(f) == 2 && f[0] == "holder:" {
			return f[1] == holder
		}
	}

	return false
}

// cloudProxyHook refuses what a Claude cloud session's git proxy was
// seen to refuse with a 403: any ref outside refs/heads/, and any
// delete. Everything else is admitted.
const cloudProxyHook = `#!/bin/sh
zero=0000000000000000000000000000000000000000
while read old new ref; do
	case "$ref" in
	refs/heads/*) ;;
	*) echo "refused: $ref is outside refs/heads" >&2; exit 1 ;;
	esac
	if [ "$new" = "$zero" ]; then
		echo "refused: deleting $ref" >&2
		exit 1
	fi
done
`

// originRefusesWhatTheCloudProxyRefuses installs that rule as the bare
// remote's pre-receive hook, so the built frit pushes into a remote
// that answers as the cloud session's proxy does.
func (w *world) originRefusesWhatTheCloudProxyRefuses() error {
	hook := filepath.Join(originPath(w), "hooks", "pre-receive")

	return os.WriteFile(hook, []byte(cloudProxyHook), 0o700) //nolint:gosec // a hook must be executable
}

// originCarriesTheRescueAsABranch reads the bare remote: the park
// landed under refs/heads/frit-rescue/<id>/, and nothing under the
// refs/frit/ namespace the proxy refuses.
func (w *world) originCarriesTheRescueAsABranch(id string) error {
	branches, err := gitCapture(w.t, originPath(w), "for-each-ref",
		"--format=%(refname)", "refs/heads/frit-rescue/"+id+"/")
	if err != nil {
		return err
	}
	if strings.TrimSpace(branches) == "" {
		return fmt.Errorf("origin carries no rescue branch for plan %s", id)
	}
	legacy, err := gitCapture(w.t, originPath(w), "for-each-ref",
		"--format=%(refname)", "refs/frit/")
	if err != nil {
		return err
	}
	if strings.TrimSpace(legacy) != "" {
		return fmt.Errorf("origin carries refs the proxy refuses: %s", legacy)
	}

	return nil
}
