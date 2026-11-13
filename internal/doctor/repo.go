package doctor

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/jeduden/frit/internal/repocfg"
	"gopkg.in/yaml.v3"
)

// repoConfigFile is the committed per-repository settings file.
const repoConfigFile = repocfg.FileName

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
// any of the shapes git accepts: a URL with a scheme — https or ssh://,
// with or without credentials or a port — or scp-like ssh. The host is
// matched whole and without regard to case, ssh.github.com included,
// since that is GitHub's own ssh-over-443 name. ok is false for any
// other host or shape.
func GitHubRepo(remote string) (owner, name string, ok bool) {
	host, path := "", ""
	if strings.Contains(remote, "://") {
		u, err := url.Parse(remote)
		if err != nil {
			return "", "", false
		}
		host, path = u.Hostname(), u.Path
	} else if before, after, found := strings.Cut(remote, ":"); found {
		// scp-like: [user@]host:path, the user being whatever precedes
		// the last @.
		host, path = before[strings.LastIndex(before, "@")+1:], after
	}
	if !isGitHubHost(host) {
		return "", "", false
	}
	parts := strings.Split(
		strings.TrimSuffix(strings.Trim(path, "/"), ".git"), "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}

	return parts[0], parts[1], true
}

// isGitHubHost reports whether host is github.com itself, by either of
// the names GitHub serves git under.
func isGitHubHost(host string) bool {
	return strings.EqualFold(host, "github.com") ||
		strings.EqualFold(host, "ssh.github.com")
}

// CheckAutoDelete asks GitHub, through gh, whether the repository
// deletes a pull request's branch when it merges. A lane's PR is opened
// from plan/<id>, so with the setting off every landed lease branch
// stays on origin until a host that can delete reaps it. A gh that is
// missing or fails is a note that the check was skipped, never a pass —
// and so is any answer but true or false: GitHub leaves the field out
// for a token without push access, which gh prints as null, and a
// setting frit could not read is not one it may call off.
func CheckAutoDelete(
	owner, name string, gh func(args ...string) ([]byte, error),
) (Finding, bool) {
	finding := Finding{Path: owner + "/" + name, Check: "auto-delete"}
	out, err := gh("api", "repos/"+owner+"/"+name, "--jq", ".delete_branch_on_merge")
	answer := strings.TrimSpace(string(out))
	switch {
	case errors.Is(err, exec.ErrNotFound):
		finding.Message = "not checked: gh is not installed"
	case err != nil:
		finding.Message = "not checked: gh: " + ghFailure(err)
	case answer == "true":
		return Finding{}, false
	case answer != "false":
		finding.Message = "not checked: GitHub did not show the setting; " +
			"gh's token may lack push access to the repository"
	default:
		finding.Message = "\"Automatically delete head branches\" is off, so a " +
			"merged plan/<id> branch stays on origin; turn it on in the " +
			"repository's Settings > General"
	}

	return finding, true
}

// ghFailure words why a gh call failed. A gh that exits non-zero says
// why on its stderr, which exec keeps beside an error reading only
// "exit status 1"; its first line is the reason, with gh's own "gh: "
// prefix dropped since the note already carries one.
func ghFailure(err error) string {
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		line, _, _ := strings.Cut(strings.TrimSpace(string(exit.Stderr)), "\n")
		if line = strings.TrimPrefix(line, "gh: "); line != "" {
			return line
		}
	}

	return err.Error()
}
