package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os/exec"
	"testing"

	"github.com/jeduden/frit/internal/ask"
	"github.com/jeduden/frit/internal/dispatch"
	"github.com/jeduden/frit/internal/gitwt"
	"github.com/jeduden/frit/internal/report"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// promptText is the text the fake herdr received on an `agent prompt`,
// "" when none was sent — what a test reads to prove the envelope
// reached the pane whole.
func promptText(rec *herdrCalls) string {
	rec.mu.Lock()
	defer rec.mu.Unlock()
	for _, c := range rec.calls {
		if len(c) >= 4 && c[0] == "agent" && c[1] == "prompt" {
			return c[3]
		}
	}

	return ""
}

// refs lists every ref of a repository, so a test can prove a verb
// touched none.
func refs(t *testing.T, repo string) string {
	t.Helper()
	out, err := exec.Command("git", "-C", repo, "for-each-ref",
		"--format=%(refname) %(objectname)").CombinedOutput()
	require.NoError(t, err, string(out))

	return string(out)
}

func TestMessageAskDryRunShowsTheEnvelopeAndWritesNothing(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := heldPlan(t, root, "atlas", 7, "Dispatch me")
	runner, rec := recordingHerdr(workingLane(repo))
	withHerdr(t, runner)
	var out, errb bytes.Buffer

	code := run([]string{"message", "7", "are you in a PR?", "--ask",
		"--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	got := out.String()
	assert.Contains(t, got, "are you in a PR?")
	assert.Contains(t, got, "reply is wanted", "the envelope says a reply is wanted")
	assert.Contains(t, got, "plan-reply", "it names the skill to load")
	assert.Contains(t, got, `frit reply "<answer>"`, "it gives the raw command")
	assert.Contains(t, got, "run again with --go to send")
	assert.False(t, rec.verb("agent", "prompt"), "a dry run sends nothing")
	_, state := ask.Read(repo, 7, gitwt.Exec)
	assert.Equal(t, ask.None, state, "a dry run writes no record")
}

func TestMessageAskGoSendsTheEnvelopeWholeAndRecordsTheAsk(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := heldPlan(t, root, "atlas", 7, "Dispatch me")
	runner, rec := recordingHerdr(workingLane(repo))
	withHerdr(t, runner)
	var out, errb bytes.Buffer

	code := run([]string{"message", "7", "are you in a PR?", "--ask", "--go",
		"--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	assert.Equal(t, dispatch.AskEnvelope("are you in a PR?"), promptText(rec),
		"the pane receives the envelope whole")
	got, state := ask.Read(repo, 7, gitwt.Exec)
	assert.Equal(t, ask.Pending, state)
	assert.Equal(t, "are you in a PR?", got.Text,
		"the record keeps the operator's words, not the envelope")
}

func TestMessageWithoutAskSendsTheTextAndRecordsNothing(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := heldPlan(t, root, "atlas", 7, "Dispatch me")
	runner, rec := recordingHerdr(workingLane(repo))
	withHerdr(t, runner)
	var out, errb bytes.Buffer

	code := run([]string{"message", "7", "are you in a PR?", "--go",
		"--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	assert.Equal(t, "are you in a PR?", promptText(rec), "no envelope without --ask")
	_, state := ask.Read(repo, 7, gitwt.Exec)
	assert.Equal(t, ask.None, state)
}

func TestMessageAskRefusedWritesNoRecord(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := heldPlan(t, root, "atlas", 7, "Dispatch me")
	runner, _ := recordingHerdr() // no live lane
	withHerdr(t, runner)
	var out, errb bytes.Buffer

	code := run([]string{"message", "7", "status?", "--ask", "--go",
		"--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	assert.Contains(t, out.String(), "refused")
	_, state := ask.Read(repo, 7, gitwt.Exec)
	assert.Equal(t, ask.None, state, "an ask nobody received leaves no record")
}

func TestMessageAskDropsTheRecordWhenTheSendFails(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := heldPlan(t, root, "atlas", 7, "Dispatch me")
	inner, _ := recordingHerdr(workingLane(repo))
	withHerdr(t, func(args ...string) ([]byte, error) {
		if len(args) > 1 && args[0] == "agent" && args[1] == "prompt" {
			return nil, errors.New("herdr: pane gone")
		}

		return inner(args...)
	})
	var out, errb bytes.Buffer

	code := run([]string{"message", "7", "status?", "--ask", "--go",
		"--root", root}, &out, &errb)

	assert.NotEqual(t, 0, code)
	_, state := ask.Read(repo, 7, gitwt.Exec)
	assert.Equal(t, ask.None, state, "an ask that never went is not pending")
}

func TestMessageAskEmitsTheEnvelopeInJSON(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := heldPlan(t, root, "atlas", 7, "Dispatch me")
	runner, _ := recordingHerdr(workingLane(repo))
	withHerdr(t, runner)
	var doc report.MessageDoc

	emit(t, &doc, "message", "7", "status?", "--ask", "--root", root)

	assert.True(t, doc.Ask)
	assert.Equal(t, "status?", doc.Text)
	assert.Equal(t, dispatch.AskEnvelope("status?"), doc.Envelope)
}

func TestMessageWithoutAskCarriesNoEnvelope(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := heldPlan(t, root, "atlas", 7, "Dispatch me")
	runner, _ := recordingHerdr(workingLane(repo))
	withHerdr(t, runner)
	var doc report.MessageDoc

	emit(t, &doc, "message", "7", "status?", "--root", root)

	assert.False(t, doc.Ask)
	assert.Empty(t, doc.Envelope)
}

// askedLane holds plan 7's lane with one pending ask, standing the
// process in the lane's checkout — where a responder runs `reply`.
func askedLane(t *testing.T) (repo string, rec *herdrCalls) {
	t.Helper()
	isolate(t)
	root := t.TempDir()
	repo = heldPlan(t, root, "atlas", 7, "Dispatch me")
	var runner func(...string) ([]byte, error)
	runner, rec = recordingHerdr(workingLane(repo))
	withHerdr(t, runner)
	require.NoError(t, ask.Open(repo, 7, "are you in a PR?", gitwt.Exec))
	t.Chdir(repo)

	return repo, rec
}

func TestReplyStoresTheAnswerAgainstThePendingAsk(t *testing.T) {
	repo, rec := askedLane(t)
	before := refs(t, repo)
	var out, errb bytes.Buffer

	code := run([]string{"reply", "in a PR, merging"}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	got, state := ask.Read(repo, 7, gitwt.Exec)
	assert.Equal(t, ask.Answered, state)
	assert.Equal(t, "in a PR, merging", got.Answer)
	assert.Empty(t, rec.calls, "reply prompts no pane and asks herdr nothing")
	assert.Equal(t, before, refs(t, repo), "reply touches no ref")
	assert.Contains(t, out.String(), "7")
}

func TestReplyTakesNoGoFlag(t *testing.T) {
	askedLane(t)
	var out, errb bytes.Buffer

	code := run([]string{"reply", "yes", "--go"}, &out, &errb)

	assert.NotEqual(t, 0, code, "reply is a local write, never a send to confirm")
}

func TestReplyRefusesWithNoAskPendingAndSaysWhy(t *testing.T) {
	repo, _ := askedLane(t)
	require.NoError(t, ask.Reply(repo, 7, "already", gitwt.Exec))
	var out, errb bytes.Buffer

	code := run([]string{"reply", "again"}, &out, &errb)

	assert.NotEqual(t, 0, code)
	assert.Contains(t, errb.String(), "no ask is pending")
	assert.Contains(t, errb.String(), "7", "it names the plan")
	got, _ := ask.Read(repo, 7, gitwt.Exec)
	assert.Equal(t, "already", got.Answer)
}

func TestReplyRefusesOutsideALane(t *testing.T) {
	isolate(t)
	var out, errb bytes.Buffer

	code := run([]string{"reply", "yes"}, &out, &errb)

	assert.NotEqual(t, 0, code)
	assert.Contains(t, errb.String(), "none inferred from the current directory")
}

func TestReplyRefusesAnEmptyAnswer(t *testing.T) {
	repo, _ := askedLane(t)
	var out, errb bytes.Buffer

	code := run([]string{"reply", ""}, &out, &errb)

	assert.NotEqual(t, 0, code)
	assert.Contains(t, errb.String(), "requires an answer")
	_, state := ask.Read(repo, 7, gitwt.Exec)
	assert.Equal(t, ask.Pending, state)
}

func TestReplyNamesThePlanExplicitly(t *testing.T) {
	repo, _ := askedLane(t)
	sub := repo + "/plan"
	t.Chdir(sub)
	var out, errb bytes.Buffer

	code := run([]string{"reply", "yes", "7"}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	_, state := ask.Read(repo, 7, gitwt.Exec)
	assert.Equal(t, ask.Answered, state)
}

func TestReplyRefusesASelectorThatIsNotAPlanID(t *testing.T) {
	askedLane(t)
	var out, errb bytes.Buffer

	code := run([]string{"reply", "yes", "dispatch-me"}, &out, &errb)

	assert.NotEqual(t, 0, code)
	assert.Contains(t, errb.String(), "plan id")
}

func TestReplyEmitsJSON(t *testing.T) {
	askedLane(t)
	var doc report.ReplyDoc

	emit(t, &doc, "reply", "in a PR")

	assert.Equal(t, "reply", doc.Command)
	assert.Equal(t, int64(7), doc.Plan)
	assert.Equal(t, "are you in a PR?", doc.Question)
	assert.Equal(t, "in a PR", doc.Answer)
}

// boardAsk runs `board --json` and returns plan 7's row, decoded raw so
// a missing key is caught rather than read as its zero value.
func boardAsk(t *testing.T, root string) map[string]any {
	t.Helper()
	var out, errb bytes.Buffer
	code := run([]string{"board", "--json", "--root", root}, &out, &errb)
	require.Equal(t, 0, code, errb.String())
	var doc struct {
		Plans []map[string]any `json:"plans"`
	}
	require.NoError(t, json.Unmarshal(out.Bytes(), &doc), out.String())
	require.Len(t, doc.Plans, 1)

	return doc.Plans[0]
}

func TestBoardReportsAnAskAsNoneWhenNothingWasAsked(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := heldPlan(t, root, "atlas", 7, "Dispatch me")
	runner, _ := recordingHerdr(workingLane(repo))
	withHerdr(t, runner)

	row := boardAsk(t, root)

	assert.Equal(t, "none", row["ask_state"])
	assert.Equal(t, "", row["ask_answer"], "every key is present")
}

func TestBoardReportsAPendingAsk(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := heldPlan(t, root, "atlas", 7, "Dispatch me")
	runner, _ := recordingHerdr(workingLane(repo))
	withHerdr(t, runner)
	require.NoError(t, ask.Open(repo, 7, "status?", gitwt.Exec))

	row := boardAsk(t, root)

	assert.Equal(t, "pending", row["ask_state"])
	assert.Equal(t, "", row["ask_answer"])
}

func TestBoardReportsAnAnsweredAskWithItsAnswer(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := heldPlan(t, root, "atlas", 7, "Dispatch me")
	runner, _ := recordingHerdr(workingLane(repo))
	withHerdr(t, runner)
	require.NoError(t, ask.Open(repo, 7, "status?", gitwt.Exec))
	require.NoError(t, ask.Reply(repo, 7, "in a PR", gitwt.Exec))

	row := boardAsk(t, root)

	assert.Equal(t, "answered", row["ask_state"])
	assert.Equal(t, "in a PR", row["ask_answer"])
}

func TestBoardReportsNoneForALaneWithNoLiveAgent(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := heldPlan(t, root, "atlas", 7, "Dispatch me")
	runner, _ := recordingHerdr() // no pane
	withHerdr(t, runner)
	require.NoError(t, ask.Open(repo, 7, "status?", gitwt.Exec))

	row := boardAsk(t, root)

	assert.Equal(t, "none", row["ask_state"],
		"an ask is read from a lane frit can see live")
}
