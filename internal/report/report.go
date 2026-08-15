// Package report renders lint results.
//
// Formats are kept apart from the engine so that adding one — SARIF, JSON —
// does not touch rules or rule execution.
package report

import (
	"fmt"
	"io"
	"strings"

	"github.com/use-plumbline/plumbline/internal/engine"
	"github.com/use-plumbline/plumbline/internal/rule"
)

// Format names an output rendering.
type Format string

const (
	// FormatText is for a terminal: one line per finding, then a summary.
	FormatText Format = "text"

	// FormatGitHub emits GitHub Actions workflow commands, which the runner
	// turns into inline annotations on a pull request diff.
	FormatGitHub Format = "github"

	// FormatJSON emits one machine-readable object, for tooling that wants
	// to do something with the findings other than print them.
	FormatJSON Format = "json"
)

// Formats lists every supported format, for flag help and validation.
var Formats = []Format{FormatText, FormatGitHub, FormatJSON}

// Valid reports whether f is a format Write understands.
func (f Format) Valid() bool {
	for _, known := range Formats {
		if f == known {
			return true
		}
	}
	return false
}

// Write renders res in the given format.
func Write(w io.Writer, res *engine.Result, f Format) error {
	switch f {
	case FormatGitHub:
		return writeGitHub(w, res)
	case FormatJSON:
		return writeJSON(w, res)
	case FormatText:
		return writeText(w, res)
	}
	return fmt.Errorf("unknown output format %q", string(f))
}

func writeText(w io.Writer, res *engine.Result) error {
	for _, s := range res.Skipped {
		if _, err := fmt.Fprintf(w, "%s: skipped: %s\n", s.Path, s.Reason); err != nil {
			return err
		}
	}
	for _, f := range res.Findings {
		if _, err := fmt.Fprintf(w, "%s:%d:%d: %s: [%s] %s\n",
			f.Path, f.Line, f.Column, f.Severity, f.RuleID, f.Message); err != nil {
			return err
		}
	}

	files := "files"
	if res.Linted == 1 {
		files = "file"
	}
	if len(res.Findings) == 0 {
		_, err := fmt.Fprintf(w, "\nNo findings in %d %s.\n", res.Linted, files)
		return err
	}
	_, err := fmt.Fprintf(w, "\n%s in %d %s.\n", countBySeverity(res.Findings), res.Linted, files)
	return err
}

// annotationLevel maps a severity to the three levels GitHub Actions
// understands. GitHub spells the lowest one "notice".
func annotationLevel(s rule.Severity) string {
	switch s {
	case rule.SeverityError:
		return "error"
	case rule.SeverityWarning:
		return "warning"
	default:
		return "notice"
	}
}

func writeGitHub(w io.Writer, res *engine.Result) error {
	for _, s := range res.Skipped {
		if _, err := fmt.Fprintf(w, "::warning file=%s::%s\n",
			s.Path, escapeData(fmt.Sprintf("Plumbline skipped this file: %s", s.Reason))); err != nil {
			return err
		}
	}
	for _, f := range res.Findings {
		// The title shows as the annotation's heading in the PR UI, so it
		// carries the rule ID and the message body carries the detail.
		if _, err := fmt.Fprintf(w, "::%s file=%s,line=%d,col=%d,title=%s::%s\n",
			annotationLevel(f.Severity),
			escapeProperty(f.Path),
			f.Line,
			f.Column,
			escapeProperty("Plumbline: "+f.RuleID),
			escapeData(f.Message),
		); err != nil {
			return err
		}
	}
	return nil
}

// escapeData escapes a workflow command's message body. An unescaped newline
// or percent sign would end the command early or be read as an escape.
func escapeData(s string) string {
	return strings.NewReplacer("%", "%25", "\r", "%0D", "\n", "%0A").Replace(s)
}

// escapeProperty escapes a workflow command property value, which additionally
// must not contain a comma or colon.
func escapeProperty(s string) string {
	return strings.NewReplacer(
		"%", "%25", "\r", "%0D", "\n", "%0A", ":", "%3A", ",", "%2C",
	).Replace(s)
}

// countBySeverity renders "2 errors, 1 warning" in a fixed order.
func countBySeverity(fs []rule.Finding) string {
	counts := map[rule.Severity]int{}
	for _, f := range fs {
		counts[f.Severity]++
	}
	var parts []string
	for _, s := range []rule.Severity{rule.SeverityError, rule.SeverityWarning, rule.SeverityNote} {
		if n := counts[s]; n > 0 {
			parts = append(parts, fmt.Sprintf("%d %s%s", n, s, plural(n)))
		}
	}
	return strings.Join(parts, ", ")
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}
