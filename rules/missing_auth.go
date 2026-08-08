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
