package skills

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// settingsFile is the repository's shared Claude Code settings,
// relative to its root: the file whose permission rules the harness
// reads for every session started there.
var settingsFile = filepath.Join(".claude", "settings.json")

// Grants is what the bundle needs pre-approved: the reply command,
// under the same invocation the skills run, and loading plan-reply.
//
// The reply writes one local file — no pane, no ref, no network — so
// approving it never approves a send. A skill's own allowed-tools
// would be the natural home, but a real session granted nothing from
// it (plan 2609181901 phase 1), while the same rule in the settings
// let the reply land.
func Grants(invoke string) []string {
	return []string{"Bash(" + invoke + " reply:*)", "Skill(plan-reply)"}
}

// readSettings reads the settings at path as a JSON object, an empty
// one when the file is absent.
func readSettings(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return map[string]any{}, nil
	}
	if err != nil {
		return nil, err
	}
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}

	return doc, nil
}

// mergeGrants adds each grant missing from doc's permissions.allow,
// keeping every other key and rule, and reports whether it added any.
// A permissions or allow of the wrong shape is refused rather than
// overwritten: it is the operator's file.
func mergeGrants(doc map[string]any, grants []string) (bool, error) {
	perms := map[string]any{}
	if raw, ok := doc["permissions"]; ok {
		p, isObj := raw.(map[string]any)
		if !isObj {
			return false, errors.New("settings: permissions is not an object")
		}
		perms = p
	}
	var allow []any
	if raw, ok := perms["allow"]; ok {
		a, isList := raw.([]any)
		if !isList {
			return false, errors.New("settings: permissions.allow is not a list")
		}
		allow = a
	}
	have := make(map[string]bool, len(allow))
	for _, rule := range allow {
		s, isText := rule.(string)
		if !isText {
			return false, errors.New("settings: permissions.allow holds a non-string rule")
		}
		have[s] = true
	}

	added := false
	for _, g := range grants {
		if !have[g] {
			allow = append(allow, g)
			added = true
		}
	}
	perms["allow"] = allow
	doc["permissions"] = perms

	return added, nil
}

// storeSettings stores doc at path as indented JSON.
func storeSettings(path string, doc map[string]any) error {
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	return os.WriteFile(path, append(data, '\n'), 0o644)
}
