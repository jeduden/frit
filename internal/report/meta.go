package report

// VersionDoc is what `frit version` printed.
type VersionDoc struct {
	header
	Version string `json:"version"`
}

// Version reports the build version.
func Version(v string) VersionDoc {
	return VersionDoc{header: newHeader("version"), Version: v}
}

// InitDoc is the file `frit init` wrote.
type InitDoc struct {
	header
	Path string `json:"path"`
}

// Init reports the config file that was written.
//
// init is the one command that is not read-only, and its document says
// only what it did: an agent that ran it needs the path back to read
// or amend the file it now owns.
func Init(path string) InitDoc {
	return InitDoc{header: newHeader("init"), Path: path}
}

// SkillsDoc is the set of skill files `frit skills` wrote.
type SkillsDoc struct {
	header
	Paths []string `json:"paths"`
}

// Skills reports the skill files that were written.
//
// Like init, skills is not read-only, and its document says only what
// it did: the paths an agent now owns and can read or amend. The list
// is `[]` never null, so a consumer ranges over it without a nil test.
func Skills(paths []string) SkillsDoc {
	if paths == nil {
		paths = []string{}
	}

	return SkillsDoc{header: newHeader("skills"), Paths: paths}
}
