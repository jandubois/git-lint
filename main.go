package main

import (
	"flag"
	"fmt"
	"os"
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

var isTTY bool

func init() {
	if stat, err := os.Stdout.Stat(); err == nil {
		isTTY = (stat.Mode() & os.ModeCharDevice) != 0
	}
}

func main() {
	fix := flag.Bool("fix", false, "auto-fix fixable violations")
	verbose := flag.Bool("verbose", false, "show all checks and all detail lines")
	quiet := flag.Bool("quiet", false, "suppress detail lines")
	flag.Parse()

	cfg, err := loadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(2)
	}

	dir, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(2)
	}

	repo, err := NewRepo(dir, cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(2)
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
		if *fix {
			results = c.Fix(repo, results)
		}
		allResults = append(allResults, results...)
	}

	// Determine how many detail lines to show per result.
	// -1 = unlimited (--verbose), 0 = none (--quiet), >0 = configured limit.
	detailLimit := cfg.DetailLines
	if detailLimit == 0 {
		detailLimit = 10
	}
	if *quiet {
		detailLimit = 0
	}
	if *verbose {
		detailLimit = -1
	}

	hasProblems := false
	for _, r := range allResults {
		if r.Status != StatusOK {
			hasProblems = true
		}
		if *verbose || r.Status != StatusOK {
			printResult(r, detailLimit, *verbose)
		}
	}

	if !hasProblems {
		if isTTY {
			fmt.Printf("%s✓ repo ok%s\n", ansiGreen, ansiReset)
		} else {
			fmt.Println("repo ok")
		}
	}

	if hasFailures(allResults) {
		os.Exit(1)
	}
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
