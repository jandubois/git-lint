package main

const (
	StatusOK   = "ok"
	StatusWarn = "warn"
	StatusFail = "fail"
	StatusFix  = "fix"
)

type Result struct {
	Name    string   // e.g. "identity/email"
	Status  string   // "ok", "warn", "fail", "fix"
	Message string
	Details []string // per-item detail lines (filenames, commits, etc.)
	Fixable bool
}

type Check interface {
	Check(repo *Repo) []Result
	Fix(repo *Repo, results []Result) []Result
}
