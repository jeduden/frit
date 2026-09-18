package skills

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
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
	return slices.Contains(rules, want)
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
	seed := `{"enabledPlugins": {"gopls-lsp@x": true},
  "permissions": {"allow": ["Bash(ls:*)"], "deny": ["Bash(rm:*)"]}}`
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

// TestLoadSettingsReadsKeysRaw: a missing file is an empty document, a
// present one keeps each top-level key as raw JSON, and a file that is
// not a JSON object or cannot be read is refused.
func TestLoadSettingsReadsKeysRaw(t *testing.T) {
	dir := t.TempDir()

	_, doc, err := loadSettings(filepath.Join(dir, "missing.json"))
	if err != nil || len(doc) != 0 {
		t.Fatalf("missing file: doc %v, err %v; want empty, nil", doc, err)
	}

	present := filepath.Join(dir, "present.json")
	if err := os.WriteFile(present, []byte(`{"a": {"b": 1}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	_, doc, err = loadSettings(present)
	if err != nil || string(doc["a"]) != `{"b": 1}` {
		t.Fatalf("present file: doc %v, err %v", doc, err)
	}

	array := filepath.Join(dir, "array.json")
	if err := os.WriteFile(array, []byte(`[1]`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := loadSettings(array); !errors.Is(err, ErrSettings) {
		t.Fatalf("a JSON array: err %v, want ErrSettings", err)
	}

	if _, _, err := loadSettings(dir); err == nil || errors.Is(err, ErrSettings) {
		t.Fatalf("a directory: err %v, want a read error", err)
	}
}

// TestAddReplyRulesRefusesShapesItCannotMerge: permissions that is not
// an object, or an allow that is not a list, is ErrSettings naming the
// file, and the document is left as it was.
func TestAddReplyRulesRefusesShapesItCannotMerge(t *testing.T) {
	for name, raw := range map[string]string{
		"permissions not an object": `{"permissions": []}`,
		"allow not a list":          `{"permissions": {"allow": "Bash(ls:*)"}}`,
	} {
		var doc map[string]json.RawMessage
		if err := json.Unmarshal([]byte(raw), &doc); err != nil {
			t.Fatal(err)
		}

		changed, err := addReplyRules("settings.json", doc, "frit")

		if changed || !errors.Is(err, ErrSettings) || !contains(err.Error(), "settings.json") {
			t.Fatalf("%s: changed %v, err %v; want unchanged ErrSettings naming the file", name, changed, err)
		}
	}
}

// TestAddReplyRulesAddsOnlyWhatIsMissing: a document with one rule
// present gains the other; one with both reports no change.
func TestAddReplyRulesAddsOnlyWhatIsMissing(t *testing.T) {
	doc := map[string]json.RawMessage{
		"permissions": json.RawMessage(`{"allow": ["Skill(plan-reply)"]}`),
	}

	changed, err := addReplyRules("s.json", doc, "frit")
	if err != nil || !changed {
		t.Fatalf("first: changed %v, err %v; want changed", changed, err)
	}
	if want := `{"allow":["Skill(plan-reply)","Bash(frit reply:*)"]}`; string(doc["permissions"]) != want {
		t.Fatalf("permissions = %s, want %s", doc["permissions"], want)
	}

	changed, err = addReplyRules("s.json", doc, "frit")
	if err != nil || changed {
		t.Fatalf("second: changed %v, err %v; want unchanged", changed, err)
	}
}

// TestMergeSettingsKeepsHTMLCharactersReadable: a rule with `&&` is
// written as typed, not as a & escape.
func TestMergeSettingsKeepsHTMLCharactersReadable(t *testing.T) {
	dir := t.TempDir()

	data, changed, err := mergeSettings(dir, "cd a && frit")
	if err != nil || !changed {
		t.Fatalf("changed %v, err %v; want changed", changed, err)
	}
	if !contains(string(data), "Bash(cd a && frit reply:*)") {
		t.Fatalf("settings escaped the rule: %s", data)
	}
}

// TestMarshalRawIsCompactAndUnescaped: compact JSON with no trailing
// newline, and `&` left as typed.
func TestMarshalRawIsCompactAndUnescaped(t *testing.T) {
	got, err := marshalRaw([]string{"a && b"})
	if err != nil || string(got) != `["a && b"]` {
		t.Fatalf("marshalRaw = %q, %v; want [\"a && b\"]", got, err)
	}
}
