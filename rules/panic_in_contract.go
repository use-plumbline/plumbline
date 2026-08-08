package rules

import "github.com/use-plumbline/plumbline/internal/rule"

// panicMethods abort the invocation with an opaque host error when the value
// is absent or an error.
//
// Matching is exact, which is what keeps unwrap_or, unwrap_or_else and
// unwrap_or_default out of the results — those are the recommended way to
// handle a missing storage entry, not a defect.
var panicMethods = map[string]bool{
	"unwrap": true,
	"expect": true,
}

// PanicInContract reports panics in a contract entry point.
type PanicInContract struct{}

func (PanicInContract) Meta() rule.Meta {
	return rule.Meta{
		ID:       "panic-in-contract",
		Severity: rule.SeverityWarning,
		Summary:  "A contract function panics instead of returning a contract error.",
		Why: "A bare panic surfaces to the caller as an opaque host error with no code " +
			"attached, so clients cannot match on it, distinguish it from a budget or " +
			"auth failure, or show a useful message. It also rolls back every state " +
			"change in the invocation, including nested calls, with no explanation of why.",
		Fix: "Return Result<T, Error> with a #[contracterror] enum so callers get a typed " +
			"error through try_ client methods, or use panic_with_error!(&env, Error::X) " +
			"when the path really must abort. For a possibly-absent storage entry, " +
			"unwrap_or / unwrap_or_default / ok_or are the intended tools.",
	}
}

func (PanicInContract) Check(c *rule.Context) []rule.Finding {
	var out []rule.Finding
	// Only entry-point bodies are scanned, so #[cfg(test)] modules and
	// non-contract impl blocks are excluded by construction. Panics inside
	// helper functions called from an entry point are not yet reported.
	for _, fn := range c.ContractFns() {
		fn.Body.Walk(func(n rule.Node) bool {
			if f, ok := panicFinding(n, fn.Name); ok {
				out = append(out, f)
			}
			return true
		})
	}
	return out
}

// panicFinding reports n if it is a panicking construct.
func panicFinding(n rule.Node, fnName string) (rule.Finding, bool) {
	if name, ok := rule.MacroName(n); ok {
		// panic_with_error! is the correct way to abort: it carries a
		// contract error code the caller can match on. Only bare panic!
		// is a finding.
		if name == "panic" {
			return rule.At(n, "%s calls panic!, which reaches the caller as an "+
				"opaque host error with no contract error code", fnName), true
		}
		return rule.Finding{}, false
	}
	if call, ok := rule.AsMethodCall(n); ok && panicMethods[call.Name] {
		return rule.At(call.Field, "%s calls %s(), which aborts with an opaque host "+
			"error when the value is absent", fnName, call.Name), true
	}
	return rule.Finding{}, false
}
