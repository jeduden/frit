package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/cucumber/godog"
	"github.com/jeduden/frit/internal/report"
)

// C13 runs an ask and its reply end to end through the built frit: a
// supervisor in the main checkout asks with message --ask --go, the
// lane's agent answers with reply from its own worktree, and the board
// reads the answer back. It registers as its own section file, per
// docs/development.md, and keeps its own state.
func init() {
	registrars = append(registrars, (*world).registerAsk)
}

// askState is C13's own: the fleet root and main checkout the
// supervisor stands in, the lane worktree the responder stands in, the
// fake herdr's call log, the refs before the reply, and the $PATH that
// puts the fake herdr first.
type askState struct {
	root     string
	repo     string
	lane     string
	herdrLog string
	path     string
	refs     string
	logLen   int
}

func (w *world) registerAsk(sc *godog.ScenarioContext) {
	sc.Step(`^plan (\d+) is held in a lane worktree whose agent is live on this host$`,
		w.planIsHeldInALaneWorktreeWhoseAgentIsLive)
	sc.Step(`^the supervisor's built frit asks plan (\d+) "([^"]+)" with --ask --go$`,
		w.theSupervisorsBuiltFritAsks)
	sc.Step(`^the lane's pane receives the ask, saying a reply is wanted and how to give it$`,
		w.theLanesPaneReceivesTheAsk)
	sc.Step(`^the lane's built frit replies "([^"]+)"$`, w.theLanesBuiltFritReplies)
	sc.Step(`^the reply reached no pane and moved no ref$`, w.theReplyReachedNoPaneAndMovedNoRef)
	sc.Step(`^board --json reports plan (\d+)'s ask answered with "([^"]+)"$`,
		w.boardReportsThePlansAskAnswered)
}

// planIsHeldInALaneWorktreeWhoseAgentIsLive builds the shape an ask
// meets: a repository whose main checkout sits on main, its plan's
// hold branch checked out in a linked worktree beside it, and a fake
// herdr on $PATH whose one working agent stands in that worktree.
func (w *world) planIsHeldInALaneWorktreeWhoseAgentIsLive(id string) error {
	isolate(w.t)
	n, err := strconv.Atoi(id)
	if err != nil {
		return err
	}
	st := section[askState](w)
	st.root = w.t.TempDir()
	st.repo = heldPlan(w.t, st.root, "atlas", n, "Dispatch me")
	git(w.t, st.repo, "checkout", "-q", "main")
	st.lane = filepath.Join(w.t.TempDir(), "atlas-"+id)
	git(w.t, st.repo, "worktree", "add", "-q", st.lane, planBranch(n, "Dispatch me"))

	herdrDir := w.t.TempDir()
	st.herdrLog = filepath.Join(herdrDir, "herdr.log")
	if err := os.WriteFile(st.herdrLog, nil, 0o600); err != nil {
		return err
	}
	if err := writeAskHerdr(herdrDir, st.lane); err != nil {
		return err
	}
	st.path = herdrDir + string(os.PathListSeparator) + os.Getenv("PATH")

	return nil
}

// writeAskHerdr stands a herdr on $PATH that answers agent list with
// one working agent in lane and logs every call, one per line, for the
// Then steps to read. Every other call succeeds silently.
func writeAskHerdr(dir, lane string) error {
	agents, err := json.Marshal(map[string]any{
		"result": map[string]any{"agents": []map[string]any{{
			"agent": "claude", "agent_status": "working", "cwd": lane,
			"pane_id": "wA:p1", "terminal_title_stripped": "lane",
		}}},
	})
	if err != nil {
		return err
	}
	body := "#!/bin/sh\n" +
		"echo \"$@\" >> \"$HERDR_LOG\"\n" +
		"case \"$1 $2\" in\n" +
		"\"agent list\")\n" +
		"  cat <<'JSON'\n" + string(agents) + "\nJSON\n" +
		"  ;;\n" +
		"esac\n" +
		"exit 0\n"

	return os.WriteFile(filepath.Join(dir, "herdr"), []byte(body), 0o755)
}

// runBuiltFrit runs the built binary in dir under C13's $PATH, so what
// is proven is the shipped command's behavior.
func (w *world) runBuiltFrit(dir string, args ...string) (string, error) {
	frit, err := builtFrit()
	if err != nil {
		return "", err
	}
	st := section[askState](w)
	cmd := exec.Command(frit, args...)
	cmd.Dir = dir
	cmd.Env = append(envWithout("FRIT_ROOT", "FRIT_CONFIG", "PATH", "HERDR_LOG"),
		"PATH="+st.path, "HERDR_LOG="+st.herdrLog)
	out, err := cmd.Output()
	if err != nil {
		var stderr string
		if ee, ok := err.(*exec.ExitError); ok {
			stderr = string(ee.Stderr)
		}
		return "", fmt.Errorf("frit %s: %w: %s", strings.Join(args, " "), err, stderr)
	}

	return string(out), nil
}

// theSupervisorsBuiltFritAsks runs message --ask --go from the main
// checkout, the supervisor's seat.
func (w *world) theSupervisorsBuiltFritAsks(id, text string) error {
	st := section[askState](w)
	_, err := w.runBuiltFrit(st.repo,
		"message", id, "--ask", text, "--go", "--root", st.root)

	return err
}

// theLanesPaneReceivesTheAsk reads the fake herdr's log: one prompt to
// the lane's pane, carrying the text, the request for a reply, the
// skill and the raw reply command.
func (w *world) theLanesPaneReceivesTheAsk() error {
	st := section[askState](w)
	log, err := os.ReadFile(st.herdrLog)
	if err != nil {
		return err
	}
	var prompts []string
	for _, line := range strings.Split(string(log), "\n") {
		if strings.HasPrefix(line, "agent prompt wA:p1 ") {
			prompts = append(prompts, line)
		}
	}
	if len(prompts) != 1 {
		return fmt.Errorf("expected one prompt to the lane, got %d in: %s", len(prompts), log)
	}
	for _, want := range []string{
		"are you in a PR?", "reply is wanted", "plan-reply", `frit reply "<answer>"`,
	} {
		if !strings.Contains(prompts[0], want) {
			return fmt.Errorf("the ask lacks %q: %s", want, prompts[0])
		}
	}

	return nil
}

// theLanesBuiltFritReplies runs reply from the lane's own worktree with
// no plan argument and no --go, after noting the refs and the herdr
// log as they stood.
func (w *world) theLanesBuiltFritReplies(answer string) error {
	st := section[askState](w)
	refs, err := gitCapture(w.t, st.repo, "for-each-ref", "--format=%(refname) %(objectname)")
	if err != nil {
		return err
	}
	st.refs = refs
	log, err := os.ReadFile(st.herdrLog)
	if err != nil {
		return err
	}
	st.logLen = len(log)
	out, err := w.runBuiltFrit(st.lane, "reply", answer)
	if err != nil {
		return err
	}
	if !strings.Contains(out, answer) {
		return fmt.Errorf("reply did not report the answer: %s", out)
	}

	return nil
}

// theReplyReachedNoPaneAndMovedNoRef: the herdr log gained nothing and
// every ref reads as it did before the reply.
func (w *world) theReplyReachedNoPaneAndMovedNoRef() error {
	st := section[askState](w)
	log, err := os.ReadFile(st.herdrLog)
	if err != nil {
		return err
	}
	if len(log) != st.logLen {
		return fmt.Errorf("reply reached herdr: %s", log[st.logLen:])
	}
	refs, err := gitCapture(w.t, st.repo, "for-each-ref", "--format=%(refname) %(objectname)")
	if err != nil {
		return err
	}
	if refs != st.refs {
		return fmt.Errorf("reply moved a ref:\nbefore %s\nafter %s", st.refs, refs)
	}

	return nil
}

// boardReportsThePlansAskAnswered reads board --json from the main
// checkout, the field an agent branches on.
func (w *world) boardReportsThePlansAskAnswered(id, answer string) error {
	st := section[askState](w)
	out, err := w.runBuiltFrit(st.repo, "board", "--json", "--root", st.root)
	if err != nil {
		return err
	}
	var doc report.BoardDoc
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		return fmt.Errorf("board --json: %w: %s", err, out)
	}
	want, _ := strconv.ParseInt(id, 10, 64)
	for _, p := range doc.Plans {
		if p.ID != want {
			continue
		}
		if p.AskState != "answered" || p.Answer != answer {
			return fmt.Errorf("plan %s's ask reads %q / %q", id, p.AskState, p.Answer)
		}

		return nil
	}

	return fmt.Errorf("plan %s not on the board: %s", id, out)
}
