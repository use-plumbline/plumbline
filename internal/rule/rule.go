// Package rule defines what a Plumbline lint rule is: the interface every rule
// implements, the value it reports, the context it is handed, and the registry
// that holds the set of rules to run.
//
// Nothing here knows about any specific rule, and no rule knows about any
// other. A rule is handed one already-parsed file and returns what it found.
//
// The parser is not part of this API. Rules see [Node], Plumbline's own view of
// a syntax tree, so that changing or replacing the parser does not touch a
// single rule.
package rule

import (
	"fmt"

	"github.com/use-plumbline/plumbline/internal/syntax"
)

// Node is a node of a parsed contract source file.
//
// It is an alias rather than a wrapper so that a rule imports one package and
// gets everything it needs.
type Node = syntax.Node

// Severity ranks a finding. A rule declares a default; how severities map to
// exit codes and to annotation levels is decided further out.
type Severity string

// The severities a rule may declare. They are deliberately unranked here:
// deciding which of them should fail a build is policy, and lives in the CLI.
const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
	SeverityNote    Severity = "note"
)

// Meta is a rule's static identity — everything true about the rule before it
// has looked at any source. Documentation and configuration key off this, so
// the CLI can list and explain rules without parsing a single file.
type Meta struct {
	// ID is the stable kebab-case name used in output and configuration.
	// It must not change once the rule has shipped.
	ID string

	// Severity is the rule's default severity. Configuration is allowed to
	// override it, so a rule must not assume its findings keep this value.
	Severity Severity

	// Summary is a one-line statement of what the rule looks for.
	Summary string

	// Why explains what goes wrong on chain if the finding is ignored.
	Why string

	// Fix states the concrete remedy.
	Fix string
}

// Finding is one reported problem at one location.
//
// A rule sets Message, Line and Column — use [At], which reads the position
// off a node. The engine stamps Path, RuleID and Severity, so configuration
// can remap severity in one place instead of every rule consulting config.
type Finding struct {
	Path     string
	RuleID   string
	Severity Severity
	Message  string
	Line     int // 1-indexed
	Column   int // 1-indexed
}

// At builds a Finding positioned at the start of n.
func At(n Node, format string, args ...any) Finding {
	return Finding{
		Message: fmt.Sprintf(format, args...),
		Line:    n.Line(),
		Column:  n.Column(),
	}
}

// Context is one parsed source file, presented to one rule.
//
// It is read-only as far as rules are concerned. The engine builds one per
// file and hands the same value to every rule, so parsing happens once.
type Context struct {
	// Path is the file path as given to the engine; it appears in output.
	Path string

	// Root is the file's root node.
	Root Node

	contractFns    []ContractFn
	contractFnsSet bool
}

// ContractFn is one externally callable contract function: a `pub fn` declared
// inside an `impl` block annotated `#[contractimpl]`.
type ContractFn struct {
	// Name is the function's identifier.
	Name string

	// Node is the function_item node.
	Node Node

	// Body is the function's block. It is always valid for a function found
	// by [Context.ContractFns], which skips bodiless signatures.
	Body Node
}

// ContractFns returns the contract's externally callable functions, computed
// on first call and cached.
//
// This is the one piece of Soroban knowledge the engine hands to every rule.
// Almost every rule starts with "for each contract entry point ...", and
// getting that set right is fiddlier than it looks — in Rust's grammar an
// `#[contractimpl]` attribute is a preceding sibling of the impl block, not a
// child of it. Computing it once here keeps that subtlety in a single tested
// place instead of copy-pasted into every rule.
func (c *Context) ContractFns() []ContractFn {
	if !c.contractFnsSet {
		c.contractFns = findContractFns(c.Root)
		c.contractFnsSet = true
	}
	return c.contractFns
}

// Rule is a single lint check.
//
// Two methods, deliberately:
//
//   - Meta is static and cheap, so the CLI can list and document rules
//     without parsing anything.
//   - Check receives one already-parsed file and returns what it found.
//
// Check does no I/O, holds no state between files, and knows nothing about
// other rules. That is what lets rules run in any order, be tested one at a
// time against a fixture, and arrive as self-contained contributor pull
// requests. It returns no error because a rule inspecting an in-memory tree
// has nothing to fail at; parse and I/O failures are the engine's problem.
type Rule interface {
	Meta() Meta
	Check(*Context) []Finding
}
