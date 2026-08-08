// Package engine loads Soroban contract sources, parses them, runs the
// registered rules over each file, and collects the findings.
//
// The engine owns everything that can fail — file discovery, reading, parsing —
// so that rules do not have to.
package engine

import (
	"errors"
	"os"
	"sort"

	"github.com/use-plumbline/plumbline/internal/rule"
	"github.com/use-plumbline/plumbline/internal/syntax"
)

// Engine runs a fixed set of rules over Rust sources.
type Engine struct {
	registry *rule.Registry

	// severity overrides a rule's default severity by rule ID. It is empty
	// today; .plumbline.toml will populate it without any rule changing.
	severity map[string]rule.Severity

	// disabled turns rules off by ID. Same story.
	disabled map[string]bool
}

// New returns an Engine that runs every rule in reg with its default severity.
func New(reg *rule.Registry) *Engine {
	return &Engine{
		registry: reg,
		severity: map[string]rule.Severity{},
		disabled: map[string]bool{},
	}
}

// SetSeverity overrides the default severity of a rule.
func (e *Engine) SetSeverity(ruleID string, s rule.Severity) { e.severity[ruleID] = s }

// Disable stops a rule from running.
func (e *Engine) Disable(ruleID string) { e.disabled[ruleID] = true }

// Skipped is a file the engine declined to lint, and why.
type Skipped struct {
	Path   string
	Reason string
}

// Result is everything one run produced.
type Result struct {
	Findings []rule.Finding
	Skipped  []Skipped
	// Linted counts the files that were actually parsed and checked.
	Linted int
}

// HasSeverity reports whether any finding carries the given severity.
func (r *Result) HasSeverity(s rule.Severity) bool {
	for _, f := range r.Findings {
		if f.Severity == s {
			return true
		}
	}
	return false
}

// Run lints every Rust source found under the given paths.
//
// A path may be a file or a directory; directories are walked. Findings come
// back sorted by path, then line, then column, then rule ID, so that output is
// deterministic regardless of walk order.
func (e *Engine) Run(paths []string) (*Result, error) {
	files, err := DiscoverRust(paths)
	if err != nil {
		return nil, err
	}

	parser, err := syntax.NewParser()
	if err != nil {
		return nil, err
	}
	defer parser.Close()

	res := &Result{}
	for _, path := range files {
		src, err := os.ReadFile(path)
		if err != nil {
			res.Skipped = append(res.Skipped, Skipped{path, err.Error()})
			continue
		}
		findings, err := e.lintFile(parser, path, src)
		if err != nil {
			res.Skipped = append(res.Skipped, Skipped{path, err.Error()})
			continue
		}
		res.Linted++
		res.Findings = append(res.Findings, findings...)
	}

	sort.SliceStable(res.Findings, func(i, j int) bool {
		a, b := res.Findings[i], res.Findings[j]
		switch {
		case a.Path != b.Path:
			return a.Path < b.Path
		case a.Line != b.Line:
			return a.Line < b.Line
		case a.Column != b.Column:
			return a.Column < b.Column
		default:
			return a.RuleID < b.RuleID
		}
	})
	return res, nil
}

// lintFile parses one source and runs every enabled rule over it.
func (e *Engine) lintFile(parser *syntax.Parser, path string, src []byte) ([]rule.Finding, error) {
	tree, err := parser.Parse(src)
	if err != nil {
		return nil, err
	}
	defer tree.Close()

	root := tree.Root()
	// A rule reading a broken parse tree reports nonsense, so a file that
	// does not parse is skipped rather than guessed at.
	if root.HasError() {
		return nil, errors.New("could not be parsed as Rust")
	}

	ctx := &rule.Context{Path: path, Root: root}

	var out []rule.Finding
	for _, r := range e.registry.Rules() {
		meta := r.Meta()
		if e.disabled[meta.ID] {
			continue
		}
		severity := meta.Severity
		if s, ok := e.severity[meta.ID]; ok {
			severity = s
		}
		for _, f := range r.Check(ctx) {
			f.Path = path
			f.RuleID = meta.ID
			f.Severity = severity
			out = append(out, f)
		}
	}
	return out, nil
}
