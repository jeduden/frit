package skills

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// settingsAllow reads a repository's .claude/settings.json and returns
// its permissions.allow list, failing the test when the file or the
// list is missing.
func settingsAllow(t *testing.T, dir string) []string {
	t.Helper()

	data, err := os.ReadFile(filepath.Join(dir, ".claude", "settings.json"))
	if err != nil {
		t.Fatalf("reading settings: %v", err)
	}
	var doc struct {
		Permissions struct {
			Allow []string `json:"allow"`
		} `json:"permissions"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("settings is not JSON: %v", err)
	}

	return doc.Permissions.Allow
}

func hasRule(rules []string, want string) bool {
	for _, r := range rules {
		if r == want {
			return true
		}
	}

	return false
}

// TestInstallWritesReplyAllowRule: a repository with no settings file
// gets one whose allow list approves the plan-reply skill and the
// `reply` command under the chosen invocation, and Install reports the
// file among those it wrote.
func TestInstallWritesReplyAllowRule(t *testing.T) {
	dir := t.TempDir()

	paths, err := Install(dir, false, "go run ./cmd/frit")
	if err != nil {
		t.Fatalf("Install: %v", err)
	}

	rules := settingsAllow(t, dir)
	for _, want := range []string{"Skill(plan-reply)", "Bash(go run ./cmd/frit reply:*)"} {
		if !hasRule(rules, want) {
			t.Fatalf("allow list %v lacks %q", rules, want)
		}
	}

	settings := filepath.Join(dir, ".claude", "settings.json")
	if !hasRule(paths, settings) {
		t.Fatalf("paths %v do not include %s", paths, settings)
	}
}

// TestInstallMergesIntoExistingSettings: other keys and other allow
// rules survive, only the two missing rules are added, and a re-run
// changes nothing and does not report the file again.
func TestInstallMergesIntoExistingSettings(t *testing.T) {
	dir := t.TempDir()
	settings := filepath.Join(dir, ".claude", "settings.json")
	if err := os.MkdirAll(filepath.Dir(settings), 0o755); err != nil {
		t.Fatal(err)
	}
	seed := `{"enabledPlugins": {"gopls-lsp@x": true}, "permissions": {"allow": ["Bash(ls:*)"], "deny": ["Bash(rm:*)"]}}`
	if err := os.WriteFile(settings, []byte(seed), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := Install(dir, false, ""); err != nil {
		t.Fatalf("Install: %v", err)
	}

	data, err := os.ReadFile(settings)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("settings is not JSON: %v", err)
	}
	if _, ok := doc["enabledPlugins"]; !ok {
		t.Fatal("merge dropped enabledPlugins")
	}
	var perms struct {
		Allow []string `json:"allow"`
		Deny  []string `json:"deny"`
	}
	if err := json.Unmarshal(doc["permissions"], &perms); err != nil {
		t.Fatal(err)
	}
	if !hasRule(perms.Deny, "Bash(rm:*)") {
		t.Fatalf("merge dropped the deny rule: %v", perms.Deny)
	}
	for _, want := range []string{"Bash(ls:*)", "Skill(plan-reply)", "Bash(frit reply:*)"} {
		if !hasRule(perms.Allow, want) {
			t.Fatalf("allow list %v lacks %q", perms.Allow, want)
		}
	}

	// Skills refuse a second run without force, so re-run with force
	// and check the settings file is left byte for byte alone.
	paths, err := Install(dir, true, "")
	if err != nil {
		t.Fatalf("second Install: %v", err)
	}
	again, err := os.ReadFile(settings)
	if err != nil {
		t.Fatal(err)
	}
	if string(again) != string(data) {
		t.Fatal("a re-run rewrote settings that already carried the rules")
	}
	if hasRule(paths, settings) {
		t.Fatalf("a re-run reported the unchanged settings %s", settings)
	}
}

// TestInstallRefusesInvalidSettings: a settings file that is not JSON
// is refused with its path named, and no skill is written.
func TestInstallRefusesInvalidSettings(t *testing.T) {
	dir := t.TempDir()
	settings := filepath.Join(dir, ".claude", "settings.json")
	if err := os.MkdirAll(filepath.Dir(settings), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(settings, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := Install(dir, false, "")
	if err == nil {
		t.Fatal("Install accepted an invalid settings file")
	}
	if !errors.Is(err, ErrSettings) {
		t.Fatalf("error = %v, want ErrSettings", err)
	}
	if !contains(err.Error(), settings) {
		t.Fatalf("error %q does not name %s", err, settings)
	}
	if _, statErr := os.Stat(filepath.Join(dir, ".claude", "skills")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatal("a refused settings merge still wrote skills")
	}
}

// TestReplyAllowRuleIsThePlanReplyApproval: the Bash rule written to
// settings is the value plan-reply ships in allowed-tools, so the two
// never drift.
func TestReplyAllowRuleIsThePlanReplyApproval(t *testing.T) {
	data, err := assets.ReadFile("assets/plan-reply/SKILL.md")
	if err != nil {
		t.Fatalf("reading plan-reply skill: %v", err)
	}
	for _, rule := range replyAllow {
		if rule == "Skill(plan-reply)" {
			continue
		}
		if !contains(string(data), "\nallowed-tools: "+rule+"\n") {
			t.Fatalf("plan-reply's allowed-tools is not the settings rule %q", rule)
		}
	}
}

// TestDogfoodSettingsAllowTheReply: frit's own settings carry both
// rules for the invocation its dogfooded skills use.
func TestDogfoodSettingsAllowTheReply(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", ".claude", "settings.json"))
	if err != nil {
		t.Fatalf("reading dogfood settings: %v", err)
	}
	var doc struct {
		Permissions struct {
			Allow []string `json:"allow"`
		} `json:"permissions"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Skill(plan-reply)", "Bash(go run ./cmd/frit reply:*)"} {
		if !hasRule(doc.Permissions.Allow, want) {
			t.Fatalf("dogfood allow list %v lacks %q", doc.Permissions.Allow, want)
		}
	}
}
