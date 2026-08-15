package report

import (
	"encoding/json"
	"io"

	"github.com/use-plumbline/plumbline/internal/engine"
	"github.com/use-plumbline/plumbline/internal/rule"
)

// SchemaVersion identifies the shape of the JSON output.
//
// It is what a consumer should branch on, not Plumbline's own version: the
// two move for different reasons, and a tool that parses this output cares
// about the field names, not about which rules shipped that week. It is
// incremented only when an existing field changes meaning or disappears;
// adding a field does not move it, so consumers must ignore fields they do
// not recognise.
const SchemaVersion = 1

// jsonReport is the whole of one run.
//
// Every slice is emitted even when empty, as [] rather than null, so a
// consumer can index into it without a nil check — the single most common way
// a JSON contract breaks its callers.
type jsonReport struct {
	SchemaVersion int           `json:"schemaVersion"`
	Findings      []jsonFinding `json:"findings"`
	Skipped       []jsonSkipped `json:"skipped"`
	Summary       jsonSummary   `json:"summary"`
}

// jsonFinding is one finding. Field names are the ones a linter consumer
// expects rather than Plumbline's internal spelling: "file", not "path".
type jsonFinding struct {
	Rule     string `json:"rule"`
	Severity string `json:"severity"`
	File     string `json:"file"`
	Line     int    `json:"line"`
	Column   int    `json:"column"`
	Message  string `json:"message"`
}

// jsonSkipped is a file Plumbline declined to lint. These are reported rather
// than dropped: a run that skipped the contract it was pointed at is not a
// clean run, and a consumer counting only findings would read it as one.
type jsonSkipped struct {
	File   string `json:"file"`
	Reason string `json:"reason"`
}

type jsonSummary struct {
	// FilesLinted counts files parsed and checked, excluding skipped ones.
	FilesLinted int `json:"filesLinted"`
	Findings    int `json:"findings"`
	Errors      int `json:"errors"`
	Warnings    int `json:"warnings"`
	Notes       int `json:"notes"`
}

func writeJSON(w io.Writer, res *engine.Result) error {
	out := jsonReport{
		SchemaVersion: SchemaVersion,
		Findings:      make([]jsonFinding, 0, len(res.Findings)),
		Skipped:       make([]jsonSkipped, 0, len(res.Skipped)),
		Summary: jsonSummary{
			FilesLinted: res.Linted,
			Findings:    len(res.Findings),
		},
	}
	for _, f := range res.Findings {
		out.Findings = append(out.Findings, jsonFinding{
			Rule:     f.RuleID,
			Severity: string(f.Severity),
			File:     f.Path,
			Line:     f.Line,
			Column:   f.Column,
			Message:  f.Message,
		})
		switch f.Severity {
		case rule.SeverityError:
			out.Summary.Errors++
		case rule.SeverityWarning:
			out.Summary.Warnings++
		case rule.SeverityNote:
			out.Summary.Notes++
		}
	}
	for _, s := range res.Skipped {
		out.Skipped = append(out.Skipped, jsonSkipped{File: s.Path, Reason: s.Reason})
	}

	// Indented and newline-terminated: this lands in CI logs as often as in
	// a parser, and jq is not always the next thing to touch it.
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}
