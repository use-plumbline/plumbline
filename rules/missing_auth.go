package rules

import "github.com/use-plumbline/plumbline/internal/rule"

// storageMutators are the Storage methods that change ledger state, verified
// against soroban-sdk 27 (soroban_sdk::storage::{Instance,Persistent,Temporary}).
//
// extend_ttl and extend_ttl_with_limits are deliberately absent. Any account
// can extend any entry's TTL with ExtendFootprintTTLOp without the contract
// being involved at all, so a TTL bump is not authority-bearing state change
// and requiring auth for it would be a false positive.
var storageMutators = map[string]bool{
	"set":        true,
	"remove":     true,
	"update":     true,
	"try_update": true,
}

// authCalls are the only two ways a contract asks the host to verify that an
// address consented (soroban_sdk::Address).
var authCalls = map[string]bool{
	"require_auth":          true,
	"require_auth_for_args": true,
}

// authAttrs are attribute macros that expand into an authorization check
// before the function body runs, so a function carrying one is authorized even
// though no require_auth is visible in its source.
//
// Verified against docs.rs/stellar-macros 0.7.2, the proc-macro crate of
// OpenZeppelin's stellar-contracts: only_owner and only_admin "require
// authorization from the owner/admin before executing the function body", and
// only_role / only_any_role check the role "and require authorization".
//
// has_role and has_any_role are deliberately absent — they check that an
// address holds a role but do not require it to have authorized the call, so a
// function whose only gate is has_role really is missing an auth check.
// when_paused and when_not_paused are pause-state checks, not authorization.
var authAttrs = []string{
	"only_owner",
	"only_admin",
	"only_role",
	"only_any_role",
}

// maxAuthDepth bounds how far the search follows helper calls looking for an
// authorization check. Four frames covers the idiomatic
// `entry -> require_admin -> admin.require_auth()` shape with room to spare;
// anything deeper is worth a human reading it anyway.
const maxAuthDepth = 4

// MissingAuth reports contract entry points that write storage without
// requiring authorization.
type MissingAuth struct{}

func (MissingAuth) Meta() rule.Meta {
	return rule.Meta{
		ID:       "missing-auth",
		Severity: rule.SeverityError,
		Summary:  "A contract function writes storage without requiring authorization.",
		Why: "Authorization in Soroban is opt-in: nothing checks a caller's identity " +
			"unless the contract calls require_auth. A state-mutating entry point with " +
			"no auth check on any path is callable by anyone, so any account can move " +
			"balances, reassign an admin, or reconfigure the contract.",
		Fix: "Call require_auth() on the address whose consent the operation needs — " +
			"for privileged paths, load the admin from storage and authorize that, " +
			"not an address the caller supplied. Use require_auth_for_args when the " +
			"signed payload should cover only some of the arguments.",
	}
}

func (MissingAuth) Check(c *rule.Context) []rule.Finding {
	fns := c.ContractFns()
	if len(fns) == 0 {
		return nil
	}
	locals := rule.LocalFns(c.Root)

	var out []rule.Finding
	for _, fn := range fns {
		// __constructor runs once, atomically, at deploy time. Whoever
		// deploys the contract already controls that invocation, so the
		// canonical constructor sets the admin with no auth check and is
		// not a finding.
		if fn.Name == "__constructor" {
			continue
		}
		if hasAuthAttribute(fn.Node) {
			continue
		}
		// A one-shot initializer is the other shape that is authorized
		// without an auth check, for the same reason a constructor is:
		// it can only ever succeed once, so there is no ongoing authority
		// to protect.
		if hasOneShotGuard(fn.Body) {
			continue
		}
		write, writes := findStorageMutation(fn.Body)
		if !writes {
			continue
		}
		if hasAuth(fn.Body, locals, map[string]bool{fn.Name: true}, maxAuthDepth) {
			continue
		}
		name, _ := fn.Node.Field("name")
		out = append(out, rule.At(name,
			"%s writes storage but no path through it calls require_auth (write at line %d)",
			fn.Name, write.Line()))
	}
	return out
}

// hasAuthAttribute reports whether fn is guarded by an authorizing attribute
// macro. The check is on the attribute's name, so it holds whether the macro
// is written bare or path-qualified, and whether or not it takes arguments.
func hasAuthAttribute(fn rule.Node) bool {
	for _, attr := range authAttrs {
		if rule.HasAttribute(fn, attr) {
			return true
		}
	}
	return false
}

// hasOneShotGuard reports whether body opens with the canonical Soroban
// "initialize once" check:
//
//	if env.storage().instance().has(&DataKey::Admin) {
//	    return Err(Error::AlreadyInitialized);
//	}
//
// A function shaped like that cannot be called a second time, so it carries no
// standing authority for an auth check to protect — the same argument that
// exempts __constructor. Contracts that predate constructor support use it for
// exactly that job, and flagging it would fire on most of the ecosystem.
//
// The match is deliberately narrow. The condition must be the storage `has`
// call itself, so the opposite guard — `if !...has(&k) { return Err(NotInit) }`,
// which asserts the contract *is* initialized and authorizes nothing — is not
// mistaken for it.
func hasOneShotGuard(body rule.Node) bool {
	found := false
	body.Walk(func(n rule.Node) bool {
		if found || n.Kind() != "if_expression" {
			return !found
		}
		cond, hasCond := n.Field("condition")
		conseq, hasConseq := n.Field("consequence")
		if !hasCond || !hasConseq {
			return true
		}
		call, isCall := rule.AsMethodCall(cond)
		if !isCall || call.Name != "has" || !receiverChainHas(call.Recv, "storage") {
			return true
		}
		if exitsEarly(conseq) {
			found = true
			return false
		}
		return true
	})
	return found
}

// exitsEarly reports whether block leaves the function without running the
// rest of it.
func exitsEarly(block rule.Node) bool {
	found := false
	block.Walk(func(n rule.Node) bool {
		if found {
			return false
		}
		if n.Kind() == "return_expression" {
			found = true
			return false
		}
		if name, ok := rule.MacroName(n); ok && (name == "panic" || name == "panic_with_error") {
			found = true
			return false
		}
		return true
	})
	return found
}

// findStorageMutation returns the first storage-mutating call in body.
func findStorageMutation(body rule.Node) (rule.Node, bool) {
	var found rule.Node
	body.Walk(func(n rule.Node) bool {
		if found.Valid() {
			return false
		}
		call, ok := rule.AsMethodCall(n)
		if ok && storageMutators[call.Name] && receiverChainHas(call.Recv, "storage") {
			found = call.Field
			return false
		}
		return true
	})
	return found, found.Valid()
}

// receiverChainHas reports whether name appears as a method in the receiver
// chain of an expression. For `env.storage().persistent().set(k, v)` the
// receiver of set is `env.storage().persistent()`, and walking down it finds
// "persistent" then "storage" — which is what separates a storage write from
// any other .set() in the file.
func receiverChainHas(recv rule.Node, name string) bool {
	for n := recv; n.Valid(); {
		call, ok := rule.AsMethodCall(n)
		if !ok {
			return false
		}
		if call.Name == name {
			return true
		}
		n = call.Recv
	}
	return false
}

// hasAuth reports whether body, or any same-file function it calls, performs
// an authorization check.
//
// Following calls matters: the idiomatic Soroban shape puts the check in a
// helper (`fn require_admin(env: &Env) { ...; admin.require_auth(); }`), and a
// body-only search would flag every contract written that way.
func hasAuth(body rule.Node, locals map[string]rule.Node, seen map[string]bool, depth int) bool {
	if depth <= 0 {
		return false
	}
	found := false
	body.Walk(func(n rule.Node) bool {
		if found {
			return false
		}
		if call, ok := rule.AsMethodCall(n); ok && authCalls[call.Name] {
			found = true
			return false
		}
		if callee, ok := rule.AsPlainCall(n); ok && !seen[callee] {
			if next, isLocal := locals[callee]; isLocal {
				seen[callee] = true
				if hasAuth(next, locals, seen, depth-1) {
					found = true
					return false
				}
			}
		}
		return true
	})
	return found
}
