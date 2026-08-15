package report

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/use-plumbline/plumbline/internal/engine"
	"github.com/use-plumbline/plumbline/internal/rule"
)

// decode renders res as JSON and parses it back, so the test asserts on what a
// consumer actually receives rather than on Go structs it never sees.
func decode(t *testing.T, res *engine.Result) map[string]any {
	t.Helper()
	var buf bytes.Buffer
	if err := Write(&buf, res, FormatJSON); err != nil {
		t.Fatalf("write: %v", err)
	}
	var out map[string]any
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, buf.String())
	}
	return out
}

func TestJSONFinding(t *testing.T) {
	res := &engine.Result{Linted: 2, Findings: []rule.Finding{{
		Path: "src/lib.rs", RuleID: "missing-auth", Severity: rule.SeverityError,
		Message: "set_admin writes storage", Line: 12, Column: 5,
	}}}

	got := decode(t, res)
	if got["schemaVersion"] != float64(SchemaVersion) {
		t.Errorf("schemaVersion is %v, want %d", got["schemaVersion"], SchemaVersion)
	}

	findings, ok := got["findings"].([]any)
	if !ok || len(findings) != 1 {
		t.Fatalf("findings is %v, want one entry", got["findings"])
	}
	// Every field a consumer needs to locate and triage the finding.
	want := map[string]any{
		"rule":     "missing-auth",
		"severity": "error",
		"file":     "src/lib.rs",
		"line":     float64(12),
		"column":   float64(5),
		"message":  "set_admin writes storage",
	}
	f := findings[0].(map[string]any)
	for k, v := range want {
		if f[k] != v {
			t.Errorf("findings[0].%s = %v, want %v", k, f[k], v)
		}
	}
	if len(f) != len(want) {
		t.Errorf("finding has fields %v, want exactly %v", f, want)
	}
}

func TestJSONSummaryCountsBySeverity(t *testing.T) {
	res := &engine.Result{Linted: 4, Findings: []rule.Finding{
		{Severity: rule.SeverityError}, {Severity: rule.SeverityError},
		{Severity: rule.SeverityWarning}, {Severity: rule.SeverityNote},
	}}
	summary := decode(t, res)["summary"].(map[string]any)
	for key, want := range map[string]float64{
		"filesLinted": 4, "findings": 4, "errors": 2, "warnings": 1, "notes": 1,
	} {
		if summary[key] != want {
			t.Errorf("summary.%s = %v, want %v", key, summary[key], want)
		}
	}
}

func TestJSONReportsSkippedFiles(t *testing.T) {
	res := &engine.Result{Skipped: []engine.Skipped{
		{Path: "broken.rs", Reason: "could not be parsed as Rust"},
	}}
	skipped := decode(t, res)["skipped"].([]any)
	if len(skipped) != 1 {
		t.Fatalf("got %v, want one skipped file", skipped)
	}
	s := skipped[0].(map[string]any)
	if s["file"] != "broken.rs" || s["reason"] != "could not be parsed as Rust" {
		t.Errorf("skipped entry is %v", s)
	}
}

// A consumer that has to nil-check before iterating is a consumer that will
// forget to, so empty collections are [] and not null.
func TestJSONEmptyCollectionsAreArrays(t *testing.T) {
	var buf bytes.Buffer
	if err := Write(&buf, &engine.Result{}, FormatJSON); err != nil {
		t.Fatalf("write: %v", err)
	}
	var out struct {
		Findings []jsonFinding `json:"findings"`
		Skipped  []jsonSkipped `json:"skipped"`
	}
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.Findings == nil || out.Skipped == nil {
		t.Errorf("empty collections rendered as null:\n%s", buf.String())
	}
}
