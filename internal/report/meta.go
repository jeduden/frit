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

// InitDoc is the set of files `frit init` wrote.
type InitDoc struct {
	header
	Paths []string `json:"paths"`
}

// Init reports the files that were written.
//
// init is the one command that is not read-only, and its document says
// only what it did: the paths an agent now owns and can read or amend —
// the .frit.yml config and the plan/proto.md schema beside it. The list
// is `[]` never null, so a consumer ranges over it without a nil test.
func Init(paths []string) InitDoc {
	if paths == nil {
		paths = []string{}
	}

	return InitDoc{header: newHeader("init"), Paths: paths}
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
