package rules

import "github.com/use-plumbline/plumbline/internal/rule"

// MissingReinitGuard reports contract entry points that look like initializers
// and can rewrite state without first refusing a second invocation.
//
// The rule is deliberately narrow: it fires only on functions whose name
// suggests they run once at setup (initialize, init, setup) and that write
// storage without a has/get guard. A function that writes to a privileged
// key but is not named like an initializer is a legitimate setter — its
// real vulnerability (missing auth) is the job of missing-auth, not this
// rule.
//
// Reverted once (PR #28, revert #30) after writesPrivilegedKey triggered
// on regular setters like set_admin and update_config. That heuristic
// checked whether a .set() call's source text contained "Admin", "Owner",
// or "Config" — a substring match on a key enum variant, not evidence
// the function is an initializer. A function named set_admin that writes
// Admin storage is a setter, not a setup function, and flagging it as
// missing a reinit guard is a false positive.
type MissingReinitGuard struct{}

func (MissingReinitGuard) Meta() rule.Meta {
	return rule.Meta{
		ID:       "missing-reinit-guard",
		Severity: rule.SeverityWarning,
		Summary:  "An initializer has no reinitialization guard.",
		Why: "An initializer that can run twice may let a caller replace " +
			"an admin, owner, or configuration value after deployment. " +
			"The contract must reject a second call before mutating state.",
		Fix: "At the start of the initializer, check the instance storage " +
			"key with has(...) or get(...).is_some() and return or panic " +
			"on the already-initialized path before writing the key.",
	}
}

func (MissingReinitGuard) Check(c *rule.Context) []rule.Finding {
	fns := c.ContractFns()
	if len(fns) == 0 {
		return nil
	}
	locals := rule.LocalFns(c.Root)

	var out []rule.Finding
	for _, fn := range fns {
		if fn.Name == "__constructor" || hasReinitGuard(fn.Body, locals) {
			continue
		}
		if !initializerName(fn.Name) || !writesStorage(fn.Body) {
			continue
		}
		name, _ := fn.Node.Field("name")
		out = append(out, rule.At(name,
			"%s is an initializer that can mutate state without a one-shot has/get guard",
			fn.Name))
	}
	return out
}

// initializerName reports whether the function name suggests it runs once
// at setup time. These are the conventional Soroban initializer names,
// verified against soroban-examples and stellar-contracts: initialize, init,
// and setup are all used as one-shot setup functions in the ecosystem.
func initializerName(name string) bool {
	switch name {
	case "initialize", "init", "setup":
		return true
	default:
		return false
	}
}

var reinitStorageMutators = map[string]bool{
	"set":        true,
	"remove":     true,
	"update":     true,
	"try_update": true,
}

// writesStorage reports whether body contains a storage-mutating call.
func writesStorage(body rule.Node) bool {
	found := false

	body.Walk(func(n rule.Node) bool {
		if found {
			return false
		}
		call, ok := rule.AsMethodCall(n)
		if ok && reinitStorageMutators[call.Name] && receiverHasStorage(call.Recv) {
			found = true
			return false
		}
		return true
	})

	return found
}

// receiverHasStorage reports whether a method call's receiver chain reaches a
// storage accessor.
//
// One implementation, in receiverChainHas. There were two, and they disagreed:
// this one handled turbofish calls and the other did not, so the same chain
// resolved differently depending on which rule was asking.
func receiverHasStorage(recv rule.Node) bool {
	return receiverChainHas(recv, "storage")
}

// hasReinitGuard recognizes the canonical early-exit guard for an existing
// instance value. It requires the condition to be a has call or a
// get(...).is_some() chain on storage, and the consequence to exit early.
// This avoids the opposite "not initialized" check and unrelated state
// predicates.
func hasReinitGuard(body rule.Node, locals map[string]rule.Node) bool {
	bindings := letBindings(body)
	found := false

	body.Walk(func(n rule.Node) bool {
		if found || n.Kind() != "if_expression" {
			return !found
		}

		condition, ok := n.Field("condition")
		consequence, hasConsequence := n.Field("consequence")
		if !ok || !hasConsequence || !guardCondition(condition, locals, bindings) {
			return true
		}
		if reinitGuardExitsEarly(consequence) {
			found = true
			return false
		}

		return true
	})

	return found
}

// guardCondition reports whether condition matches the canonical storage
// guard shapes: has(&key), get(&key).is_some(), or !get(&key).is_none().
// All three express the same intent: "if already initialized, exit early."
func guardCondition(condition rule.Node, locals map[string]rule.Node, bindings map[string]rule.Node) bool {
	// has(&DataKey::X) — the most common shape
	if call, ok := rule.AsMethodCall(condition); ok && call.Name == "has" && guardReceiver(call.Recv, locals, bindings) {
		return true
	}
	// get(&DataKey::X).is_some(), or a helper's Result via .is_ok().
	if call, ok := rule.AsMethodCall(condition); ok &&
		guardPredicates[call.Name] && guardReceiver(call.Recv, locals, bindings) {
		return true
	}
	// !get(&DataKey::X).is_none() — the negated alternative, seen in
	// scout-soroban-examples/amm. Semantically equivalent to is_some().
	if condition.Kind() == "unary_expression" {
		inner, ok := condition.Child(0)
		if ok {
			if call, ok := rule.AsMethodCall(inner); ok && call.Name == "is_none" && guardReceiver(call.Recv, locals, bindings) {
				return true
			}
		}
	}
	return false
}

// guardPredicates are the ways a contract asks "is the value already there?".
//
// is_ok belongs with is_some because the idiom that motivated it wraps the
// storage read in a helper returning Result — Ok when the key is present, Err
// when it is not — so `helper().is_ok()` is the same question as
// `get(&k).is_some()`.
var guardPredicates = map[string]bool{
	"is_some": true, "is_ok": true,
}

// guardReceiver reports whether recv reads storage, either directly or through
// a function declared in the same file.
//
// Following the call is what stops this rule from reporting a contract that is
// genuinely guarded. Both governance and payment-channel in
// scout-soroban-examples guard their initializers with
//
//	if Self::get_state(env).is_ok() { return Err(AlreadyInitialized) }
//
// where get_state reads instance storage and maps absence to Err. Without the
// hop the rule reports them, and CoinFabrik's review of that same codebase
// records the corresponding issue (IS-17, unrestricted initialize) as
// Resolved — so the rule would have been contradicting a professional audit
// that had already been satisfied. See docs/corpus-run.md.
//
// The hop is one frame deep and does not recurse. A guard that needs two hops
// to find its storage read is far enough from the idiom that a human should
// look at it.
func guardReceiver(recv rule.Node, locals map[string]rule.Node, bindings map[string]rule.Node) bool {
	if receiverHasStorage(recv) {
		return true
	}
	// The receiver is often a local holding the helper's result rather than
	// the call itself — `let state = Self::get_state(env); if state.is_ok()`
	// is the shape both scout-soroban-examples contracts use. Resolve one
	// binding before giving up.
	if recv.Kind() == "identifier" {
		if bound, ok := bindings[recv.Text()]; ok {
			recv = bound
		}
	}
	callee, ok := rule.AsPlainCall(recv)
	if !ok {
		return false
	}
	body, isLocal := locals[callee]
	if !isLocal {
		return false
	}
	return readsStorage(body)
}

// letBindings maps each simple `let name = value` in body to its value.
//
// Only the last binding of a name is kept. Shadowing a guard's variable
// between the binding and the check would be perverse, and modelling scopes
// properly is more machinery than this rule earns.
func letBindings(body rule.Node) map[string]rule.Node {
	out := map[string]rule.Node{}
	body.Walk(func(n rule.Node) bool {
		if n.Kind() != "let_declaration" {
			return true
		}
		name, hasName := n.Field("pattern")
		value, hasValue := n.Field("value")
		if hasName && hasValue && name.Kind() == "identifier" {
			out[name.Text()] = value
		}
		return true
	})
	return out
}

// readsStorage reports whether body reads a storage entry — the thing a
// one-shot guard's helper must do for the guard to mean anything.
func readsStorage(body rule.Node) bool {
	found := false
	body.Walk(func(n rule.Node) bool {
		if found {
			return false
		}
		if call, ok := rule.AsMethodCall(n); ok &&
			(call.Name == "get" || call.Name == "has") && receiverHasStorage(call.Recv) {
			found = true
			return false
		}
		return true
	})
	return found
}

// reinitGuardExitsEarly reports whether block leaves the function without
// running the rest of it.
func reinitGuardExitsEarly(block rule.Node) bool {
	found := false

	block.Walk(func(n rule.Node) bool {
		if found {
			return false
		}
		if n.Kind() == "return_expression" {
			found = true
			return false
		}

		if name, ok := rule.MacroName(n); ok &&
			(name == "panic" || name == "panic_with_error") {
			found = true
			return false
		}
		return true
	})

	return found
}
