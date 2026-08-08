package report

import (
	"bytes"
	"strings"
	"testing"

	"github.com/use-plumbline/plumbline/internal/engine"
	"github.com/use-plumbline/plumbline/internal/rule"
)

func result(fs ...rule.Finding) *engine.Result {
	return &engine.Result{Findings: fs, Linted: 1}
}

func TestGitHubAnnotation(t *testing.T) {
	var buf bytes.Buffer
	err := Write(&buf, result(rule.Finding{
		Path: "src/lib.rs", RuleID: "missing-auth", Severity: rule.SeverityError,
		Message: "set_admin writes storage", Line: 12, Column: 5,
	}), FormatGitHub)
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	want := "::error file=src/lib.rs,line=12,col=5,title=Plumbline%3A missing-auth::set_admin writes storage\n"
	if got := buf.String(); got != want {
		t.Errorf("got  %q\nwant %q", got, want)
	}
}

func TestGitHubAnnotationLevels(t *testing.T) {
	// GitHub spells the lowest level "notice", not "note".
	for severity, want := range map[rule.Severity]string{
		rule.SeverityError:   "::error ",
		rule.SeverityWarning: "::warning ",
		rule.SeverityNote:    "::notice ",
	} {
		var buf bytes.Buffer
		f := rule.Finding{Path: "a.rs", RuleID: "r", Severity: severity, Message: "m", Line: 1, Column: 1}
		if err := Write(&buf, result(f), FormatGitHub); err != nil {
			t.Fatalf("write: %v", err)
		}
		if !strings.HasPrefix(buf.String(), want) {
			t.Errorf("severity %q rendered %q, want prefix %q", severity, buf.String(), want)
		}
	}
}

// A message carrying a newline or a percent sign would otherwise terminate the
// workflow command early and swallow the rest of the annotation.
func TestGitHubEscapesMessageAndProperties(t *testing.T) {
	var buf bytes.Buffer
	f := rule.Finding{
		Path: "a,b:c.rs", RuleID: "r", Severity: rule.SeverityWarning,
		Message: "100% wrong\nsecond line", Line: 1, Column: 1,
	}
	if err := Write(&buf, result(f), FormatGitHub); err != nil {
		t.Fatalf("write: %v", err)
	}
	got := buf.String()
	if strings.Count(got, "\n") != 1 {
		t.Errorf("annotation spans multiple lines: %q", got)
	}
	for _, want := range []string{"file=a%2Cb%3Ac.rs", "100%25 wrong%0Asecond line"} {
		if !strings.Contains(got, want) {
			t.Errorf("got %q, want it to contain %q", got, want)
		}
	}
}

func TestTextSummary(t *testing.T) {
	tests := []struct {
		name string
		res  *engine.Result
		want string
	}{
		{"clean", &engine.Result{Linted: 3}, "No findings in 3 files."},
		{"one file", &engine.Result{Linted: 1}, "No findings in 1 file."},
		{
			"mixed severities",
			&engine.Result{Linted: 2, Findings: []rule.Finding{
				{Severity: rule.SeverityWarning}, {Severity: rule.SeverityError},
				{Severity: rule.SeverityError},
			}},
			"2 errors, 1 warning in 2 files.",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			if err := Write(&buf, tc.res, FormatText); err != nil {
				t.Fatalf("write: %v", err)
			}
			if !strings.Contains(buf.String(), tc.want) {
				t.Errorf("got %q, want it to contain %q", buf.String(), tc.want)
			}
		})
	}
}

func TestTextReportsSkippedFiles(t *testing.T) {
	var buf bytes.Buffer
	res := &engine.Result{Skipped: []engine.Skipped{{Path: "broken.rs", Reason: "could not be parsed as Rust"}}}
	if err := Write(&buf, res, FormatText); err != nil {
		t.Fatalf("write: %v", err)
	}
	if !strings.Contains(buf.String(), "broken.rs: skipped: could not be parsed as Rust") {
		t.Errorf("skipped files are not surfaced: %q", buf.String())
	}
}

func TestUnknownFormatIsAnError(t *testing.T) {
	if Format("sarif").Valid() {
		t.Fatal("sarif reported as valid")
	}
	if err := Write(&bytes.Buffer{}, result(), Format("sarif")); err == nil {
		t.Fatal("want an error for an unknown format, got nil")
	}
}
