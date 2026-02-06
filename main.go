package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	fix := flag.Bool("fix", false, "auto-fix fixable violations")
	verbose := flag.Bool("verbose", false, "show all checks including passing ones")
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

	hasProblems := false
	for _, r := range allResults {
		if r.Status != StatusOK {
			hasProblems = true
		}
		if *verbose || r.Status != StatusOK {
			printResult(r)
		}
	}

	if !hasProblems {
		fmt.Println("repo ok")
	}

	if hasFailures(allResults) {
		os.Exit(1)
	}
}

func printResult(r Result) {
	fmt.Printf("%-4s %-24s %s\n", r.Status, r.Name, r.Message)
}

func hasFailures(results []Result) bool {
	for _, r := range results {
		if r.Status == StatusFail {
			return true
		}
	}
	return false
}
