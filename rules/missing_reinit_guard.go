package rules

import (
	"strings"

	"github.com/use-plumbline/plumbline/internal/rule"
)

// MissingReinitGuard reports contract entry points that can rewrite state during
// initialization without first refusing a second invocation.
type MissingReinitGuard struct{}

func (MissingReinitGuard) Meta() rule.Meta {
	return rule.Meta{
		ID:       "missing-reinit-guard",
		Severity: rule.SeverityError,
		Summary:  "An initializer or privileged storage write has no reinitialization guard.",
		Why:      "An initializer that can run twice may let a caller replace an admin, owner, or configuration value after deployment. The contract must reject a second call before mutating state.",
		Fix:      "At the start of the initializer, check the instance storage key with has(...) or get(...).is_some() and return or panic on the already-initialized path before writing the key.",
	}
}

func (MissingReinitGuard) Check(c *rule.Context) []rule.Finding {
	var out []rule.Finding
	for _, fn := range c.ContractFns() {
		if fn.Name == "__constructor" || reinitHasOneShotGuard(fn.Body) {
			continue
		}
		if !writesStorage(fn.Body) {
			continue
		}
		if initializerName(fn.Name) || writesPrivilegedKey(fn.Body) {
			name, _ := fn.Node.Field("name")
			out = append(out, rule.At(name, "%s can mutate initialization state without a one-shot has/get guard", fn.Name))
		}
	}
	return out
}

func initializerName(name string) bool {
	switch name {
	case "initialize", "init", "setup":
		return true
	default:
		return false
	}
}

var reinitStorageMutators = map[string]bool{
	"set": true, "remove": true, "update": true, "try_update": true,
}

func writesStorage(body rule.Node) bool {
	found := false
	body.Walk(func(n rule.Node) bool {
		if call, ok := rule.AsMethodCall(n); ok && reinitStorageMutators[call.Name] && receiverHasStorage(call.Recv) {
			found = true
			return false
		}
		return !found
	})
	return found
}

func writesPrivilegedKey(body rule.Node) bool {
	found := false
	body.Walk(func(n rule.Node) bool {
		if call, ok := rule.AsMethodCall(n); ok && reinitStorageMutators[call.Name] && receiverHasStorage(call.Recv) {
			// The parser's source text is intentionally used here: key enums may
			// be wrapped in constructors or qualified paths, but Admin/Owner/
			// Config remain explicit source-level markers.
			text := call.Node.Text()
			if strings.Contains(text, "Admin") || strings.Contains(text, "Owner") || strings.Contains(text, "Config") {
				found = true
				return false
			}
		}
		return !found
	})
	return found
}

func receiverHasStorage(recv rule.Node) bool {
	for n := recv; n.Valid(); {
		call, ok := rule.AsMethodCall(n)
		if !ok {
			return false
		}
		if call.Name == "storage" {
			return true
		}
		n = call.Recv
	}
	return false
}

// hasOneShotGuard recognizes the canonical early-exit guard for an existing
// instance value. It deliberately requires the condition itself to be a has
// call or a get(...).is_some() chain, avoiding the opposite "not initialized"
// check and unrelated state predicates.
func reinitHasOneShotGuard(body rule.Node) bool {
	found := false
	body.Walk(func(n rule.Node) bool {
		if found || n.Kind() != "if_expression" {
			return !found
		}
		condition, ok := n.Field("condition")
		consequence, hasConsequence := n.Field("consequence")
		if !ok || !hasConsequence || !guardCondition(condition) {
			return true
		}
		if reinitExitsEarly(consequence) {
			found = true
			return false
		}
		return true
	})
	return found
}

func guardCondition(condition rule.Node) bool {
	text := condition.Text()
	if strings.Contains(text, "!") {
		return false
	}
	if strings.Contains(text, ".get(") && strings.Contains(text, ".is_some(") && strings.Contains(text, "storage") {
		return true
	}
	if call, ok := rule.AsMethodCall(condition); ok {
		return call.Name == "has" && receiverHasStorage(call.Recv)
	}
	if condition.Kind() != "call_expression" {
		return false
	}
	fn, ok := condition.Field("function")
	if !ok {
		return false
	}
	call, ok := rule.AsMethodCall(fn)
	if ok && call.Name == "is_some" && receiverHasStorage(call.Recv) {
		return true
	}
	return false
}

func reinitExitsEarly(block rule.Node) bool {
	text := block.Text()
	if strings.Contains(text, "return") || strings.Contains(text, "panic!(") || strings.Contains(text, "panic_with_error!(") {
		return true
	}
	found := false
	block.Walk(func(n rule.Node) bool {
		if n.Kind() == "return_expression" {
			found = true
			return false
		}
		if name, ok := rule.MacroName(n); ok && (name == "panic" || name == "panic_with_error") {
			found = true
			return false
		}
		return !found
	})
	return found
}
