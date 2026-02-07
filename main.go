package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ANSI escape codes for TTY output.
const (
	ansiReset  = "\033[0m"
	ansiBold   = "\033[1m"
	ansiDim    = "\033[2m"
	ansiRed    = "\033[31m"
	ansiGreen  = "\033[32m"
	ansiYellow = "\033[33m"
	ansiCyan   = "\033[36m"
)

// version is set at build time via -ldflags "-X main.version=..."
var version = "dev"

var isTTY bool

func init() {
	if stat, err := os.Stdout.Stat(); err == nil {
		isTTY = (stat.Mode() & os.ModeCharDevice) != 0
	}
}

func main() {
	dir := flag.String("C", "", "run as if started in this directory")
	clone := flag.String("clone", "", "clone a GitHub repo and configure it")
	fix := flag.Bool("fix", false, "auto-fix fixable violations")
	var recursive bool
	flag.BoolVar(&recursive, "R", false, "check each git repo in subdirectories")
	flag.BoolVar(&recursive, "recursive", false, "check each git repo in subdirectories")
	verbose := flag.Bool("verbose", false, "show all checks and all detail lines")
	quiet := flag.Bool("quiet", false, "suppress detail lines")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Println("git-lint version " + version)
		return
	}

	if *dir != "" {
		if err := os.Chdir(*dir); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(2)
		}
	}

	cfg, err := loadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(2)
	}

	if *clone != "" {
		if err := cloneRepo(cfg, *clone); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(2)
		}
		return
	}

	opts := lintOptions{
		cfg:     cfg,
		fix:     *fix,
		verbose: *verbose,
		quiet:   *quiet,
	}

	if recursive {
		os.Exit(lintRecursive(opts))
	}

	wd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(2)
	}
	os.Exit(lintRepo(wd, opts))
}

type lintOptions struct {
	cfg     *Config
	fix     bool
	verbose bool
	quiet   bool
}

func lintRecursive(opts lintOptions) int {
	entries, err := os.ReadDir(".")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 2
	}

	exitCode := 0
	first := true
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if _, err := os.Stat(filepath.Join(entry.Name(), ".git")); err != nil {
			continue
		}

		absDir, err := filepath.Abs(entry.Name())
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			if exitCode < 2 {
				exitCode = 2
			}
			continue
		}

		results, code := runChecks(absDir, opts)
		if code == 2 {
			if exitCode < 2 {
				exitCode = 2
			}
			continue
		}

		hasProblems := hasNonOK(results)
		if opts.quiet && !hasProblems {
			continue
		}

		if !first {
			fmt.Println()
		}
		first = false

		if isTTY {
			fmt.Printf("%s%s%s\n", ansiBold, entry.Name(), ansiReset)
		} else {
			fmt.Printf("=== %s ===\n", entry.Name())
		}

		printResults(results, opts)
		if code > exitCode {
			exitCode = code
		}
	}

	if first {
		if opts.quiet {
			return exitCode
		}
		fmt.Fprintf(os.Stderr, "no git repos found\n")
		return 2
	}
	return exitCode
}

func lintRepo(dir string, opts lintOptions) int {
	results, code := runChecks(dir, opts)
	if code == 2 {
		return 2
	}
	printResults(results, opts)
	return code
}

func runChecks(dir string, opts lintOptions) ([]Result, int) {
	repo, err := NewRepo(dir, opts.cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return nil, 2
	}

	checks := []Check{
		&IdentityCheck{},
		&ProtocolCheck{},
		&RemoteCheck{},
		&AttributionCheck{},
		&StalenessCheck{},
		&SubmoduleCheck{},
		&BranchCleanupCheck{},
		&UnpushedCheck{},
	}

	var allResults []Result
	for _, c := range checks {
		results := c.Check(repo)
		if opts.fix {
			results = c.Fix(repo, results)
		}
		allResults = append(allResults, results...)
	}

	if hasFailures(allResults) {
		return allResults, 1
	}
	return allResults, 0
}

func printResults(results []Result, opts lintOptions) {
	// Determine how many detail lines to show per result.
	// -1 = unlimited (--verbose), 0 = none (--quiet), >0 = configured limit.
	detailLimit := opts.cfg.DetailLines
	if detailLimit == 0 {
		detailLimit = 10
	}
	if opts.quiet {
		detailLimit = 0
	}
	if opts.verbose {
		detailLimit = -1
	}

	hasProblems := false
	for _, r := range results {
		if r.Status != StatusOK {
			hasProblems = true
		}
		if opts.verbose || r.Status != StatusOK {
			printResult(r, detailLimit, opts.verbose)
		}
	}

	if !hasProblems {
		if isTTY {
			fmt.Printf("%s✓ repo ok%s\n", ansiGreen, ansiReset)
		} else {
			fmt.Println("repo ok")
		}
	}
}

func hasNonOK(results []Result) bool {
	for _, r := range results {
		if r.Status != StatusOK {
			return true
		}
	}
	return false
}

func printResult(r Result, detailLimit int, verbose bool) {
	if isTTY {
		printResultTTY(r, verbose)
	} else {
		fix := ""
		if r.Fixable && r.Status == StatusWarn {
			fix = " [--fix]"
		}
		fmt.Printf("%-4s %-24s %s%s\n", r.Status, r.Name, r.Message, fix)
	}

	if detailLimit == 0 || len(r.Details) == 0 {
		return
	}
	show := len(r.Details)
	if detailLimit > 0 && show > detailLimit {
		show = detailLimit
	}
	for _, d := range r.Details[:show] {
		if isTTY {
			fmt.Printf("  %s%s%s\n", ansiDim, d, ansiReset)
		} else {
			fmt.Printf("      %s\n", d)
		}
	}
	if remaining := len(r.Details) - show; remaining > 0 {
		if isTTY {
			fmt.Printf("  %s...and %d more%s\n", ansiDim, remaining, ansiReset)
		} else {
			fmt.Printf("      ...and %d more\n", remaining)
		}
	}
}

func printResultTTY(r Result, verbose bool) {
	rule, param := splitResultName(r.Name)

	// Status marker: verbose always shows one; non-verbose only for fail/fix/fixable.
	var marker string
	switch r.Status {
	case StatusOK:
		marker = ansiGreen + "✓" + ansiReset + " "
	case StatusWarn:
		if r.Fixable {
			marker = ansiCyan + "~" + ansiReset + " "
		} else if verbose {
			marker = ansiYellow + "!" + ansiReset + " "
		}
	case StatusFail:
		marker = ansiRed + "✗" + ansiReset + " "
	case StatusFix:
		marker = ansiGreen + "✓" + ansiReset + " "
	}

	// Main content: param bold+colored, then message.
	// Fixable warnings use cyan to distinguish from manual warnings.
	var content string
	if param != "" {
		color := statusColor(r.Status)
		if r.Fixable && r.Status == StatusWarn {
			color = ansiCyan
		}
		content = color + ansiBold + param + ansiReset + ": " + r.Message
	} else {
		content = r.Message
	}

	fmt.Printf("%s%s  %s(%s)%s\n", marker, content, ansiDim, rule, ansiReset)
}

func statusColor(status string) string {
	switch status {
	case StatusWarn:
		return ansiYellow
	case StatusFail:
		return ansiRed
	case StatusFix, StatusOK:
		return ansiGreen
	}
	return ""
}

// splitResultName separates "staleness/unpushed[bats]" into
// rule="staleness/unpushed" and param="bats".
func splitResultName(name string) (rule, param string) {
	if i := strings.IndexByte(name, '['); i >= 0 {
		return name[:i], name[i+1 : len(name)-1]
	}
	return name, ""
}

func hasFailures(results []Result) bool {
	for _, r := range results {
		if r.Status == StatusFail {
			return true
		}
	}
	return false
}
