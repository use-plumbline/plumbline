package rule

// MethodCall describes a call of the form `receiver.name(args)`.
type MethodCall struct {
	// Name is the method identifier, such as "require_auth".
	Name string

	// Recv is the receiver expression the method was called on.
	Recv Node

	// Field is the method name node. Report findings here rather than at
	// Node: in a chain like env.storage().persistent().set(k, v) the call
	// expression starts back at `env`, so an annotation anchored to Node
	// lands several lines above the call it is talking about.
	Field Node

	// Node is the call_expression itself.
	Node Node
}

// AsMethodCall reports whether n is a method call, and describes it.
func AsMethodCall(n Node) (MethodCall, bool) {
	if n.Kind() != "call_expression" {
		return MethodCall{}, false
	}
	fn, ok := n.Field("function")
	if !ok || fn.Kind() != "field_expression" {
		return MethodCall{}, false
	}
	field, ok := fn.Field("field")
	if !ok || field.Kind() != "field_identifier" {
		return MethodCall{}, false
	}
	recv, _ := fn.Field("value")
	return MethodCall{Name: field.Text(), Recv: recv, Field: field, Node: n}, true
}

// AsPlainCall returns the name of a free-function call — `name(args)`, or the
// final segment of `Path::name(args)`. It is how a rule spots a call to a
// helper declared alongside the contract.
func AsPlainCall(n Node) (string, bool) {
	if n.Kind() != "call_expression" {
		return "", false
	}
	fn, ok := n.Field("function")
	if !ok {
		return "", false
	}
	switch fn.Kind() {
	case "identifier":
		return fn.Text(), true
	case "scoped_identifier":
		name, ok := fn.Field("name")
		if !ok {
			return "", false
		}
		return name.Text(), true
	case "generic_function":
		inner, ok := fn.Field("function")
		if !ok {
			return "", false
		}
		return lastSegment(inner.Text()), true
	}
	return "", false
}

// MacroName returns the name of a macro invocation, such as "panic" for
// `panic!(...)`.
func MacroName(n Node) (string, bool) {
	if n.Kind() != "macro_invocation" {
		return "", false
	}
	m, ok := n.Field("macro")
	if !ok {
		return "", false
	}
	return lastSegment(m.Text()), true
}

// LocalFns maps every function declared anywhere in the file to its body,
// keyed by name. Rules use it to follow a call into a helper declared beside
// the contract, which is where idiomatic Soroban code keeps shared checks.
func LocalFns(root Node) map[string]Node {
	out := map[string]Node{}
	root.Walk(func(n Node) bool {
		if n.Kind() != "function_item" {
			return true
		}
		name, hasName := n.Field("name")
		body, hasBody := n.Field("body")
		if hasName && hasBody {
			out[name.Text()] = body
		}
		return true
	})
	return out
}
