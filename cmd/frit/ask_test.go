package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jeduden/frit/internal/ask"
	"github.com/jeduden/frit/internal/discovery"
	"github.com/jeduden/frit/internal/fleet"
	"github.com/jeduden/frit/internal/gitwt"
	"github.com/jeduden/frit/internal/herdr"
	"github.com/jeduden/frit/internal/report"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// askRecord reads the ask record a repository keeps for a plan, with
// whether one exists at all.
func askRecord(t *testing.T, repo string, id int64) (ask.Record, bool) {
	t.Helper()
	path, err := ask.Path(repo, id, gitwt.Exec)
	require.NoError(t, err)
	rec, ok, err := ask.Read(path)
	require.NoError(t, err)

	return rec, ok
}

// TestMessageAskDryRunShowsTheEnvelopeAndRecordsNothing: --ask without
// --go prints the envelope — the text, that a reply is wanted, the
// skill to load, the raw reply command — and neither sends nor
// records an ask.
func TestMessageAskDryRunShowsTheEnvelopeAndRecordsNothing(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := heldPlan(t, root, "atlas", 7, "Dispatch me")
	runner, rec := recordingHerdr(workingLane(repo))
	withHerdr(t, runner)
	var out, errb bytes.Buffer

	code := run([]string{"message", "7", "--ask", "are you in a PR?",
		"--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	assert.False(t, rec.verb("agent", "prompt"), "dry-run sends nothing")
	assert.Contains(t, out.String(), "are you in a PR?")
	assert.Contains(t, out.String(), "reply is wanted")
	assert.Contains(t, out.String(), "plan-reply")
	assert.Contains(t, out.String(), "frit reply")
	_, ok := askRecord(t, repo, 7)
	assert.False(t, ok, "a dry run records no ask")
}

// TestMessageAskGoSendsTheEnvelopeWholeAndRecordsAPendingAsk: under
// --go the pane receives the envelope as one argument and one pending
// ask record exists for the lane to answer.
func TestMessageAskGoSendsTheEnvelopeWholeAndRecordsAPendingAsk(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := heldPlan(t, root, "atlas", 7, "Dispatch me")
	runner, rec := recordingHerdr(workingLane(repo))
	withHerdr(t, runner)
	var out, errb bytes.Buffer

	code := run([]string{"message", "7", "--ask", "are you in a PR?",
		"--go", "--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	assert.True(t, rec.verb("agent", "prompt", "wC:p1",
		ask.Envelope("are you in a PR?")), "the envelope goes whole")
	got, ok := askRecord(t, repo, 7)
	require.True(t, ok, "the ask is recorded")
	assert.Equal(t, ask.StatePending, ask.StateOf(got, ok))
	assert.Equal(t, "are you in a PR?", got.Question)
}

// TestMessageAskEmitsTheEnvelopeBesideTheText: --json keeps the
// operator's own words in text and carries what goes in envelope.
func TestMessageAskEmitsTheEnvelopeBesideTheText(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := heldPlan(t, root, "atlas", 7, "Dispatch me")
	runner, _ := recordingHerdr(workingLane(repo))
	withHerdr(t, runner)
	var doc report.MessageDoc

	emit(t, &doc, "message", "7", "--ask", "status?", "--root", root)

	assert.True(t, doc.Ask)
	assert.Equal(t, "status?", doc.Text)
	assert.Equal(t, ask.Envelope("status?"), doc.Envelope)
}

// TestMessageWithoutAskSendsItsTextAsTheEnvelope: a plain message is
// untouched — its envelope is its text, and no ask is recorded.
func TestMessageWithoutAskSendsItsTextAsTheEnvelope(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := heldPlan(t, root, "atlas", 7, "Dispatch me")
	runner, _ := recordingHerdr(workingLane(repo))
	withHerdr(t, runner)
	var doc report.MessageDoc

	emit(t, &doc, "message", "7", "status?", "--go", "--root", root)

	assert.False(t, doc.Ask)
	assert.Equal(t, "status?", doc.Envelope)
	_, ok := askRecord(t, repo, 7)
	assert.False(t, ok, "a plain message records no ask")
}

// TestMessageAskGoWithdrawsTheAskWhenTheSendFails: the record is
// written before the send, so a fast reply always finds it — and taken
// back when the send fails, so the lane owes no reply to a question it
// never saw.
func TestMessageAskGoWithdrawsTheAskWhenTheSendFails(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := heldPlan(t, root, "atlas", 7, "Dispatch me")
	base, _ := recordingHerdr(workingLane(repo))
	withHerdr(t, func(args ...string) ([]byte, error) {
		if len(args) >= 2 && args[0] == "agent" && args[1] == "prompt" {
			return nil, errors.New("boom")
		}

		return base(args...)
	})
	var out, errb bytes.Buffer

	code := run([]string{"message", "7", "--ask", "status?", "--go",
		"--root", root}, &out, &errb)

	require.Equal(t, 1, code)
	assert.Contains(t, errb.String(), "prompt")
	_, ok := askRecord(t, repo, 7)
	assert.False(t, ok, "a failed send leaves no pending ask")
}

// TestMessageAskGoSendsNothingWhenTheAskCannotBeRecorded: an ask whose
// record cannot be written is never sent, since its reply would have
// nowhere to land.
func TestMessageAskGoSendsNothingWhenTheAskCannotBeRecorded(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := heldPlan(t, root, "atlas", 7, "Dispatch me")
	common, err := gitwt.CommonDir(repo, gitwt.Exec)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(common, "frit"), nil, 0o600))
	runner, rec := recordingHerdr(workingLane(repo))
	withHerdr(t, runner)
	var out, errb bytes.Buffer

	code := run([]string{"message", "7", "--ask", "status?", "--go",
		"--root", root}, &out, &errb)

	require.Equal(t, 1, code)
	assert.Contains(t, errb.String(), "record the ask")
	assert.False(t, rec.verb("agent", "prompt"), "nothing is sent")
}

// TestAskRefusalNamesALaneOnAnotherHost: the ask record is a local
// file, so an ask reaches only a lane on this host; a remote lane is
// refused by name rather than left pending forever.
func TestAskRefusalNamesALaneOnAnotherHost(t *testing.T) {
	local := herdr.Lane{Pane: herdr.Pane{PaneID: "wC:p1"}, Branch: "plan/7"}
	remote := herdr.Lane{
		Pane: herdr.Pane{PaneID: "wC:p1", Host: "box"}, Branch: "plan/7",
	}

	assert.Empty(t, askRefusal(local))
	assert.Contains(t, askRefusal(remote), "box")
	assert.Contains(t, askRefusal(remote), "this host")
}

// TestMessageSendRefusesAnAskToALaneOnAnotherHost: messageSend applies
// askRefusal before anything is recorded or sent.
func TestMessageSendRefusesAnAskToALaneOnAnotherHost(t *testing.T) {
	runner, rec := recordingHerdr()
	rt := &runtime{git: gitwt.Exec, herdr: runner}
	m := &messageCmd{Selector: "7", Text: "status?", Ask: true, Go: true}
	doc := report.NewMessage("/fleet", "atlas", 7, "t", "status?", true)
	lane := herdr.Lane{
		Pane: herdr.Pane{
			PaneID: "wC:p1", Host: "box", Agent: "claude",
			Status: herdr.StatusWorking,
		},
		Root: t.TempDir(), Branch: "plan/7",
	}

	err := messageSend(rt, m, doc, discovery.Plan{ID: 7}, lane, true)

	require.NoError(t, err)
	assert.Contains(t, doc.Refused, "box")
	assert.False(t, doc.Sent)
	assert.False(t, rec.verb("agent", "prompt"))
}

// TestReplyRecordsTheAnswerFromTheLane: run from the lane with no plan
// argument, reply finds the plan from the checkout and records the
// answer against the pending ask — no --go, no herdr call at all.
func TestReplyRecordsTheAnswerFromTheLane(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := heldPlan(t, root, "atlas", 7, "Dispatch me")
	_, err := ask.Pose(repo, 7, "are you in a PR?", time.Now(), gitwt.Exec)
	require.NoError(t, err)
	runner, rec := recordingHerdr()
	withHerdr(t, runner)
	t.Chdir(repo)
	var out, errb bytes.Buffer

	code := run([]string{"reply", "in PR #9"}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	assert.Contains(t, out.String(), "in PR #9")
	assert.Contains(t, out.String(), "are you in a PR?")
	got, ok := askRecord(t, repo, 7)
	assert.Equal(t, ask.StateAnswered, ask.StateOf(got, ok))
	assert.Equal(t, "in PR #9", got.Answer)
	assert.Empty(t, rec.calls, "reply never reaches herdr")
}

// TestReplyTouchesNoRef: the reply is a local write, so every ref in
// the repository reads the same before and after.
func TestReplyTouchesNoRef(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := heldPlan(t, root, "atlas", 7, "Dispatch me")
	_, err := ask.Pose(repo, 7, "status?", time.Now(), gitwt.Exec)
	require.NoError(t, err)
	before, err := gitCapture(t, repo, "for-each-ref", "--format=%(refname) %(objectname)")
	require.NoError(t, err)
	t.Chdir(repo)
	var out, errb bytes.Buffer

	code := run([]string{"reply", "done"}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	after, err := gitCapture(t, repo, "for-each-ref", "--format=%(refname) %(objectname)")
	require.NoError(t, err)
	assert.Equal(t, before, after)
}

// TestReplyRefusesWithNoAskPending: nothing asked, nothing recorded —
// the refusal says why.
func TestReplyRefusesWithNoAskPending(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := heldPlan(t, root, "atlas", 7, "Dispatch me")
	t.Chdir(repo)
	var doc report.ReplyDoc

	emit(t, &doc, "reply", "in PR #9")

	assert.False(t, doc.Recorded)
	assert.Contains(t, doc.Refused, "no ask is pending for plan 7")
	_, ok := askRecord(t, repo, 7)
	assert.False(t, ok, "a refused reply writes nothing")
}

// TestReplyPrintsItsRefusal: the table rendering says the reply was
// refused and why.
func TestReplyPrintsItsRefusal(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := heldPlan(t, root, "atlas", 7, "Dispatch me")
	t.Chdir(repo)
	var out, errb bytes.Buffer

	code := run([]string{"reply", "in PR #9"}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	assert.Contains(t, out.String(), "refused: no ask is pending for plan 7")
}

// TestReplyEmitsJSON pins the fields a responder's skill reads: the
// plan, the question settled, the answer and recorded.
func TestReplyEmitsJSON(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := heldPlan(t, root, "atlas", 7, "Dispatch me")
	_, err := ask.Pose(repo, 7, "status?", time.Now(), gitwt.Exec)
	require.NoError(t, err)
	t.Chdir(repo)
	var doc report.ReplyDoc

	emit(t, &doc, "reply", "in PR #9")

	assert.Equal(t, "reply", doc.Command)
	assert.Equal(t, "atlas", doc.Repo)
	assert.Equal(t, int64(7), doc.ID)
	assert.Equal(t, "status?", doc.Question)
	assert.Equal(t, "in PR #9", doc.Answer)
	assert.True(t, doc.Recorded)
}

// TestReplyTakesAnExplicitPlan: --plan answers a plan the cwd's branch
// does not name, in the cwd's own repository.
func TestReplyTakesAnExplicitPlan(t *testing.T) {
	isolate(t)
	repo := initRepo(t, t.TempDir(), "atlas")
	_, err := ask.Pose(repo, 99, "status?", time.Now(), gitwt.Exec)
	require.NoError(t, err)
	t.Chdir(repo)
	var doc report.ReplyDoc

	emit(t, &doc, "reply", "--plan", "99", "landed")

	assert.True(t, doc.Recorded)
	assert.Equal(t, int64(99), doc.ID)
	got, ok := askRecord(t, repo, 99)
	assert.Equal(t, ask.StateAnswered, ask.StateOf(got, ok))
}

// TestReplyFailsWithNoPlanInferred: off any lane and with no --plan,
// there is no ask to answer.
func TestReplyFailsWithNoPlanInferred(t *testing.T) {
	isolate(t)
	repo := initRepo(t, t.TempDir(), "atlas")
	t.Chdir(repo)
	var out, errb bytes.Buffer

	code := run([]string{"reply", "landed"}, &out, &errb)

	require.Equal(t, 1, code)
	assert.Contains(t, errb.String(), "no plan given")
}

// TestReplyRefusesEmptyText: an empty answer is never a real reply.
func TestReplyRefusesEmptyText(t *testing.T) {
	isolate(t)
	var out, errb bytes.Buffer

	code := run([]string{"reply", ""}, &out, &errb)

	require.Equal(t, 1, code)
	assert.Contains(t, errb.String(), "reply requires an answer")
}

// TestReplySurfacesAnUnreadableRecord: a torn record is a fault, not
// "no ask pending".
func TestReplySurfacesAnUnreadableRecord(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := heldPlan(t, root, "atlas", 7, "Dispatch me")
	path, err := ask.Path(repo, 7, gitwt.Exec)
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o750))
	require.NoError(t, os.WriteFile(path, []byte("{torn"), 0o600))
	t.Chdir(repo)
	var out, errb bytes.Buffer

	code := run([]string{"reply", "x"}, &out, &errb)

	require.Equal(t, 1, code)
	assert.NotEmpty(t, errb.String())
}

// boardRow finds plan id's row in a board document.
func boardRow(t *testing.T, doc report.BoardDoc, id int64) report.BoardPlan {
	t.Helper()
	for _, p := range doc.Plans {
		if p.ID == id {
			return p
		}
	}
	t.Fatalf("plan %d not on the board", id)

	return report.BoardPlan{}
}

// TestBoardReportsALaneNeverAskedAsNone: no record is "none", with
// the answer key present and empty.
func TestBoardReportsALaneNeverAskedAsNone(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	heldPlan(t, root, "atlas", 7, "Dispatch me")
	runner, _ := recordingHerdr()
	withHerdr(t, runner)
	var out, errb bytes.Buffer

	code := run([]string{"board", "--json", "--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	var raw struct {
		Plans []map[string]any `json:"plans"`
	}
	require.NoError(t, json.Unmarshal(out.Bytes(), &raw))
	require.NotEmpty(t, raw.Plans)
	for _, p := range raw.Plans {
		assert.Equal(t, "none", p["ask_state"])
		assert.Contains(t, p, "answer", "every key present")
	}
}

// TestBoardReportsAPendingAsk: an ask posed and not yet answered reads
// pending on the plan's row.
func TestBoardReportsAPendingAsk(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := heldPlan(t, root, "atlas", 7, "Dispatch me")
	_, err := ask.Pose(repo, 7, "status?", time.Now(), gitwt.Exec)
	require.NoError(t, err)
	runner, _ := recordingHerdr()
	withHerdr(t, runner)
	var doc report.BoardDoc

	emit(t, &doc, "board", "--root", root)

	row := boardRow(t, doc, 7)
	assert.Equal(t, "pending", row.AskState)
	assert.Empty(t, row.Answer)
}

// TestBoardReportsAnAnsweredAskWithItsText: an answered ask carries the
// answer, so an agent reads it off a field.
func TestBoardReportsAnAnsweredAskWithItsText(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := heldPlan(t, root, "atlas", 7, "Dispatch me")
	_, err := ask.Pose(repo, 7, "status?", time.Now(), gitwt.Exec)
	require.NoError(t, err)
	_, err = ask.Answer(repo, 7, "in PR #9", time.Now(), gitwt.Exec)
	require.NoError(t, err)
	runner, _ := recordingHerdr()
	withHerdr(t, runner)
	var doc report.BoardDoc

	emit(t, &doc, "board", "--root", root)

	row := boardRow(t, doc, 7)
	assert.Equal(t, "answered", row.AskState)
	assert.Equal(t, "in PR #9", row.Answer)
}

// TestBoardCarriesAnUnreadableAskAsAProblem: a torn record is carried
// in the document as a problem, never silently read as "none" only.
func TestBoardCarriesAnUnreadableAskAsAProblem(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := heldPlan(t, root, "atlas", 7, "Dispatch me")
	path, err := ask.Path(repo, 7, gitwt.Exec)
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o750))
	require.NoError(t, os.WriteFile(path, []byte("{torn"), 0o600))
	runner, _ := recordingHerdr()
	withHerdr(t, runner)
	var doc report.BoardDoc

	emit(t, &doc, "board", "--root", root)

	assert.Equal(t, "none", boardRow(t, doc, 7).AskState)
	require.NotEmpty(t, doc.Problems)
	assert.Equal(t, "atlas", doc.Problems[0].Repo)
}

// TestBoardAskLeavesAPlanWithNoCheckoutHereAsNone: a repository with no
// coordinate on this host has no record to read; the row keeps "none"
// and nothing is reported as a fault.
func TestBoardAskLeavesAPlanWithNoCheckoutHereAsNone(t *testing.T) {
	rt := &runtime{git: gitwt.Exec}
	doc := report.NewBoard("/fleet", true)
	p := discovery.Plan{Repo: "atlas", ID: 7}
	doc.AddPlan(p, "", "", false)

	boardAsk(rt, fleet.Result{}, doc, p)

	assert.Equal(t, "none", doc.Plans[0].AskState)
	assert.Empty(t, doc.Problems)
}

// TestBoardAskCarriesAnUnplaceableCheckoutAsAProblem: a coordinate git
// cannot place is a problem on the row's repository.
func TestBoardAskCarriesAnUnplaceableCheckoutAsAProblem(t *testing.T) {
	rt := &runtime{git: gitwt.Exec}
	doc := report.NewBoard("/fleet", true)
	p := discovery.Plan{Repo: "atlas", ID: 7}
	doc.AddPlan(p, "", "", false)
	res := fleet.Result{Coords: map[string]fleet.Coord{
		"atlas": {Path: t.TempDir()},
	}}

	boardAsk(rt, res, doc, p)

	assert.Equal(t, "none", doc.Plans[0].AskState)
	require.Len(t, doc.Problems, 1)
	assert.Equal(t, "atlas", doc.Problems[0].Repo)
}

// TestReplySurfacesAGetwdFailure: reply's own os.Getwd error, called
// directly — the full CLI's parser reads the cwd first.
func TestReplySurfacesAGetwdFailure(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	require.NoError(t, os.RemoveAll(dir))
	rt := &runtime{git: gitwt.Exec}

	err := (&replyCmd{Text: "x"}).Run(&cli{}, rt)

	assert.Error(t, err)
}

// whoLaneFor finds the lane on pane in a who document.
func whoLaneFor(t *testing.T, doc report.WhoDoc, pane string) report.WhoLane {
	t.Helper()
	for _, l := range doc.Lanes {
		if l.Pane == pane {
			return l
		}
	}
	t.Fatalf("pane %s not in who", pane)

	return report.WhoLane{}
}

// TestWhoReportsALanesPendingAsk: who reads the ask record from the
// lane's own checkout, so a lane asked and not yet answered reads
// pending.
func TestWhoReportsALanesPendingAsk(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := heldPlan(t, root, "atlas", 7, "Dispatch me")
	_, err := ask.Pose(repo, 7, "status?", time.Now(), gitwt.Exec)
	require.NoError(t, err)
	withHerdr(t, herdrReturning(idleLane(repo)))
	var doc report.WhoDoc

	emit(t, &doc, "who", "--root", root)

	assert.Equal(t, "pending", whoLaneFor(t, doc, "wC:p1").AskState)
}

// TestWhoReportsAnAnsweredAskWithItsText: the answer rides the lane.
func TestWhoReportsAnAnsweredAskWithItsText(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := heldPlan(t, root, "atlas", 7, "Dispatch me")
	_, err := ask.Pose(repo, 7, "status?", time.Now(), gitwt.Exec)
	require.NoError(t, err)
	_, err = ask.Answer(repo, 7, "in PR #9", time.Now(), gitwt.Exec)
	require.NoError(t, err)
	withHerdr(t, herdrReturning(idleLane(repo)))
	var doc report.WhoDoc

	emit(t, &doc, "who", "--root", root)

	lane := whoLaneFor(t, doc, "wC:p1")
	assert.Equal(t, "answered", lane.AskState)
	assert.Equal(t, "in PR #9", lane.Answer)
}

// TestWhoCarriesAnUnreadableAskAsAProblem: a torn record is a problem
// in the document, and the lane keeps "none".
func TestWhoCarriesAnUnreadableAskAsAProblem(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := heldPlan(t, root, "atlas", 7, "Dispatch me")
	path, err := ask.Path(repo, 7, gitwt.Exec)
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o750))
	require.NoError(t, os.WriteFile(path, []byte("{torn"), 0o600))
	withHerdr(t, herdrReturning(idleLane(repo)))
	var doc report.WhoDoc

	emit(t, &doc, "who", "--root", root)

	assert.Equal(t, "none", whoLaneFor(t, doc, "wC:p1").AskState)
	require.NotEmpty(t, doc.Problems)
	assert.Equal(t, "atlas", doc.Problems[0].Repo)
}

// TestWhoAskReadsNoneOffThisHostOrOffAPlan: a lane on another host has
// no record here, and a lane whose branch names no plan was never
// asked — both read none without touching git.
func TestWhoAskReadsNoneOffThisHostOrOffAPlan(t *testing.T) {
	rt := &runtime{git: func(string, ...string) ([]byte, error) {
		t.Fatal("whoAsk reached git")
		return nil, nil
	}}

	for name, lane := range map[string]herdr.Lane{
		"remote":   {Pane: herdr.Pane{Host: "box"}, Root: "/r", PlanID: 7},
		"planless": {Root: "/r"},
	} {
		state, answer, err := whoAsk(rt, lane)
		require.NoError(t, err, name)
		assert.Equal(t, "none", state, name)
		assert.Empty(t, answer, name)
	}
}

// TestWhoAskSurfacesAnUnplaceableCheckout: a lane root git cannot
// place is an error handed back.
func TestWhoAskSurfacesAnUnplaceableCheckout(t *testing.T) {
	rt := &runtime{git: gitwt.Exec}

	_, _, err := whoAsk(rt, herdr.Lane{Root: t.TempDir(), PlanID: 7})

	assert.Error(t, err)
}

// TestPrintWhoShowsTheAskState: the who table prints the same ask
// lines as the board, beneath its rows.
func TestPrintWhoShowsTheAskState(t *testing.T) {
	doc := report.NewWho("/fleet")
	doc.AddLane(herdr.Lane{
		Pane: herdr.Pane{PaneID: "wC:p1", Agent: "claude"}, PlanID: 7,
	})
	doc.SetAsk("wC:p1", "pending", "")
	var buf bytes.Buffer

	printWho(&buf, doc)

	assert.Contains(t, buf.String(),
		"7: asked, no reply yet — silence is not evidence the lane is gone")
}
