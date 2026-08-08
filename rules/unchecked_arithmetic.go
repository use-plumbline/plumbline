package rules

import (
	"strings"

	ts "github.com/tree-sitter/go-tree-sitter"

	"github.com/use-plumbline/plumbline/internal/rule"
)

// overflowOps are the operators that can carry a value past the bounds of its
// type. Division is excluded: it cannot overflow except for MIN / -1, and
// flagging every division to catch that would drown the useful findings.
// Shifts are not covered yet.
var overflowOps = map[string]string{
	"+": "add", "-": "sub", "*": "mul",
	"+=": "add", "-=": "sub", "*=": "mul",
}

// intWidth is what a rule could work out about the integer type of an
// expression.
//
// The values are ordered by how much they say, so combining the two sides of
// an operator is a max. Rust requires both operands of `+` to have the same
// type, so resolving either side resolves the expression — which is what lets
// `count + 1` be recognised as u32 arithmetic from `count` alone.
type intWidth int

const (
	widthNeutral intWidth = iota // an unsuffixed literal: says nothing
	widthUnknown                 // could not be resolved
	widthNarrow                  // 64 bits or fewer
	widthWide                    // i128 / u128
)

func maxWidth(a, b intWidth) intWidth {
	if a > b {
		return a
	}
	return b
}

// narrowTypes are the integer types this rule leaves alone.
var narrowTypes = map[string]bool{
	"i8": true, "i16": true, "i32": true, "i64": true, "isize": true,
	"u8": true, "u16": true, "u32": true, "u64": true, "usize": true,
}

// UncheckedArithmetic reports unchecked 128-bit arithmetic in contract
// entry points.
type UncheckedArithmetic struct{}

func (UncheckedArithmetic) Meta() rule.Meta {
	return rule.Meta{
		ID:       "unchecked-arithmetic",
		Severity: rule.SeverityWarning,
		Summary:  "A contract function does arithmetic on a token-sized integer without checked or saturating handling.",
		Why: "i128 is the canonical token amount type in Soroban, so this is the " +
			"arithmetic that moves money. A release profile with overflow-checks = true " +
			"turns an overflow into a panic rather than a wrapped balance, but that is a " +
			"build setting a contract can lose by editing Cargo.toml, and the resulting " +
			"panic is an opaque host error either way.",
		Fix: "Use checked_add / checked_sub / checked_mul and handle the None case with a " +
			"contract error, or saturating_* where clamping is the intended behaviour. " +
			"Validate that i128 amounts are non-negative on the way in: i128 admits " +
			"negatives, and transfer(from, to, -1000) is a withdrawal from `to`.",
	}
}

func (UncheckedArithmetic) Check(c *rule.Context) []rule.Finding {
	var out []rule.Finding
	for _, fn := range c.ContractFns() {
		scope := fnScope(fn, c.Src)
		rule.Walk(fn.Body, func(n *ts.Node) bool {
			switch n.Kind() {
			case "binary_expression", "compound_assignment_expr":
			default:
				return true
			}
			op := n.ChildByFieldName("operator")
			left := n.ChildByFieldName("left")
			right := n.ChildByFieldName("right")
			if op == nil || left == nil || right == nil {
				return true
			}
			method, overflows := overflowOps[op.Utf8Text(c.Src)]
			if !overflows {
				return true
			}
			switch maxWidth(exprWidth(left, c.Src, scope), exprWidth(right, c.Src, scope)) {
			case widthWide, widthUnknown:
				out = append(out, rule.At(n,
					"%s: %q is unchecked arithmetic on a token-sized integer; use checked_%s or saturating_%s",
					fn.Name, strings.Join(strings.Fields(c.Text(n)), " "), method, method))
			}
			return true
		})
	}
	return out
}

// fnScope maps the identifiers in scope for a function to what is known about
// their integer width.
//
// It reads parameter types, then walks let bindings in source order so that a
// binding can be resolved from ones declared above it. Shadowing in nested
// blocks is not modelled; a shadowed name takes the width of its last binding,
// which errs toward reporting.
func fnScope(fn rule.ContractFn, src []byte) map[string]intWidth {
	scope := map[string]intWidth{}

	if params := fn.Node.ChildByFieldName("parameters"); params != nil {
		for i := uint(0); i < params.NamedChildCount(); i++ {
			p := params.NamedChild(i)
			if p.Kind() != "parameter" {
				continue
			}
			name, typ := p.ChildByFieldName("pattern"), p.ChildByFieldName("type")
			if name != nil && typ != nil && name.Kind() == "identifier" {
				scope[name.Utf8Text(src)] = typeWidth(typ.Utf8Text(src))
			}
		}
	}

	var lets []*ts.Node
	rule.Walk(fn.Body, func(n *ts.Node) bool {
		if n.Kind() == "let_declaration" {
			lets = append(lets, n)
		}
		return true
	})
	for _, l := range lets {
		name := l.ChildByFieldName("pattern")
		if name == nil || name.Kind() != "identifier" {
			continue
		}
		if typ := l.ChildByFieldName("type"); typ != nil {
			scope[name.Utf8Text(src)] = typeWidth(typ.Utf8Text(src))
			continue
		}
		if value := l.ChildByFieldName("value"); value != nil {
			scope[name.Utf8Text(src)] = exprWidth(value, src, scope)
		}
	}
	return scope
}

// exprWidth works out what is known about the integer width of an expression.
func exprWidth(n *ts.Node, src []byte, scope map[string]intWidth) intWidth {
	switch n.Kind() {
	case "identifier":
		if w, ok := scope[n.Utf8Text(src)]; ok {
			return w
		}
		return widthUnknown

	case "integer_literal":
		return literalWidth(n.Utf8Text(src))

	case "binary_expression":
		left, right := n.ChildByFieldName("left"), n.ChildByFieldName("right")
		if left == nil || right == nil {
			return widthUnknown
		}
		return maxWidth(exprWidth(left, src, scope), exprWidth(right, src, scope))

	case "type_cast_expression":
		if typ := n.ChildByFieldName("type"); typ != nil {
			return typeWidth(typ.Utf8Text(src))
		}
		return widthUnknown

	case "parenthesized_expression", "unary_expression", "reference_expression":
		if inner := n.NamedChild(0); inner != nil {
			return exprWidth(inner, src, scope)
		}
		return widthUnknown
	}
	// Calls, field accesses, indexes and macros are opaque without type
	// resolution, so they stay unknown.
	return widthUnknown
}

// typeWidth classifies a written-out type.
func typeWidth(name string) intWidth {
	switch {
	case name == "i128" || name == "u128":
		return widthWide
	case narrowTypes[name]:
		return widthNarrow
	}
	return widthUnknown
}

// literalWidth reads an integer literal's type suffix. A literal without one
// carries no type information, which is what keeps a constant expression such
// as 120 * 17280 from being reported.
func literalWidth(text string) intWidth {
	text = strings.ReplaceAll(text, "_", "")
	if strings.HasSuffix(text, "i128") || strings.HasSuffix(text, "u128") {
		return widthWide
	}
	for suffix := range narrowTypes {
		if strings.HasSuffix(text, suffix) {
			return widthNarrow
		}
	}
	return widthNeutral
}
