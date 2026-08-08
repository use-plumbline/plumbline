// Command plumbline lints Soroban smart contract source.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/use-plumbline/plumbline/internal/engine"
	"github.com/use-plumbline/plumbline/internal/report"
	"github.com/use-plumbline/plumbline/internal/rule"
	"github.com/use-plumbline/plumbline/rules"
)

// version is overridden at build time with -ldflags "-X main.version=...".
var version = "dev"

// Exit codes. These are the Action's contract with the workflow that runs it,
// so they are deliberately few and stable.
const (
	exitClean    = 0 // nothing at or above the fail-on threshold
	exitFindings = 1 // findings worth failing the build over
	exitUsage    = 2 // bad invocation, or a path that could not be read
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("plumbline", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var (
		format    = fs.String("format", string(report.FormatText), "output format: "+formatList())
		failOn    = fs.String("fail-on", string(rule.SeverityError), "lowest severity that fails the run: error, warning, note, or never")
		listRules = fs.Bool("list-rules", false, "list the rules and exit")
		explain   = fs.String("explain", "", "print the full documentation for one rule and exit")
		showVer   = fs.Bool("version", false, "print the version and exit")
	)
	fs.Usage = func() {
		printf(stderr, "plumbline lints Soroban smart contract source.\n\nUsage:\n  plumbline [flags] <path>...\n\nFlags:\n")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return exitUsage
	}

	registry := rules.Default()

	switch {
	case *showVer:
		printf(stdout, "plumbline %s\n", version)
		return exitClean
	case *listRules:
		writeRuleList(stdout, registry)
		return exitClean
	case *explain != "":
		r, ok := registry.Get(*explain)
		if !ok {
			printf(stderr, "plumbline: no rule named %q; run --list-rules to see them all\n", *explain)
			return exitUsage
		}
		writeRuleDoc(stdout, r.Meta())
		return exitClean
	}

	outFormat := report.Format(*format)
	if !outFormat.Valid() {
		printf(stderr, "plumbline: unknown --format %q; expected one of %s\n", *format, formatList())
		return exitUsage
	}
	threshold, ok := parseFailOn(*failOn)
	if !ok {
		printf(stderr, "plumbline: unknown --fail-on %q; expected error, warning, note, or never\n", *failOn)
		return exitUsage
	}

	paths := fs.Args()
	if len(paths) == 0 {
		fs.Usage()
		return exitUsage
	}

	res, err := engine.New(registry).Run(paths)
	if err != nil {
		printf(stderr, "plumbline: %v\n", err)
		return exitUsage
	}
	if err := report.Write(stdout, res, outFormat); err != nil {
		printf(stderr, "plumbline: writing output: %v\n", err)
		return exitUsage
	}

	for _, s := range threshold {
		if res.HasSeverity(s) {
			return exitFindings
		}
	}
	return exitClean
}

// printf writes to w and deliberately drops the error. Every caller is
// emitting a usage message or a finding to a terminal, and there is nothing
// useful to do when that fails — the one write whose failure matters is the
// report itself, which goes through report.Write and returns an error.
func printf(w io.Writer, format string, args ...any) {
	_, _ = fmt.Fprintf(w, format, args...)
}

// parseFailOn returns the severities that should fail the run.
//
// Severities are an unordered set in the rule package rather than ranked
// integers, so the ranking lives here — the one place that needs it.
func parseFailOn(name string) ([]rule.Severity, bool) {
	switch strings.ToLower(name) {
	case "error":
		return []rule.Severity{rule.SeverityError}, true
	case "warning":
		return []rule.Severity{rule.SeverityError, rule.SeverityWarning}, true
	case "note":
		return []rule.Severity{rule.SeverityError, rule.SeverityWarning, rule.SeverityNote}, true
	case "never":
		return nil, true
	}
	return nil, false
}

func formatList() string {
	names := make([]string, len(report.Formats))
	for i, f := range report.Formats {
		names[i] = string(f)
	}
	return strings.Join(names, ", ")
}

func writeRuleList(w io.Writer, reg *rule.Registry) {
	for _, r := range reg.Rules() {
		m := r.Meta()
		printf(w, "%-22s %-8s %s\n", m.ID, m.Severity, m.Summary)
	}
}

func writeRuleDoc(w io.Writer, m rule.Meta) {
	printf(w, "%s (%s)\n\n%s\n\nWhy it matters\n  %s\n\nHow to fix it\n  %s\n",
		m.ID, m.Severity, m.Summary, wrap(m.Why, 76, "  "), wrap(m.Fix, 76, "  "))
}

// wrap reflows text to width, indenting every line after the first.
func wrap(text string, width int, indent string) string {
	var lines []string
	line := ""
	for _, word := range strings.Fields(text) {
		switch {
		case line == "":
			line = word
		case len(line)+1+len(word) <= width:
			line += " " + word
		default:
			lines = append(lines, line)
			line = word
		}
	}
	if line != "" {
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n"+indent)
}
