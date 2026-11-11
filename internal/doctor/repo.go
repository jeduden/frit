package doctor

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// repoConfigFile is the committed per-repository settings file.
const repoConfigFile = ".frit.yml"

// CheckHolderInRepoConfig reports a committed .frit.yml that names a
// holder. The holder names a host, and this file is shared by every
// clone, so frit ignores the key there; the finding says so rather
// than leaving it silently dead. A missing file is nothing to report.
func CheckHolderInRepoConfig(root string) (Finding, bool, error) {
	body, err := os.ReadFile(filepath.Join(root, repoConfigFile))
	if errors.Is(err, os.ErrNotExist) {
		return Finding{}, false, nil
	}
	if err != nil {
		return Finding{}, false, err
	}
	var keys map[string]any
	if err := yaml.Unmarshal(body, &keys); err != nil {
		return Finding{}, false, fmt.Errorf("read %s: %w", repoConfigFile, err)
	}
	if _, ok := keys["holder"]; !ok {
		return Finding{}, false, nil
	}

	return Finding{
		Path:  repoConfigFile,
		Check: "holder",
		Message: "holder is ignored in a committed .frit.yml, since every " +
			"clone shares it; set it with --holder, FRIT_HOLDER or the user config",
	}, true, nil
}

// GitHubRepo reads owner and repository off a github.com remote URL in
// any of the shapes git accepts: https, scp-like ssh, or ssh://. ok is
// false for any other host or shape.
func GitHubRepo(url string) (owner, name string, ok bool) {
	path := ""
	for _, prefix := range []string{
		"https://github.com/", "git@github.com:", "ssh://git@github.com/",
	} {
		if rest, found := strings.CutPrefix(url, prefix); found {
			path = rest
			break
		}
	}
	parts := strings.Split(strings.TrimSuffix(path, ".git"), "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}

	return parts[0], parts[1], true
}

// CheckAutoDelete asks GitHub, through gh, whether the repository
// deletes a pull request's branch when it merges. A lane's PR is opened
// from plan/<id>, so with the setting off every landed lease branch
// stays on origin until a host that can delete reaps it. A gh that is
// missing or fails is a note that the check was skipped, never a pass.
func CheckAutoDelete(
	owner, name string, gh func(args ...string) ([]byte, error),
) (Finding, bool) {
	finding := Finding{Path: owner + "/" + name, Check: "auto-delete"}
	out, err := gh("api", "repos/"+owner+"/"+name, "--jq", ".delete_branch_on_merge")
	switch {
	case errors.Is(err, exec.ErrNotFound):
		finding.Message = "not checked: gh is not installed"
		return finding, true
	case err != nil:
		finding.Message = "not checked: gh: " + err.Error()
		return finding, true
	case strings.TrimSpace(string(out)) == "true":
		return Finding{}, false
	}
	finding.Message = "\"Automatically delete head branches\" is off, so a " +
		"merged plan/<id> branch stays on origin; turn it on in the " +
		"repository's Settings > General"

	return finding, true
}
