package main

import (
	"fmt"
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
