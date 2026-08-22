package skills

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// TestInstallWritesSkills checks that Install lays each bundled skill
// into .claude/skills/<name>/SKILL.md under the target repo, reports
// every path it wrote, and writes plan-pick and plan-tidy among them.
func TestInstallWritesSkills(t *testing.T) {
	dir := t.TempDir()

	paths, err := Install(dir, false)
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if len(paths) == 0 {
		t.Fatal("Install wrote no skills")
	}

	wants := []string{
		filepath.Join(dir, ".claude", "skills", "plan-pick", "SKILL.md"),
		filepath.Join(dir, ".claude", "skills", "plan-tidy", "SKILL.md"),
	}
	found := make(map[string]bool, len(wants))
	for _, p := range paths {
		for _, w := range wants {
			if p == w {
				found[w] = true
			}
		}
		data, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("reading written skill %s: %v", p, err)
		}
		if len(data) == 0 {
			t.Fatalf("written skill %s is empty", p)
		}
	}
	for _, w := range wants {
		if !found[w] {
			t.Fatalf("paths %v do not include %s", paths, w)
		}
	}
}

// TestInstallRefusesToClobber checks that a second run without force
// reports ErrExists and does not overwrite the file.
func TestInstallRefusesToClobber(t *testing.T) {
	dir := t.TempDir()

	if _, err := Install(dir, false); err != nil {
		t.Fatalf("first Install: %v", err)
	}

	target := filepath.Join(dir, ".claude", "skills", "plan-pick", "SKILL.md")
	if err := os.WriteFile(target, []byte("edited"), 0o600); err != nil {
		t.Fatalf("editing skill: %v", err)
	}

	_, err := Install(dir, false)
	if !errors.Is(err, ErrExists) {
		t.Fatalf("second Install error = %v, want ErrExists", err)
	}

	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("reading skill: %v", err)
	}
	if string(data) != "edited" {
		t.Fatal("Install clobbered an existing skill without force")
	}
}

// TestInstallForceOverwrites checks that force replaces an existing
// skill with the bundled content.
func TestInstallForceOverwrites(t *testing.T) {
	dir := t.TempDir()

	if _, err := Install(dir, false); err != nil {
		t.Fatalf("first Install: %v", err)
	}

	target := filepath.Join(dir, ".claude", "skills", "plan-pick", "SKILL.md")
	if err := os.WriteFile(target, []byte("edited"), 0o600); err != nil {
		t.Fatalf("editing skill: %v", err)
	}

	if _, err := Install(dir, true); err != nil {
		t.Fatalf("forced Install: %v", err)
	}

	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("reading skill: %v", err)
	}
	if string(data) == "edited" {
		t.Fatal("force did not overwrite the edited skill")
	}
}

// TestPlanNewFrontsDoctor guards the standing rule CLAUDE.md records:
// a new agent-facing verb ships with the thin skill that fronts it.
// doctor checks exactly what plan-new already shapes a plan to
// satisfy — a Goal, a tier, an Execution row — so its mention lives
// there rather than in a fifth skill.
func TestPlanNewFrontsDoctor(t *testing.T) {
	data, err := assets.ReadFile("assets/plan-new/SKILL.md")
	if err != nil {
		t.Fatalf("reading plan-new skill: %v", err)
	}
	if !contains(string(data), "frit doctor") {
		t.Fatal("plan-new skill does not mention `frit doctor`")
	}
}

// TestShippedSkillNamesFritOnPath guards the load-bearing content
// detail: a shipped skill runs where frit is a binary on PATH, so its
// commands must read `frit`, not `go run ./cmd/frit`.
func TestShippedSkillNamesFritOnPath(t *testing.T) {
	dir := t.TempDir()
	if _, err := Install(dir, false); err != nil {
		t.Fatalf("Install: %v", err)
	}

	target := filepath.Join(dir, ".claude", "skills", "plan-pick", "SKILL.md")
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("reading skill: %v", err)
	}

	body := string(data)
	if !contains(body, "frit pick") {
		t.Fatal("shipped skill does not name the `frit pick` verb")
	}
	if contains(body, "go run ./cmd/frit pick") {
		t.Fatal("shipped skill uses the source-checkout command form for a verb")
	}
}

// TestDogfoodCopiesMatchCanonical pins frit's own .claude/skills, the
// dogfooded output of Install, byte-equal to the embedded assets it
// came from. CLAUDE.md claims the two trees never drift; this is what
// enforces that rather than leaving it to hand-checking.
func TestDogfoodCopiesMatchCanonical(t *testing.T) {
	files, err := bundledFiles()
	if err != nil {
		t.Fatalf("bundledFiles: %v", err)
	}
	if len(files) == 0 {
		t.Fatal("bundledFiles returned no files")
	}

	for _, rel := range files {
		canonical, err := assets.ReadFile(path("assets", rel))
		if err != nil {
			t.Fatalf("reading canonical asset %s: %v", rel, err)
		}

		dogfood := filepath.Join("..", "..", ".claude", "skills", rel)
		got, err := os.ReadFile(dogfood)
		if err != nil {
			t.Fatalf("reading dogfood copy %s: %v", dogfood, err)
		}

		if string(got) != string(canonical) {
			t.Fatalf("%s has drifted from %s", dogfood, rel)
		}
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
