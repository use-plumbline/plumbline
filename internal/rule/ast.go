package rule

import (
	"strings"

	ts "github.com/tree-sitter/go-tree-sitter"
)

// MethodCall describes a call of the form `receiver.name(args)`.
type MethodCall struct {
	// Name is the method identifier, e.g. "require_auth".
	Name string
	// Recv is the receiver expression the method was called on.
	Recv *ts.Node
	// Field is the method name node. Report findings here rather than at
	// Node: in a chain like env.storage().persistent().set(k, v) the call
	// expression starts back at `env`, so an annotation anchored to Node
	// lands several lines above the call it is talking about.
	Field *ts.Node
	// Node is the call_expression itself.
	Node *ts.Node
}

// AsMethodCall reports whether n is a method call, and describes it.
func AsMethodCall(n *ts.Node, src []byte) (MethodCall, bool) {
	if n.Kind() != "call_expression" {
		return MethodCall{}, false
	}
	fn := n.ChildByFieldName("function")
	if fn == nil || fn.Kind() != "field_expression" {
		return MethodCall{}, false
	}
	field := fn.ChildByFieldName("field")
	if field == nil || field.Kind() != "field_identifier" {
		return MethodCall{}, false
	}
	return MethodCall{
		Name:  field.Utf8Text(src),
		Recv:  fn.ChildByFieldName("value"),
		Field: field,
		Node:  n,
	}, true
}

// AsPlainCall returns the name of a free-function call — `name(args)` or the
// final segment of `Path::name(args)`. It is how a rule spots a call to a
// helper declared alongside the contract.
func AsPlainCall(n *ts.Node, src []byte) (string, bool) {
	if n.Kind() != "call_expression" {
		return "", false
	}
	fn := n.ChildByFieldName("function")
	if fn == nil {
		return "", false
	}
	switch fn.Kind() {
	case "identifier":
		return fn.Utf8Text(src), true
	case "scoped_identifier":
		name := fn.ChildByFieldName("name")
		if name == nil {
			return "", false
		}
		return name.Utf8Text(src), true
	case "generic_function":
		inner := fn.ChildByFieldName("function")
		if inner == nil {
			return "", false
		}
		text := inner.Utf8Text(src)
		if i := strings.LastIndex(text, "::"); i >= 0 {
			text = text[i+2:]
		}
		return text, true
	}
	return "", false
}

// MacroName returns the name of a macro invocation, e.g. "panic" for `panic!()`.
func MacroName(n *ts.Node, src []byte) (string, bool) {
	if n.Kind() != "macro_invocation" {
		return "", false
	}
	m := n.ChildByFieldName("macro")
	if m == nil {
		return "", false
	}
	text := m.Utf8Text(src)
	if i := strings.LastIndex(text, "::"); i >= 0 {
		text = text[i+2:]
	}
	return text, true
}

// LocalFns maps every function declared anywhere in the file to its body,
// keyed by name. Rules use it to follow a call into a helper declared beside
// the contract, which is where idiomatic Soroban code keeps shared checks.
func LocalFns(root *ts.Node, src []byte) map[string]*ts.Node {
	out := map[string]*ts.Node{}
	Walk(root, func(n *ts.Node) bool {
		if n.Kind() != "function_item" {
			return true
		}
		name := n.ChildByFieldName("name")
		body := n.ChildByFieldName("body")
		if name != nil && body != nil {
			out[name.Utf8Text(src)] = body
		}
		return true
	})
	return out
}
