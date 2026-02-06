package main

import (
	"flag"
	"fmt"
	"os"
)

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
			printResult(r, detailLimit)
		}
	}

	if !hasProblems {
		fmt.Println("repo ok")
	}

	if hasFailures(allResults) {
		os.Exit(1)
	}
}

func printResult(r Result, detailLimit int) {
	fmt.Printf("%-4s %-24s %s\n", r.Status, r.Name, r.Message)
	if detailLimit == 0 || len(r.Details) == 0 {
		return
	}
	show := len(r.Details)
	if detailLimit > 0 && show > detailLimit {
		show = detailLimit
	}
	for _, d := range r.Details[:show] {
		fmt.Printf("      %s\n", d)
	}
	if remaining := len(r.Details) - show; remaining > 0 {
		fmt.Printf("      ...and %d more\n", remaining)
	}
}

func hasFailures(results []Result) bool {
	for _, r := range results {
		if r.Status == StatusFail {
			return true
		}
	}
	return false
}
