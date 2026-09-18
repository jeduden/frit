package skills

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// ErrSettings reports a .claude/settings.json that frit cannot merge
// its allow rules into. Install refuses rather than overwrite it: the
// file holds the repository's own permissions and plugins.
var ErrSettings = errors.New("settings file cannot take the reply rule")

// settingsRel is the project settings file, relative to a repository
// root, that Claude Code reads permission rules from.
var settingsRel = filepath.Join(".claude", "settings.json")

// replyAllow are the rules that let a responder answer an ask with no
// operator sign-off: the harness prompts before it loads a skill, and
// a skill's own allowed-tools did not approve the command in a real
// session, so a project rule does both. The Bash rule is the value
// plan-reply ships in allowed-tools, with the invocation token
// substituted the way the skill's is.
var replyAllow = []string{
	"Skill(plan-reply)",
	"Bash(" + invokeToken + " reply:*)",
}

// mergeSettings returns the bytes settings.json should hold after the
// reply rules are added to permissions.allow, and whether that differs
// from the file on disk. It edits nothing else: every other key and
// rule survives, though a rewritten file's keys come back sorted and
// indented. A file that already carries both rules is left as it is.
func mergeSettings(repoDir, invoke string) ([]byte, bool, error) {
	dst := filepath.Join(repoDir, settingsRel)

	data, doc, err := loadSettings(dst)
	if err != nil {
		return nil, false, err
	}
	changed, err := addReplyRules(dst, doc, invoke)
	if err != nil || !changed {
		return data, false, err
	}

	var out bytes.Buffer
	enc := json.NewEncoder(&out)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(doc); err != nil {
		return nil, false, err
	}

	return out.Bytes(), true, nil
}

// marshalRaw encodes v as compact JSON without escaping <, > and &, so
// a rule like `cd a && frit` stays as typed. json.Marshal escapes them,
// and an escape written into a raw value survives the final encode.
func marshalRaw(v any) (json.RawMessage, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}

	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}

// loadSettings reads the settings file into its top-level keys, each
// left as raw JSON so a key frit does not edit survives untouched. A
// missing file is an empty document; one that is not a JSON object is
// ErrSettings.
func loadSettings(dst string) ([]byte, map[string]json.RawMessage, error) {
	doc := map[string]json.RawMessage{}
	data, err := os.ReadFile(dst)
	switch {
	case err == nil:
		if err := json.Unmarshal(data, &doc); err != nil {
			return nil, nil, fmt.Errorf("%s: %w: %w", dst, ErrSettings, err)
		}
	case !errors.Is(err, fs.ErrNotExist):
		return nil, nil, err
	}

	return data, doc, nil
}

// addReplyRules appends whichever reply rules permissions.allow lacks,
// with the invocation substituted, and reports whether it added any. A
// permissions value that is not an object, or an allow that is not a
// list, is ErrSettings.
func addReplyRules(dst string, doc map[string]json.RawMessage, invoke string) (bool, error) {
	perms := map[string]json.RawMessage{}
	if raw, ok := doc["permissions"]; ok {
		if err := json.Unmarshal(raw, &perms); err != nil {
			return false, fmt.Errorf("%s: permissions: %w: %w", dst, ErrSettings, err)
		}
	}
	var allow []json.RawMessage
	if raw, ok := perms["allow"]; ok {
		if err := json.Unmarshal(raw, &allow); err != nil {
			return false, fmt.Errorf("%s: permissions.allow: %w: %w", dst, ErrSettings, err)
		}
	}

	have := map[string]bool{}
	for _, raw := range allow {
		var rule string
		if json.Unmarshal(raw, &rule) == nil {
			have[rule] = true
		}
	}

	changed := false
	for _, rule := range replyAllow {
		rule = strings.ReplaceAll(rule, invokeToken, invoke)
		if have[rule] {
			continue
		}
		enc, err := marshalRaw(rule)
		if err != nil {
			return false, err
		}
		allow = append(allow, enc)
		changed = true
	}
	if !changed {
		return false, nil
	}

	var err error
	if perms["allow"], err = marshalRaw(allow); err != nil {
		return false, err
	}
	if doc["permissions"], err = marshalRaw(perms); err != nil {
		return false, err
	}

	return true, nil
}
