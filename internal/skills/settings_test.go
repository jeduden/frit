package skills

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// readAllow reads permissions.allow from a repository's
// .claude/settings.json.
func readAllow(t *testing.T, dir string) []string {
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
		t.Fatalf("parsing settings: %v", err)
	}

	return doc.Permissions.Allow
}

func count(list []string, want string) int {
	n := 0
	for _, s := range list {
		if s == want {
			n++
		}
	}

	return n
}

// TestGrantsFollowTheInvocation: the reply rule names the command the
// way the installed skills run it, and loading plan-reply is granted
// beside it.
func TestGrantsFollowTheInvocation(t *testing.T) {
	got := Grants("go run ./cmd/frit")

	want := []string{"Bash(go run ./cmd/frit reply:*)", "Skill(plan-reply)"}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("Grants = %v, want %v", got, want)
	}
}

// TestInstallWritesTheGrantsIntoANewSettingsFile: a repository with no
// settings file gains one carrying both rules, and the path is
// reported with the skills.
func TestInstallWritesTheGrantsIntoANewSettingsFile(t *testing.T) {
	dir := t.TempDir()

	paths, err := Install(dir, false, "")
	if err != nil {
		t.Fatalf("Install: %v", err)
	}

	allow := readAllow(t, dir)
	for _, g := range Grants("frit") {
		if count(allow, g) != 1 {
			t.Fatalf("allow %v lacks %s", allow, g)
		}
	}
	settings := filepath.Join(dir, ".claude", "settings.json")
	if count(paths, settings) != 1 {
		t.Fatalf("paths %v do not report %s", paths, settings)
	}
}

// TestInstallMergesAnExistingSettingsFile: other keys and other rules
// survive, and the grants are added once.
func TestInstallMergesAnExistingSettingsFile(t *testing.T) {
	dir := t.TempDir()
	writeSettings(t, dir, `{"enabledPlugins":{"x":true},`+
		`"permissions":{"allow":["Bash(make:*)"],"deny":["Bash(rm:*)"]}}`)

	if _, err := Install(dir, false, "mise exec -- frit"); err != nil {
		t.Fatalf("Install: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, ".claude", "settings.json"))
	if err != nil {
		t.Fatalf("reading settings: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("parsing settings: %v", err)
	}
	if doc["enabledPlugins"] == nil {
		t.Fatal("another top-level key was dropped")
	}
	perms := doc["permissions"].(map[string]any)
	if perms["deny"] == nil {
		t.Fatal("the deny list was dropped")
	}
	allow := readAllow(t, dir)
	for _, want := range []string{
		"Bash(make:*)", "Bash(mise exec -- frit reply:*)", "Skill(plan-reply)",
	} {
		if count(allow, want) != 1 {
			t.Fatalf("allow %v should carry %s once", allow, want)
		}
	}
}

// TestInstallTwiceRepeatsNoGrant: a forced second install leaves the
// settings unchanged and does not report them as written.
func TestInstallTwiceRepeatsNoGrant(t *testing.T) {
	dir := t.TempDir()
	if _, err := Install(dir, false, ""); err != nil {
		t.Fatalf("first Install: %v", err)
	}
	settings := filepath.Join(dir, ".claude", "settings.json")
	before, err := os.ReadFile(settings)
	if err != nil {
		t.Fatalf("reading settings: %v", err)
	}

	paths, err := Install(dir, true, "")
	if err != nil {
		t.Fatalf("second Install: %v", err)
	}

	after, err := os.ReadFile(settings)
	if err != nil {
		t.Fatalf("reading settings: %v", err)
	}
	if string(before) != string(after) {
		t.Fatalf("a second install changed the settings:\n%s\n%s", before, after)
	}
	if count(paths, settings) != 0 {
		t.Fatal("unchanged settings were reported as written")
	}
}

// TestInstallRefusesMalformedSettingsBeforeWritingAnything: a settings
// file frit cannot merge into stops the install before a skill lands.
func TestInstallRefusesMalformedSettingsBeforeWritingAnything(t *testing.T) {
	for name, body := range map[string]string{
		"not json":       `{not json`,
		"not an object":  `[]`,
		"permissions":    `{"permissions":[]}`,
		"allow":          `{"permissions":{"allow":"Bash(x)"}}`,
		"allow non-text": `{"permissions":{"allow":[1]}}`,
	} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			writeSettings(t, dir, body)

			if _, err := Install(dir, false, ""); err == nil {
				t.Fatal("Install accepted settings it cannot merge into")
			}

			if _, err := os.Stat(filepath.Join(dir, ".claude", "skills")); !os.IsNotExist(err) {
				t.Fatalf("a skill was written before the refusal: %v", err)
			}
		})
	}
}

// TestInstallSurfacesAnUnreadableSettingsFile: a settings path that
// cannot be read as a file is a fault, not an absent file.
func TestInstallSurfacesAnUnreadableSettingsFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".claude", "settings.json"), 0o750); err != nil {
		t.Fatalf("staging: %v", err)
	}

	if _, err := Install(dir, false, ""); err == nil {
		t.Fatal("Install read a directory as settings")
	}
}

func writeSettings(t *testing.T, dir, body string) {
	t.Helper()
	path := filepath.Join(dir, ".claude", "settings.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("staging: %v", err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("staging: %v", err)
	}
}

// TestDogfoodSettingsCarryTheGrants pins frit's own settings, written
// by `frit skills --via "go run ./cmd/frit"`, to the rules that
// invocation needs.
func TestDogfoodSettingsCarryTheGrants(t *testing.T) {
	allow := readAllow(t, filepath.Join("..", ".."))
	for _, g := range Grants("go run ./cmd/frit") {
		if count(allow, g) != 1 {
			t.Fatalf("frit's own settings %v lack %s", allow, g)
		}
	}
}

// TestStoreSettingsFailsWhereNoDirectoryCanBeMade: a plain file where
// .claude would go is a write failure handed back.
func TestStoreSettingsFailsWhereNoDirectoryCanBeMade(t *testing.T) {
	blocker := filepath.Join(t.TempDir(), ".claude")
	if err := os.WriteFile(blocker, nil, 0o600); err != nil {
		t.Fatalf("staging: %v", err)
	}

	if err := storeSettings(filepath.Join(blocker, "settings.json"), map[string]any{}); err == nil {
		t.Fatal("storeSettings wrote through a file")
	}
}
