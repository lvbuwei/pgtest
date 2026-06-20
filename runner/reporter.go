package runner

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

// Status represents a test result status.
type Status string

const (
	StatusPassed  Status = "PASSED"
	StatusFailed  Status = "FAILED"
	StatusSkipped Status = "SKIPPED"
	StatusError   Status = "ERROR"
)

// ReportLine holds a single test line result.
type ReportLine struct {
	Status   Status        `json:"status"`
	Name     string        `json:"name"`
	Duration time.Duration `json:"duration"`
	Error    string        `json:"error,omitempty"`
	Retries  int           `json:"retries,omitempty"`
}

// Summary holds overall test run statistics.
type Summary struct {
	Total    int           `json:"total"`
	Passed   int           `json:"passed"`
	Failed   int           `json:"failed"`
	Skipped  int           `json:"skipped"`
	Errors   int           `json:"errors"`
	Duration time.Duration `json:"duration"`
}

// Reporter formats and outputs test results.
type Reporter struct {
	Format    string // "console" (default), "json", "junit"
	out       *os.File
	lines     []ReportLine
	startTime time.Time
}

// NewReporter creates a new Reporter.
func NewReporter(format string) *Reporter {
	return &Reporter{
		Format:    format,
		out:       os.Stdout,
		startTime: time.Now(),
	}
}

// Add appends a result line.
func (r *Reporter) Add(line ReportLine) {
	r.lines = append(r.lines, line)
}

// Summary returns the aggregate summary.
func (r *Reporter) Summary() Summary {
	s := Summary{Total: len(r.lines)}
	for _, line := range r.lines {
		switch line.Status {
		case StatusPassed:
			s.Passed++
		case StatusFailed:
			s.Failed++
		case StatusSkipped:
			s.Skipped++
		case StatusError:
			s.Errors++
		}
	}
	s.Duration = time.Since(r.startTime)
	return s
}

// Flush writes all buffered results.
func (r *Reporter) Flush() {
	switch r.Format {
	case "json":
		r.flushJSON()
	case "junit":
		r.flushJUnit()
	default:
		r.flushConsole()
	}
}

func (r *Reporter) flushConsole() {
	for _, line := range r.lines {
		icon := "✓"
		switch line.Status {
		case StatusPassed:
			icon = "✓"
		case StatusFailed:
			icon = "✗"
		case StatusSkipped:
			icon = "○"
		case StatusError:
			icon = "⚠"
		}
		dur := line.Duration.Round(time.Millisecond)
		extra := ""
		if line.Retries > 0 {
			extra = fmt.Sprintf(" (retries: %d)", line.Retries)
		}
		fmt.Fprintf(r.out, "  %s %s [%v]%s\n", icon, line.Name, dur, extra)
		if line.Error != "" {
			fmt.Fprintf(r.out, "    Error: %s\n", line.Error)
		}
	}

	summary := r.Summary()
	fmt.Fprintf(r.out, "\n━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	fmt.Fprintf(r.out, "Results: %d total, %d passed, %d failed, %d skipped, %d errors\n",
		summary.Total, summary.Passed, summary.Failed, summary.Skipped, summary.Errors)
	fmt.Fprintf(r.out, "Duration: %v\n", summary.Duration.Round(time.Millisecond))

	if summary.Failed+summary.Errors > 0 {
		fmt.Fprintf(r.out, "Status: FAILED\n")
	} else {
		fmt.Fprintf(r.out, "Status: PASSED\n")
	}
}

func (r *Reporter) flushJSON() {
	output := map[string]interface{}{
		"tests":   r.lines,
		"summary": r.Summary(),
	}
	data, _ := json.MarshalIndent(output, "", "  ")
	fmt.Fprintln(r.out, string(data))
}

func (r *Reporter) flushJUnit() {
	s := r.Summary()
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	b.WriteString(`<testsuites>` + "\n")
	fmt.Fprintf(&b, `  <testsuite name="pgtest" tests="%d" failures="%d" errors="%d" skipped="%d" time="%.3f">`+"\n",
		s.Total, s.Failed, s.Errors, s.Skipped, s.Duration.Seconds())

	for _, line := range r.lines {
		errMsg := ""
		if line.Status == StatusFailed || line.Status == StatusError {
			errMsg = xmlEscape(line.Error)
		}
		fmt.Fprintf(&b, `    <testcase name="%s" time="%.3f">`+"\n",
			xmlEscape(line.Name), line.Duration.Seconds())
		if errMsg != "" {
			fmt.Fprintf(&b, `      <failure message="%s"></failure>`+"\n", errMsg)
		}
		if line.Status == StatusSkipped {
			b.WriteString(`      <skipped></skipped>` + "\n")
		}
		b.WriteString(`    </testcase>` + "\n")
	}
	b.WriteString(`  </testsuite>` + "\n")
	b.WriteString(`</testsuites>` + "\n")
	fmt.Fprint(r.out, b.String())
}

func xmlEscape(s string) string {
	s = strings.ReplaceAll(s, "\x26", "\x26amp;")
	s = strings.ReplaceAll(s, "\x3c", "\x26lt;")
	s = strings.ReplaceAll(s, "\x3e", "\x26gt;")
	s = strings.ReplaceAll(s, "\x22", "\x26quot;")
	s = strings.ReplaceAll(s, "\x27", "\x26apos;")
	return s
}
