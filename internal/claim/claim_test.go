package claim

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jeduden/frit/internal/gitwt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// gitCmd runs git in dir for test setup, failing loudly. Signing is off
// so a developer's global commit.gpgsign does not stall the marker.
func gitCmd(t *testing.T, dir string, args ...string) string {
	t.Helper()
	full := append([]string{"-C", dir, "-c", "commit.gpgsign=false"}, args...)
	out, err := exec.Command("git", full...).CombinedOutput()
	require.NoError(t, err, "git %v: %s", args, out)

	return strings.TrimSpace(string(out))
}

// gitCapture runs git in dir and returns its output and error, for
// asserting on a ref that should or should not exist.
func gitCapture(t *testing.T, dir string, args ...string) (string, error) {
	t.Helper()
	full := append([]string{"-C", dir}, args...)
	out, err := exec.Command("git", full...).CombinedOutput()

	return strings.TrimSpace(string(out)), err
}

// originAndClone builds a bare origin.git with a working clone that has
// one commit on main pushed to it, and returns the working repo dir.
func originAndClone(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	origin := filepath.Join(root, "origin.git")
	gitCmd(t, root, "init", "-q", "--bare", "-b", "main", origin)

	work := filepath.Join(root, "work")
	require.NoError(t, os.Mkdir(work, 0o750))
	gitCmd(t, work, "init", "-q", "-b", "main")
	gitCmd(t, work, "config", "user.email", "t@example.com")
	gitCmd(t, work, "config", "user.name", "frit-test")
	gitCmd(t, work, "config", "commit.gpgsign", "false")
	require.NoError(t, os.WriteFile(
		filepath.Join(work, "README.md"), []byte("x\n"), 0o600))
	gitCmd(t, work, "add", "-A")
	gitCmd(t, work, "commit", "-q", "-m", "init")
	gitCmd(t, work, "remote", "add", "origin", origin)
	gitCmd(t, work, "push", "-q", "origin", "main")

	return work
}

// cloneAgain makes a second working clone of the same origin the first
// clone in work points at, so two machines can race for one plan.
func cloneAgain(t *testing.T, work string) string {
	t.Helper()
	origin := gitCmd(t, work, "config", "--get", "remote.origin.url")
	dst := t.TempDir()
	gitCmd(t, dst, "clone", "-q", origin, dst)
	gitCmd(t, dst, "config", "user.email", "t2@example.com")
	gitCmd(t, dst, "config", "user.name", "frit-test-2")
	gitCmd(t, dst, "config", "commit.gpgsign", "false")

	return dst
}

func sampleOptions() Options {
	return Options{
		Branch:   "plan/7-shader-unit",
		Base:     "origin/main",
		Remote:   "origin",
		PlanID:   7,
		PlanFile: "plan/7-shader-unit.md",
		Lane:     "/work/lanes/shader-unit",
		Host:     "mm-box",
	}
}

// TestBranchDerivesTheHoldName: the lease branch is plan/<id>-<slug>,
// with the slug taken from the plan file after its id prefix. A file
// with no underscore contributes its whole stem.
func TestBranchDerivesTheHoldName(t *testing.T) {
	assert.Equal(t, "plan/7-shader-unit",
		Branch(7, "plan/7_shader-unit.md"))
	assert.Equal(t, "plan/2608161810-dispatch-ladder",
		Branch(2608161810, "plan/2608161810_dispatch-ladder.md"))
	assert.Equal(t, "plan/42-notes",
		Branch(42, "docs/notes.md"),
		"a file with no id prefix contributes its whole stem")
}

// TestMintCreatesTheLease: the claim ref is minted both locally and on
// origin, and the Result is dated against the base commit.
func TestMintCreatesTheLease(t *testing.T) {
	work := originAndClone(t)
	baseSHA := gitCmd(t, work, "rev-parse", "origin/main")

	res, err := Mint(work, sampleOptions(), gitwt.Exec)
	require.NoError(t, err)

	assert.Equal(t, "plan/7-shader-unit", res.Branch)
	assert.Equal(t, baseSHA, res.BaseSHA)

	local := gitCmd(t, work, "rev-parse", "refs/heads/plan/7-shader-unit")
	remote := gitCmd(t, work,
		"ls-remote", "origin", "refs/heads/plan/7-shader-unit")
	assert.NotEmpty(t, local)
	assert.Contains(t, remote, local,
		"the same marker is on origin as locally")
}

// TestMintIsAnEmptyMarker: the marker's tree equals the base tree, so
// the claim touched no file — it is only a lease.
func TestMintIsAnEmptyMarker(t *testing.T) {
	work := originAndClone(t)
	baseTree := gitCmd(t, work, "rev-parse", "origin/main^{tree}")

	_, err := Mint(work, sampleOptions(), gitwt.Exec)
	require.NoError(t, err)

	markerTree := gitCmd(t, work,
		"rev-parse", "refs/heads/plan/7-shader-unit^{tree}")
	assert.Equal(t, baseTree, markerTree)
}

// TestMintLosesTheRaceAndRollsBack: once the first clone has planted the
// ref on origin, the second clone's Mint must lose and leave no local
// ref behind, so a retry starts clean.
func TestMintLosesTheRaceAndRollsBack(t *testing.T) {
	first := originAndClone(t)
	second := cloneAgain(t, first)

	_, err := Mint(first, sampleOptions(), gitwt.Exec)
	require.NoError(t, err)

	_, err = Mint(second, sampleOptions(), gitwt.Exec)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "lost the race for plan 7")

	verify := exec.Command("git", "-C", second,
		"rev-parse", "--verify", "-q", "refs/heads/plan/7-shader-unit")
	require.Error(t, verify.Run(),
		"the local ref was rolled back after losing the race")
}

// TestMintReportsANonRaceFailure: a push that fails for a reason other
// than a pre-existing ref is a real fault, not a lost race, and the local
// ref is still rolled back.
func TestMintReportsANonRaceFailure(t *testing.T) {
	work := originAndClone(t)
	opts := sampleOptions()
	opts.Remote = "no-such-remote" // never configured, so the push faults

	_, err := Mint(work, opts, gitwt.Exec)

	require.Error(t, err)
	assert.False(t, errors.Is(err, ErrLostRace),
		"an unreachable remote is not another machine winning the race")

	_, refErr := gitCapture(t, work,
		"rev-parse", "--verify", "refs/heads/plan/7-shader-unit")
	assert.Error(t, refErr, "the local ref was rolled back")
}

// TestMintClassifiesByTheRemoteRefNotTheErrorText: a push a server hook
// rejects is a real fault, even when the rejection text happens to carry
// race-like words like "already exists". The claim is lost only when the
// hold ref actually exists on the remote — so classification asks the
// remote, never git's human-readable stderr.
func TestMintClassifiesByTheRemoteRefNotTheErrorText(t *testing.T) {
	work := originAndClone(t)
	origin := gitCmd(t, work, "config", "--get", "remote.origin.url")
	// A hook that declines every push with wording that would fool a
	// stderr match. It never creates the ref, so this is a fault.
	hook := filepath.Join(origin, "hooks", "pre-receive")
	require.NoError(t, os.WriteFile(hook,
		[]byte("#!/bin/sh\necho 'error: object already exists' >&2\nexit 1\n"),
		0o755))

	_, err := Mint(work, sampleOptions(), gitwt.Exec)

	require.Error(t, err)
	assert.False(t, errors.Is(err, ErrLostRace),
		"a hook decline is a fault, not another machine winning")

	remote, lsErr := gitCapture(t, work,
		"ls-remote", "origin", "refs/heads/plan/7-shader-unit")
	require.NoError(t, lsErr)
	assert.Empty(t, remote, "the push was declined, so no ref is on origin")
}

// TestMintKeepsAClaimTheRemoteAcceptedDespiteAClientError: if the push
// plants our own marker on the remote but still reports an error — a
// connection dropped after the ref transaction committed — the claim is
// ours, not a lost race. Classification compares the remote's ref to our
// marker, so our own commit reads as a win rather than a competitor's.
func TestMintKeepsAClaimTheRemoteAcceptedDespiteAClientError(t *testing.T) {
	marker := ""
	run := func(_ string, args ...string) ([]byte, error) {
		switch args[0] {
		case "rev-parse":
			return []byte("basesha\n"), nil // the base, then its tree
		case "commit-tree":
			marker = "markersha"
			return []byte(marker + "\n"), nil
		case "update-ref":
			return nil, nil
		case "push":
			return nil, errors.New("fatal: the remote end hung up")
		case "ls-remote":
			return []byte(marker + "\trefs/heads/plan/7-shader-unit\n"), nil
		}

		return nil, nil
	}

	res, err := Mint("/repo", sampleOptions(), run)

	require.NoError(t, err, "our own marker on the remote is a win, not a fault")
	assert.Equal(t, "plan/7-shader-unit", res.Branch)
	assert.Equal(t, "basesha", res.BaseSHA)
}

// TestMintNamesAClaimHeldOnThisHost: a claim lost to a ref this same
// machine minted — the stale-status retry — reads its marker and reports
// the holder as this host, not a competitor that is not there.
func TestMintNamesAClaimHeldOnThisHost(t *testing.T) {
	first := originAndClone(t)
	second := cloneAgain(t, first)

	a := sampleOptions()
	a.Host = "box-a"
	_, err := Mint(first, a, gitwt.Exec)
	require.NoError(t, err)

	b := sampleOptions()
	b.Host = "box-a" // the same machine, retrying after a partial success
	_, err = Mint(second, b, gitwt.Exec)

	var lost *LostRaceError
	require.ErrorAs(t, err, &lost)
	require.True(t, lost.Holder.Known, "the holder marker was read")
	assert.True(t, lost.Holder.ThisHost, "the holder is this host")
	assert.Equal(t, "box-a", lost.Holder.Host)
	assert.False(t, lost.Holder.Landed, "a fresh marker is not merged")
}

// TestMintNamesAClaimHeldElsewhere: a claim lost to another machine's ref
// reads that marker's host and names the machine, so a genuine race still
// reports the truth.
func TestMintNamesAClaimHeldElsewhere(t *testing.T) {
	first := originAndClone(t)
	second := cloneAgain(t, first)

	a := sampleOptions()
	a.Host = "box-a"
	_, err := Mint(first, a, gitwt.Exec)
	require.NoError(t, err)

	b := sampleOptions()
	b.Host = "box-b" // a different machine really racing
	_, err = Mint(second, b, gitwt.Exec)

	var lost *LostRaceError
	require.ErrorAs(t, err, &lost)
	require.True(t, lost.Holder.Known)
	assert.False(t, lost.Holder.ThisHost, "the holder is another machine")
	assert.Equal(t, "box-a", lost.Holder.Host)
	assert.False(t, lost.Holder.Landed)
}

// TestMintReadsHostFromMarkerUnderWorkCommits: an active lane advances its
// branch past the marker with real work, so the hold ref's tip carries no
// host line. The host is still read from the marker beneath the work, so a
// lost race to this host's own in-progress lane names this host rather than
// misreading an empty host as another machine.
func TestMintReadsHostFromMarkerUnderWorkCommits(t *testing.T) {
	first := originAndClone(t)

	a := sampleOptions()
	a.Host = "box-a"
	_, err := Mint(first, a, gitwt.Exec)
	require.NoError(t, err)

	// The lane does real work on top of its marker and pushes it, so the
	// remote hold ref's tip is a work commit with no host line.
	gitCmd(t, first, "checkout", "-q", "plan/7-shader-unit")
	require.NoError(t, os.WriteFile(
		filepath.Join(first, "w.txt"), []byte("z\n"), 0o600))
	gitCmd(t, first, "add", "-A")
	gitCmd(t, first, "commit", "-q", "-m", "real work, no host line")
	gitCmd(t, first, "push", "-q", "origin", "plan/7-shader-unit")
	gitCmd(t, first, "checkout", "-q", "main")

	second := cloneAgain(t, first)
	b := sampleOptions()
	b.Host = "box-a" // the same machine, retrying
	_, err = Mint(second, b, gitwt.Exec)

	var lost *LostRaceError
	require.ErrorAs(t, err, &lost)
	require.True(t, lost.Holder.Known)
	assert.Equal(t, "box-a", lost.Holder.Host,
		"the host is read from the marker, not the work tip")
	assert.True(t, lost.Holder.ThisHost)
	assert.False(t, lost.Holder.Landed, "the lane has not merged")
}

// TestMintNamesALandedClaim: a push rejected by a lane whose work already
// merged into the base is landed work with a stale status, not a live
// competitor. The lane carries frit's marker, so the holder is known, and
// its work is an ancestor of the base, so Landed reads true.
func TestMintNamesALandedClaim(t *testing.T) {
	work := originAndClone(t)

	// Mint a real claim so the branch carries frit's marker, do work on top,
	// merge it no-ff into main, and leave the branch on the remote — a plan
	// that landed but was never set to ✅.
	_, err := Mint(work, sampleOptions(), gitwt.Exec)
	require.NoError(t, err)
	gitCmd(t, work, "checkout", "-q", "plan/7-shader-unit")
	require.NoError(t, os.WriteFile(
		filepath.Join(work, "f.txt"), []byte("y\n"), 0o600))
	gitCmd(t, work, "add", "-A")
	gitCmd(t, work, "commit", "-q", "-m", "work on plan 7")
	gitCmd(t, work, "push", "-q", "origin", "plan/7-shader-unit")
	gitCmd(t, work, "checkout", "-q", "main")
	gitCmd(t, work, "merge", "-q", "--no-ff", "-m", "land plan 7",
		"plan/7-shader-unit")
	gitCmd(t, work, "push", "-q", "origin", "main")

	// A fresh claim attempt loses to the branch still on the remote.
	_, err = Mint(work, sampleOptions(), gitwt.Exec)

	var lost *LostRaceError
	require.ErrorAs(t, err, &lost)
	require.True(t, lost.Holder.Known)
	assert.True(t, lost.Holder.Landed, "the holder's work is merged into the base")
}

// TestMintNamesALandedClaimMergedElsewhere: a lane merged on another
// machine still reads as landed even when this machine's view of the
// default branch predates the merge — the landed check refreshes the base
// rather than trusting a stale local origin/main.
func TestMintNamesALandedClaimMergedElsewhere(t *testing.T) {
	first := originAndClone(t)

	_, err := Mint(first, sampleOptions(), gitwt.Exec)
	require.NoError(t, err)
	gitCmd(t, first, "checkout", "-q", "plan/7-shader-unit")
	require.NoError(t, os.WriteFile(
		filepath.Join(first, "f.txt"), []byte("y\n"), 0o600))
	gitCmd(t, first, "add", "-A")
	gitCmd(t, first, "commit", "-q", "-m", "work on plan 7")
	gitCmd(t, first, "push", "-q", "origin", "plan/7-shader-unit")

	// second clones after the branch exists but before the merge, so its
	// origin/main goes stale the moment first lands the work.
	second := cloneAgain(t, first)

	gitCmd(t, first, "checkout", "-q", "main")
	gitCmd(t, first, "merge", "-q", "--no-ff", "-m", "land plan 7",
		"plan/7-shader-unit")
	gitCmd(t, first, "push", "-q", "origin", "main")

	b := sampleOptions()
	b.Host = "box-b"
	_, err = Mint(second, b, gitwt.Exec)

	var lost *LostRaceError
	require.ErrorAs(t, err, &lost)
	require.True(t, lost.Holder.Known)
	assert.True(t, lost.Holder.Landed,
		"a branch merged on another machine still reads as landed")
}

// TestMarkerHost reads the host line from a marker body, and reports "" for
// a plain work commit that carries none.
func TestMarkerHost(t *testing.T) {
	body := "plan 7: claim shader-unit\n\n" +
		"lane:     /work/lanes/shader-unit\n" +
		"host:     mm-box\n" +
		"base:     abc123\n" +
		"plan:     plan/7-shader-unit.md"
	assert.Equal(t, "mm-box", markerHost(body))
	assert.Empty(t, markerHost("real work, no host line"))

	lease := "plan 7: claim\n\n" +
		"epoch:   1\n" +
		"nonce:   cafe\n" +
		"holder:  box-a\n" +
		"lane:    /lanes/a\n" +
		"session: -\n" +
		"base:    abc123"
	assert.Equal(t, "box-a", markerHost(lease),
		"a lease marker's holder trailer reads as the host")
}

// TestBaseBranch reduces every base-ref shape to the remote branch name a
// fresh landed check fetches, and leaves a bare name unchanged.
func TestBaseBranch(t *testing.T) {
	assert.Equal(t, "main", baseBranch("refs/remotes/origin/main", "origin"))
	assert.Equal(t, "main", baseBranch("origin/main", "origin"))
	assert.Equal(t, "main", baseBranch("refs/heads/main", "origin"))
	assert.Equal(t, "main", baseBranch("main", "origin"))
}

// TestIsAncestor: a zero exit reads as merged, any non-zero exit as not.
func TestIsAncestor(t *testing.T) {
	yes := func(_ string, _ ...string) ([]byte, error) { return nil, nil }
	no := func(_ string, _ ...string) ([]byte, error) {
		return nil, errors.New("not an ancestor")
	}
	assert.True(t, isAncestor("/r", "a", "b", yes))
	assert.False(t, isAncestor("/r", "a", "b", no))
}

// TestHolderMarker returns the marker body on a match and reports not-found
// when the grep selects nothing or the object is missing.
func TestHolderMarker(t *testing.T) {
	found := func(_ string, _ ...string) ([]byte, error) {
		return []byte("plan 7: claim x\n\nhost:     mm-box\n"), nil
	}
	body, ok := holderMarker("/r", sampleOptions(), "tip", found)
	require.True(t, ok)
	assert.Contains(t, body, "host:     mm-box")

	missing := func(_ string, _ ...string) ([]byte, error) {
		return nil, errors.New("bad object tip")
	}
	_, ok = holderMarker("/r", sampleOptions(), "tip", missing)
	assert.False(t, ok)

	empty := func(_ string, _ ...string) ([]byte, error) { return nil, nil }
	_, ok = holderMarker("/r", sampleOptions(), "tip", empty)
	assert.False(t, ok, "an empty body is not a marker")
}

// TestLanded fetches the base then asks merge-base; the merge-base answer
// decides, and a failed fetch falls back to the local base rather than
// failing the check.
func TestLanded(t *testing.T) {
	all := func(_ string, _ ...string) ([]byte, error) { return nil, nil }
	assert.True(t, landed("/r", sampleOptions(), "tip", all))

	notMerged := func(_ string, args ...string) ([]byte, error) {
		if args[0] == "merge-base" {
			return nil, errors.New("not an ancestor")
		}
		return nil, nil
	}
	assert.False(t, landed("/r", sampleOptions(), "tip", notMerged))
}

// TestLostRaceErrorMessage pins the sentinel wording and that the error
// still unwraps to ErrLostRace.
func TestLostRaceErrorMessage(t *testing.T) {
	e := &LostRaceError{PlanID: 7}
	assert.Equal(t,
		"lost the race for plan 7: the claim ref already exists", e.Error())
	assert.True(t, errors.Is(e, ErrLostRace))
}

// TestMintFallsBackWhenTheHolderMarkerIsUnreadable: a lost race whose
// holder marker cannot be read — the object is missing and the fetch fails
// — reports an unknown holder, so the caller falls back to today's wording
// rather than failing the command.
func TestMintFallsBackWhenTheHolderMarkerIsUnreadable(t *testing.T) {
	run := func(_ string, args ...string) ([]byte, error) {
		switch args[0] {
		case "rev-parse":
			return []byte("basesha\n"), nil
		case "commit-tree":
			return []byte("markersha\n"), nil
		case "update-ref":
			return nil, nil
		case "push":
			return nil, errors.New("rejected: already exists")
		case "ls-remote":
			return []byte("othersha\trefs/heads/plan/7-shader-unit\n"), nil
		case "fetch":
			return nil, errors.New("could not fetch the holder ref")
		case "log":
			return nil, errors.New("missing object")
		}

		return nil, nil
	}

	_, err := Mint("/repo", sampleOptions(), run)

	var lost *LostRaceError
	require.ErrorAs(t, err, &lost)
	assert.True(t, errors.Is(err, ErrLostRace),
		"the error still wraps the sentinel")
	assert.False(t, lost.Holder.Known, "an unreadable marker is not known")
}

// TestMintWinningPathReadsNoHolder: a push that lands runs none of the
// holder-reading git calls, so naming a holder costs nothing on the win.
func TestMintWinningPathReadsNoHolder(t *testing.T) {
	var calls []string
	run := func(_ string, args ...string) ([]byte, error) {
		calls = append(calls, args[0])
		switch args[0] {
		case "rev-parse":
			return []byte("basesha\n"), nil
		case "commit-tree":
			return []byte("markersha\n"), nil
		case "update-ref", "push":
			return nil, nil // the push succeeds
		}

		return nil, nil
	}

	_, err := Mint("/repo", sampleOptions(), run)

	require.NoError(t, err)
	for _, c := range calls {
		assert.NotContains(t,
			[]string{"ls-remote", "fetch", "log", "merge-base"}, c,
			"the winning path reads no holder")
	}
}

// TestDropDeletesTheClaim: a minted legacy claim can be unwound, leaving
// no ref locally or on the remote for a lane that never stood up.
func TestDropDeletesTheClaim(t *testing.T) {
	work := originAndClone(t)
	_, err := Mint(work, sampleOptions(), gitwt.Exec)
	require.NoError(t, err)

	require.NoError(t,
		Drop(work, "plan/7-shader-unit", "origin", gitwt.Exec))

	_, localErr := gitCapture(t, work,
		"rev-parse", "--verify", "refs/heads/plan/7-shader-unit")
	assert.Error(t, localErr, "the local ref is gone")
	remote, err := gitCapture(t, work,
		"ls-remote", "origin", "refs/heads/plan/7-shader-unit")
	require.NoError(t, err)
	assert.Empty(t, remote, "the remote ref is gone")
}

// TestMarkerMessage pins the exact commit body, including the empty-Lane
// case that records "-" for both the title and the lane line.
func TestMarkerMessage(t *testing.T) {
	got := markerMessage(sampleOptions(), "abc123")
	want := "plan 7: claim shader-unit\n\n" +
		"lane:     /work/lanes/shader-unit\n" +
		"host:     mm-box\n" +
		"base:     abc123\n" +
		"plan:     plan/7-shader-unit.md"
	assert.Equal(t, want, got)

	opts := sampleOptions()
	opts.Lane = ""
	gotEmpty := markerMessage(opts, "abc123")
	wantEmpty := "plan 7: claim -\n\n" +
		"lane:     -\n" +
		"host:     mm-box\n" +
		"base:     abc123\n" +
		"plan:     plan/7-shader-unit.md"
	assert.Equal(t, wantEmpty, gotEmpty)
}
