package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"pgtest/config"
	"pgtest/dotsql"
	"pgtest/runner"
)

// Version is the current version of pgtest. Edit this constant when releasing.
const Version = "26.06.08"

func printUsage() {
	fmt.Fprintf(os.Stderr, `pgtest — PostgreSQL stored procedure / SQL testing tool
Version: %s

Usage: pgtest [options]

Options:
  -config string    Path to YAML test configuration file (default "test.yaml")
  -sql string       Path to SQL file with named queries (default "queries.sql")
  -format string    Output format: console, json, junit (default "console")
  -verbose          Enable verbose output: print DSN, connection status, and case names
  -name string      Run only the test case with the given name (e.g. -name "验证用户发单")
  -query string     Run only steps matching the given query name (e.g. -query "find-all-users")
  -notice           Print PostgreSQL RAISE NOTICE messages from stored procedures
  -stop-on-error    Stop executing remaining test cases when an error occurs (default true)
  -version          Print version information and exit
  -help             Show this help message and exit

Examples:
  pgtest
  pgtest -config mytest.yaml -sql myqueries.sql
  pgtest -format junit -verbose > report.xml
  pgtest -name "验证用户发单"
  pgtest -query "find-all-users"
  pgtest -notice
`, Version)
}

func main() {
	configPath := flag.String("config", "test.yaml", "path to YAML configuration file")
	sqlPath := flag.String("sql", "queries.sql", "path to SQL file with named queries")
	format := flag.String("format", "console", "output format: console, json, junit")
	verbose := flag.Bool("verbose", false, "enable verbose output: print DSN, connection status, and case names")
	nameFilter := flag.String("name", "", "run only the test case with the given name")
	queryFilter := flag.String("query", "", "run only steps matching the given query name")
	notice := flag.Bool("notice", false, "print PostgreSQL RAISE NOTICE messages from stored procedures")
	stopOnError := flag.Bool("stop-on-error", true, "stop executing remaining test cases when an error occurs (default true)")
	version := flag.Bool("version", false, "print version information and exit")
	help := flag.Bool("help", false, "show this help message and exit")

	flag.Parse()

	// -help flag
	if *help {
		printUsage()
		os.Exit(0)
	}

	// -version flag
	if *version {
		fmt.Printf("pgtest version %s\n", Version)
		os.Exit(0)
	}

	// No arguments and default config not present → print usage
	if len(os.Args) == 1 {
		if _, err := os.Stat("test.yaml"); os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "Error: no test.yaml found in current directory and no arguments provided.\n\n")
			printUsage()
			os.Exit(1)
		}
	}

	// If -config was explicitly set but -sql was not, derive sql path from config name
	configExplicitlySet := false
	sqlExplicitlySet := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == "config" {
			configExplicitlySet = true
		}
		if f.Name == "sql" {
			sqlExplicitlySet = true
		}
	})
	if configExplicitlySet && !sqlExplicitlySet {
		ext := filepath.Ext(*configPath)
		base := strings.TrimSuffix(*configPath, ext)
		*sqlPath = base + ".sql"
	}

	// Check config file exists
	if _, err := os.Stat(*configPath); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Error: config file not found: %s\n", *configPath)
		os.Exit(1)
	}

	// Load config
	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	// Filter by test case name if -name is specified
	if *nameFilter != "" {
		cfg = cfg.Flatten()
		var matched []config.CaseConfig
		for _, tc := range cfg.Cases {
			if tc.Name == *nameFilter {
				matched = append(matched, tc)
			}
		}
		if len(matched) == 0 {
			fmt.Fprintf(os.Stderr, "Error: no test case found with name %q\n", *nameFilter)
			os.Exit(1)
		}
		cfg.Cases = matched
		if *verbose {
			fmt.Printf("[verbose] filtered to case: %s\n", *nameFilter)
		}
	}

	// Filter steps by query name if -query is specified
	if *queryFilter != "" {
		cfg.Cases = filterCasesByQuery(cfg.Cases, *queryFilter)
		if len(cfg.Cases) == 0 {
			fmt.Fprintf(os.Stderr, "Error: no steps found matching query %q\n", *queryFilter)
			os.Exit(1)
		}
		if *verbose {
			fmt.Printf("[verbose] filtered to query: %s\n", *queryFilter)
		}
	}

	// Load SQL
	dot, err := dotsql.LoadFromFile(*sqlPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading SQL file: %v\n", err)
		os.Exit(1)
	}

	if *verbose {
		fmt.Printf("[verbose] config file: %s\n", *configPath)
		fmt.Printf("[verbose] sql file: %s\n", *sqlPath)
	}

	// Create reporter
	reporter := runner.NewReporter(*format)

	// Create and run runner
	r, err := runner.New(cfg, dot, reporter, *verbose, *notice, *stopOnError)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating runner: %v\n", err)
		os.Exit(1)
	}
	defer r.Close()

	r.Run()

	// Exit with appropriate code
	summary := reporter.Summary()
	if summary.Failed+summary.Errors > 0 {
		os.Exit(1)
	}
}

// filterCasesByQuery recursively filters test cases to only keep steps whose
// query field (or name field when query is empty) matches q. Cases with no
// matching steps after filtering are removed.
func filterCasesByQuery(cases []config.CaseConfig, q string) []config.CaseConfig {
	var out []config.CaseConfig
	for _, tc := range cases {
		// Filter steps
		var matchedSteps []config.StepConfig
		for _, step := range tc.Steps {
			// Match against step.Query; fall back to step.Name when Query is omitted
			if step.Query == q || (step.Query == "" && step.Name == q) {
				matchedSteps = append(matchedSteps, step)
			}
		}
		tc.Steps = matchedSteps

		// Recursively filter nested cases
		if len(tc.Cases) > 0 {
			tc.Cases = filterCasesByQuery(tc.Cases, q)
		}

		// Keep the case if it has any matching steps or nested cases
		if len(tc.Steps) > 0 || len(tc.Cases) > 0 {
			out = append(out, tc)
		}
	}
	return out
}
