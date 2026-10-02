package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/jeduden/frit/internal/ask"
	"github.com/jeduden/frit/internal/fleet"
	"github.com/jeduden/frit/internal/report"
)

type replyCmd struct {
	Text string `arg:"" help:"The answer to the pending ask; put -- before text starting with a dash."`
	Plan int64  `help:"Plan id to answer; default is the plan this lane holds."`
}

// Run records the lane's answer to the pending `frit message --ask`
// for its plan. It is the one verb an agent runs to answer, so it is
// built to need no operator's sign-off: it writes one local file
// beside the repository's git dir and touches no pane, no ref and no
// network — no fleet gather, no fetch, no herdr. That is why it takes
// no --go and why the plan-reply skill can pre-approve it. The plan is
// inferred from the lane's branch, the way open and nudge infer
// theirs; --plan names it in the cwd's own repository instead.
func (r *replyCmd) Run(c *cli, rt *runtime) error {
	if r.Text == "" {
		return errors.New("reply requires an answer")
	}
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	repo, id, err := replyPlan(rt, cwd, r.Plan)
	if err != nil {
		return err
	}

	doc := report.NewReply(repo, id, r.Text)
	rec, err := ask.Answer(cwd, id, r.Text, time.Now(), rt.git)
	switch {
	case errors.Is(err, ask.ErrNoPending):
		doc.Refuse(fmt.Sprintf("no ask is pending for plan %d", id))
	case err != nil:
		return err
	default:
		doc.Record(rec.Question)
	}

	if c.JSON {
		return report.WriteJSON(rt.stdout, doc)
	}
	printReply(rt.stdout, doc)

	return nil
}

// replyPlan names the repository and plan a reply answers: the
// explicit id in the cwd's own repository, or the plan the cwd's lane
// branch holds.
func replyPlan(rt *runtime, cwd string, explicit int64) (string, int64, error) {
	if explicit != 0 {
		return fleet.RepoName(cwd, rt.git), explicit, nil
	}
	repo, id, ok := fleet.CurrentPlanID(cwd, rt.git, holdsForRoot)
	if !ok {
		return "", 0, errors.New(
			"no plan given and none inferred from the current directory; pass --plan")
	}

	return repo, id, nil
}

// printReply reports the answer recorded and the question it settles,
// or why nothing was recorded.
func printReply(out io.Writer, doc *report.ReplyDoc) {
	if doc.Refused != "" {
		_, _ = fmt.Fprintf(out, "refused: %s\n", doc.Refused)
		return
	}
	_, _ = fmt.Fprintf(out, "answered plan %d's ask %q: %q\n",
		doc.ID, doc.Question, doc.Answer)
}
