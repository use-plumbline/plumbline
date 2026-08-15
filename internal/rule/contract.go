package rule

import "strings"

// contractImplAttr marks an impl block whose public functions become the
// contract's callable interface. Verified against soroban-sdk 27.
const contractImplAttr = "contractimpl"

// findContractFns collects the `pub fn` items of every `#[contractimpl] impl`
// block in the file. It descends into modules, since a contract is often
// declared inside `mod` rather than at the top level.
func findContractFns(root Node) []ContractFn {
	var out []ContractFn
	root.Walk(func(item Node) bool {
		// A contract declared inside `#[cfg(test)] mod tests` is a mock
		// built to exercise one path from a test. It is not compiled into
		// the deployed wasm, so holding it to the rules that protect a
		// deployed contract reports defects that cannot reach a ledger.
		if item.Kind() == "mod_item" && isCfgTest(item) {
			return false
		}
		if item.Kind() != "impl_item" || !HasAttribute(item, contractImplAttr) {
			return true
		}
		body, ok := item.Field("body")
		if !ok {
			return false
		}
		for _, fn := range body.Children() {
			if fn.Kind() != "function_item" || !isPub(fn) {
				continue
			}
			name, hasName := fn.Field("name")
			block, hasBody := fn.Field("body")
			if !hasName || !hasBody {
				continue
			}
			out = append(out, ContractFn{Name: name.Text(), Node: fn, Body: block})
		}
		return false // no contract impls nested inside a contract impl
	})
	return out
}

// HasAttribute reports whether item carries the named outer attribute, as in
// HasAttribute(n, "contractimpl") for `#[contractimpl]`.
//
// Attributes are preceding siblings of the item they decorate in Rust's
// grammar, and they stack, so this walks backwards over the run of
// attribute_item nodes immediately before item. A path-qualified attribute
// such as `#[soroban_sdk::contractimpl]` matches on its final segment.
func HasAttribute(item Node, name string) bool {
	for prev, ok := item.PrevSibling(); ok; prev, ok = prev.PrevSibling() {
		if prev.Kind() != "attribute_item" {
			return false
		}
		if attributeName(prev) == name {
			return true
		}
	}
	return false
}

// isCfgTest reports whether item carries `#[cfg(test)]`.
//
// It matches the exact predicate rather than any `cfg`, so `#[cfg(feature =
// "x")]` — which does end up in the deployed wasm — is still linted.
func isCfgTest(item Node) bool {
	for prev, ok := item.PrevSibling(); ok; prev, ok = prev.PrevSibling() {
		if prev.Kind() != "attribute_item" {
			return false
		}
		if attr, ok := prev.Child(0); ok && strings.Join(strings.Fields(attr.Text()), "") == "cfg(test)" {
			return true
		}
	}
	return false
}

// attributeName returns the final path segment of an attribute_item's name,
// or "" if it cannot be read.
func attributeName(attrItem Node) string {
	attr, ok := attrItem.Child(0)
	if !ok || attr.Kind() != "attribute" {
		return ""
	}
	path, ok := attr.Child(0)
	if !ok {
		return ""
	}
	return lastSegment(path.Text())
}

// lastSegment drops any `a::b::` prefix from a path.
func lastSegment(text string) string {
	if i := strings.LastIndex(text, "::"); i >= 0 {
		text = text[i+2:]
	}
	return strings.TrimSpace(text)
}

// isPub reports whether a function_item is declared `pub`.
func isPub(fn Node) bool {
	for _, c := range fn.Children() {
		if c.Kind() == "visibility_modifier" {
			return true
		}
	}
	return false
}
