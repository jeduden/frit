package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/cucumber/godog"
	"github.com/jeduden/frit/internal/skills"
	"github.com/stretchr/testify/require"
)

// C13 is a single-host command behavior — an ask, its answer and the
// board that reports it — so it registers its own steps like every
// command section and reads no lease vocabulary.
func init() {
	registrars = append(registrars, (*world).registerAsk)
}

// askAnswer is the text the lane's reply carries, so the board's row
// can be compared against exactly what was said.
const askAnswer = "in a PR, merging"

// askState holds C13's own state beside the shared world: the lane a
// supervisor asks and its parent directory, the frit invocation and
// the skill's reply command prefix, the fake herdr's log, and the refs
// and log a reply must leave untouched.
type askState struct {
	root, lane string
	// prefix is the installed plan-reply skill's own invocation — the
	// text before `reply` in its example command.
	prefix    string
	herdrLog  string
	out, errb bytes.Buffer
	// logBefore and refsBefore are read just before the reply runs.
	logBefore, refsBefore string
}

func (w *world) registerAsk(sc *godog.ScenarioContext) {
	sc.Step(`^a lane on plan (\d+) with a live agent, and the bundled plan-reply skill `+
		`installed with the built frit invocation$`,
		w.aLaneWithALiveAgentAndThePlanReplySkillInstalled)
	sc.Step(`^a supervisor asks the lane with message --ask --go$`, w.aSupervisorAsksTheLane)
	sc.Step(`^the lane answers with the installed skill's reply command$`,
		w.theLaneAnswersWithTheInstalledSkillsReplyCommand)
	sc.Step(`^the lane's pane was sent an envelope saying a reply is wanted$`,
		w.theLanesPaneWasSentAnEnvelope)
	sc.Step(`^board --json reports plan (\d+)'s ask as answered with the answer text$`,
		w.boardReportsTheAskAnswered)
	sc.Step(`^the reply prompted no pane and moved no ref$`, w.theReplyPromptedNoPaneAndMovedNoRef)
}

// replyCommandPattern finds the plan-reply skill's own example command
// span, so the scenario runs what the skill tells an agent to run
// rather than a hand-built equivalent.
var replyCommandPattern = regexp.MustCompile("`([^`]*) reply \"<answer>\" --json`")

// extractReplyPrefix reads the invocation the installed skill's example
// reply command starts with.
func extractReplyPrefix(skillText string) (string, error) {
	m := replyCommandPattern.FindStringSubmatch(skillText)
	if m == nil {
		return "", fmt.Errorf(
			"plan-reply skill names no `reply \"<answer>\" --json` command")
	}

	return m[1], nil
}

// writeAskHerdr puts a throwaway herdr on $PATH that logs every call
// and answers `agent list` with one working agent standing in the
// lane, so the built frit finds a live lane to ask; every other call
// succeeds silently.
func writeAskHerdr(dir, lane string) error {
	pane, err := json.Marshal(map[string]any{"result": map[string]any{
		"agents": []map[string]any{{
			"agent": "claude", "agent_status": "working", "cwd": lane,
			"pane_id": "wC:p1", "terminal_title_stripped": "busy",
		}}}})
	if err != nil {
		return err
	}
	body := "#!/bin/sh\n" +
		"echo \"$@\" >> \"$HERDR_LOG\"\n" +
		"case \"$1 $2\" in\n" +
		"\"agent list\") echo '" + string(pane) + "' ;;\n" +
		"*) exit 0 ;;\n" +
		"esac\n"

	return os.WriteFile(filepath.Join(dir, "herdr"), []byte(body), 0o755)
}

// aLaneWithALiveAgentAndThePlanReplySkillInstalled is C13's Given: a
// held plan whose lane a fake herdr shows a working agent in, and the
// canonical plan-reply skill installed with the built frit as its
// invocation — read back, as an agent would, for the command it runs.
// The installed approval pattern must name that same invocation, or the
// skill would approve a command it never runs.
func (w *world) aLaneWithALiveAgentAndThePlanReplySkillInstalled(id string) error {
	frit, err := builtFrit()
	if err != nil {
		return err
	}
	isolate(w.t)
	n, _ := strconv.Atoi(id)
	w.planID = n
	as := section[askState](w)
	as.root = w.t.TempDir()
	as.lane = heldPlan(w.t, as.root, "atlas", n, "Dispatch me")

	dir := w.t.TempDir()
	if _, err := skills.Install(dir, false, frit); err != nil {
		return fmt.Errorf("installing the skill bundle: %w", err)
	}
	data, err := os.ReadFile(filepath.Join(
		dir, ".claude", "skills", "plan-reply", "SKILL.md"))
	if err != nil {
		return fmt.Errorf("reading the installed plan-reply skill: %w", err)
	}
	if want := "\nallowed-tools: Bash(" + frit + " reply:*)\n"; !strings.Contains(string(data), want) {
		return fmt.Errorf("the installed skill does not approve %q", want)
	}
	if as.prefix, err = extractReplyPrefix(string(data)); err != nil {
		return err
	}
	if as.prefix != frit {
		return fmt.Errorf("the skill runs %q, not the built frit %q", as.prefix, frit)
	}

	herdrDir := w.t.TempDir()
	as.herdrLog = filepath.Join(herdrDir, "herdr.log")
	if err := os.WriteFile(as.herdrLog, nil, 0o600); err != nil {
		return err
	}
	if err := writeAskHerdr(herdrDir, as.lane); err != nil {
		return err
	}
	w.t.Setenv("PATH", herdrDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	w.t.Setenv("HERDR_LOG", as.herdrLog)

	return nil
}

// runBuiltFrit runs the built frit as a real subprocess from dir, with
// the empty FRIT_ROOT/FRIT_CONFIG isolate() sets stripped — the shape
// an agent's own shell has — and returns its exit error.
func runBuiltFrit(as *askState, dir string, args ...string) error {
	frit, err := builtFrit()
	if err != nil {
		return err
	}
	as.out.Reset()
	as.errb.Reset()
	cmd := exec.Command(frit, args...)
	cmd.Dir = dir
	cmd.Env = envWithout("FRIT_ROOT", "FRIT_CONFIG")
	cmd.Stdout = &as.out
	cmd.Stderr = &as.errb

	return cmd.Run()
}

// aSupervisorAsksTheLane is C13's first When: the built frit's own
// message verb with --ask --go, from the directory holding the repo.
func (w *world) aSupervisorAsksTheLane() error {
	as := section[askState](w)
	if err := runBuiltFrit(as, as.root, "message", strconv.Itoa(w.planID),
		"are you in a PR?", "--ask", "--go", "--root", as.root); err != nil {
		return fmt.Errorf("message --ask --go failed: %w: %s%s", err, as.out.String(), as.errb.String())
	}

	return nil
}

// theLaneAnswersWithTheInstalledSkillsReplyCommand is C13's second
// When: the skill's own example command, run from the lane's checkout —
// where the responder's shell sits — with the herdr log and the refs
// read first, so the Then step can prove the reply touched neither.
func (w *world) theLaneAnswersWithTheInstalledSkillsReplyCommand() error {
	as := section[askState](w)
	logBefore, err := os.ReadFile(as.herdrLog)
	if err != nil {
		return err
	}
	as.logBefore, as.refsBefore = string(logBefore), refs(w.t, as.lane)

	if err := runBuiltFrit(as, as.lane, "reply", askAnswer, "--json"); err != nil {
		return fmt.Errorf("the reply failed: %w: %s%s", err, as.out.String(), as.errb.String())
	}

	return checkReplyOutput(as.out.Bytes(), w.planID)
}

// checkReplyOutput holds the built frit's reply output to what the
// installed skill tells an agent to expect: a document naming the
// command, the plan, the question asked and the answer recorded.
func checkReplyOutput(out []byte, planID int) error {
	var doc struct {
		Command  string `json:"command"`
		Plan     int    `json:"plan"`
		Question string `json:"question"`
		Answer   string `json:"answer"`
	}
	if err := json.Unmarshal(out, &doc); err != nil {
		return fmt.Errorf("reply did not emit valid json: %w: %s", err, out)
	}
	if doc.Command != "reply" || doc.Plan != planID ||
		doc.Question != "are you in a PR?" || doc.Answer != askAnswer {
		return fmt.Errorf("reply's document is not what the skill promises: %s", out)
	}

	return nil
}

// theLanesPaneWasSentAnEnvelope is C13's first Then: the fake herdr's
// own log shows one prompt into the lane's pane carrying the question,
// the line that a reply is wanted, the skill and the raw command.
func (w *world) theLanesPaneWasSentAnEnvelope() error {
	log, err := os.ReadFile(section[askState](w).herdrLog)
	if err != nil {
		return err
	}
	got := string(log)
	for _, want := range []string{
		"agent prompt wC:p1 are you in a PR?",
		"reply is wanted", "plan-reply", `frit reply "<answer>"`,
	} {
		if !strings.Contains(got, want) {
			return fmt.Errorf("the pane's prompt lacks %q: %s", want, got)
		}
	}

	return nil
}

// boardReportsTheAskAnswered is C13's second Then: the built frit's own
// board --json carries plan 7's row as answered, with the answer's text.
func (w *world) boardReportsTheAskAnswered(id string) error {
	as := section[askState](w)
	if err := runBuiltFrit(as, as.root, "board", "--json", "--root", as.root); err != nil {
		return fmt.Errorf("board failed: %w: %s%s", err, as.out.String(), as.errb.String())
	}
	return checkAskAnswered(as.out.Bytes(), id)
}

// checkAskAnswered reads plan id's row off a board --json document and
// fails unless it reads answered with the scenario's answer text.
func checkAskAnswered(boardJSON []byte, id string) error {
	var doc struct {
		Plans []struct {
			ID        int64  `json:"id"`
			AskState  string `json:"ask_state"`
			AskAnswer string `json:"ask_answer"`
		} `json:"plans"`
	}
	if err := json.Unmarshal(boardJSON, &doc); err != nil {
		return fmt.Errorf("board did not emit valid json: %w: %s", err, boardJSON)
	}
	want, _ := strconv.ParseInt(id, 10, 64)
	for _, p := range doc.Plans {
		if p.ID != want {
			continue
		}
		if p.AskState != "answered" || p.AskAnswer != askAnswer {
			return fmt.Errorf("plan %s reads ask %q with answer %q, want answered %q",
				id, p.AskState, p.AskAnswer, askAnswer)
		}

		return nil
	}

	return fmt.Errorf("no board row for plan %s in: %s", id, boardJSON)
}

// theReplyPromptedNoPaneAndMovedNoRef is C13's last Then: the herdr log
// and the lane's refs stand exactly as they did before the reply ran.
func (w *world) theReplyPromptedNoPaneAndMovedNoRef() error {
	as := section[askState](w)
	logAfter, err := os.ReadFile(as.herdrLog)
	if err != nil {
		return err
	}
	if !strings.HasPrefix(string(logAfter), as.logBefore) ||
		strings.Contains(string(logAfter)[len(as.logBefore):], "prompt") {
		return fmt.Errorf("the reply reached a pane: %s", strings.TrimPrefix(string(logAfter), as.logBefore))
	}
	if now := refs(w.t, as.lane); now != as.refsBefore {
		return fmt.Errorf("the reply moved a ref:\nbefore %s\nafter  %s", as.refsBefore, now)
	}

	return nil
}

// TestExtractReplyPrefixReadsTheSkillsExampleCommand: the invocation
// before `reply` is what the scenario runs; a skill with no such
// command fails the step rather than passing on nothing.
func TestExtractReplyPrefixReadsTheSkillsExampleCommand(t *testing.T) {
	got, err := extractReplyPrefix("Run `go run ./cmd/frit reply \"<answer>\" --json` now.")
	require.NoError(t, err)
	require.Equal(t, "go run ./cmd/frit", got)

	_, err = extractReplyPrefix("no command here")
	require.Error(t, err)
}

// TestWriteAskHerdrLogsAndListsTheLane: the fake herdr answers agent
// list with the lane's pane and logs every call it gets.
func TestWriteAskHerdrLogsAndListsTheLane(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, writeAskHerdr(dir, "/lane"))
	log := filepath.Join(dir, "log")
	cmd := exec.Command(filepath.Join(dir, "herdr"), "agent", "list")
	cmd.Env = append(os.Environ(), "HERDR_LOG="+log)

	out, err := cmd.Output()

	require.NoError(t, err)
	require.Contains(t, string(out), `"cwd":"/lane"`)
	logged, err := os.ReadFile(log)
	require.NoError(t, err)
	require.Contains(t, string(logged), "agent list")
}

// TestCheckAskAnsweredReadsTheRow: a row that reads pending, carries
// another answer, is missing or is not JSON fails; the answered row
// with the scenario's answer passes.
func TestCheckAskAnsweredReadsTheRow(t *testing.T) {
	row := func(state, answer string) []byte {
		return []byte(fmt.Sprintf(
			`{"plans":[{"id":7,"ask_state":%q,"ask_answer":%q}]}`, state, answer))
	}

	require.Error(t, checkAskAnswered(row("pending", ""), "7"))
	require.Error(t, checkAskAnswered(row("answered", "something else"), "7"))
	require.Error(t, checkAskAnswered(row("answered", askAnswer), "8"), "no row for the plan")
	require.Error(t, checkAskAnswered([]byte("not json"), "7"))
	require.NoError(t, checkAskAnswered(row("answered", askAnswer), "7"))
}

// TestCheckReplyOutputHoldsTheSkillsPromise: the document must name the
// reply command, the plan, the question and the answer, as plan-reply
// says it will; anything else fails the step.
func TestCheckReplyOutputHoldsTheSkillsPromise(t *testing.T) {
	good := `{"command":"reply","plan":7,"question":"are you in a PR?","answer":"` + askAnswer + `"}`

	require.NoError(t, checkReplyOutput([]byte(good), 7))
	require.Error(t, checkReplyOutput([]byte(good), 8), "another plan")
	require.Error(t, checkReplyOutput([]byte(`{"command":"reply","plan":7}`), 7), "no answer")
	require.Error(t, checkReplyOutput([]byte("not json"), 7))
}
