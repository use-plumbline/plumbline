package rules

import (
	ts "github.com/tree-sitter/go-tree-sitter"

	"github.com/use-plumbline/plumbline/internal/rule"
)

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
// authorization check. Three frames covers the idiomatic
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
	locals := rule.LocalFns(c.Root, c.Src)

	var out []rule.Finding
	for _, fn := range fns {
		// __constructor runs once, atomically, at deploy time. Whoever
		// deploys the contract already controls that invocation, so the
		// canonical constructor sets the admin with no auth check and is
		// not a finding.
		if fn.Name == "__constructor" {
			continue
		}
		write := findStorageMutation(fn.Body, c.Src)
		if write == nil {
			continue
		}
		if hasAuth(fn.Body, c.Src, locals, map[string]bool{fn.Name: true}, maxAuthDepth) {
			continue
		}
		name := fn.Node.ChildByFieldName("name")
		out = append(out, rule.At(name,
			"%s writes storage but no path through it calls require_auth (write at line %d)",
			fn.Name, int(write.StartPosition().Row)+1))
	}
	return out
}

// findStorageMutation returns the first storage-mutating call in body, or nil.
func findStorageMutation(body *ts.Node, src []byte) *ts.Node {
	var found *ts.Node
	rule.Walk(body, func(n *ts.Node) bool {
		if found != nil {
			return false
		}
		call, ok := rule.AsMethodCall(n, src)
		if ok && storageMutators[call.Name] && receiverChainHas(call.Recv, src, "storage") {
			found = call.Node
			return false
		}
		return true
	})
	return found
}

// receiverChainHas reports whether name appears as a method in the receiver
// chain of an expression. For `env.storage().persistent().set(k, v)` the
// receiver of set is `env.storage().persistent()`, and walking down it finds
// "persistent" then "storage" — which is what separates a storage write from
// any other .set() in the file.
func receiverChainHas(recv *ts.Node, src []byte, name string) bool {
	for n := recv; n != nil; {
		call, ok := rule.AsMethodCall(n, src)
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
func hasAuth(body *ts.Node, src []byte, locals map[string]*ts.Node, seen map[string]bool, depth int) bool {
	if depth <= 0 {
		return false
	}
	found := false
	rule.Walk(body, func(n *ts.Node) bool {
		if found {
			return false
		}
		if call, ok := rule.AsMethodCall(n, src); ok && authCalls[call.Name] {
			found = true
			return false
		}
		if callee, ok := rule.AsPlainCall(n, src); ok && !seen[callee] {
			if next, isLocal := locals[callee]; isLocal {
				seen[callee] = true
				if hasAuth(next, src, locals, seen, depth-1) {
					found = true
					return false
				}
			}
		}
		return true
	})
	return found
}
