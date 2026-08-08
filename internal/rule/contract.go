package rule

import (
	"strings"

	ts "github.com/tree-sitter/go-tree-sitter"
)

// contractImplAttr marks an impl block whose public functions become the
// contract's callable interface. Verified against soroban-sdk 27.
const contractImplAttr = "contractimpl"

// findContractFns collects the `pub fn` items of every `#[contractimpl] impl`
// block in the file. It descends into modules, since a contract is often
// declared inside `mod` rather than at the top level.
func findContractFns(root *ts.Node, src []byte) []ContractFn {
	var out []ContractFn
	Walk(root, func(item *ts.Node) bool {
		if item.Kind() != "impl_item" || !HasAttribute(item, src, contractImplAttr) {
			return true
		}
		body := item.ChildByFieldName("body")
		if body == nil {
			return false
		}
		for j := uint(0); j < body.NamedChildCount(); j++ {
			fn := body.NamedChild(j)
			if fn.Kind() != "function_item" || !isPub(fn) {
				continue
			}
			name := fn.ChildByFieldName("name")
			block := fn.ChildByFieldName("body")
			if name == nil || block == nil {
				continue
			}
			out = append(out, ContractFn{
				Name: name.Utf8Text(src),
				Node: fn,
				Body: block,
			})
		}
		return false // no contract impls nested inside a contract impl
	})
	return out
}

// Walk visits n and its named descendants in source order. Returning false
// from fn skips that node's children.
//
// This is the only traversal helper the engine provides. Rules that need a
// different shape of walk are free to write it inline — a rule is easier to
// read as one self-contained file than as a call into a traversal toolkit.
func Walk(n *ts.Node, fn func(*ts.Node) bool) {
	if !fn(n) {
		return
	}
	for i := uint(0); i < n.NamedChildCount(); i++ {
		Walk(n.NamedChild(i), fn)
	}
}

// HasAttribute reports whether item carries the named outer attribute, e.g.
// HasAttribute(n, src, "contractimpl") for `#[contractimpl]`.
//
// Attributes are preceding siblings of the item they decorate in tree-sitter's
// Rust grammar, and they stack, so this walks backwards over the run of
// attribute_item nodes immediately before item. A path-qualified attribute such
// as `#[soroban_sdk::contractimpl]` matches on its final segment.
func HasAttribute(item *ts.Node, src []byte, name string) bool {
	for prev := item.PrevNamedSibling(); prev != nil; prev = prev.PrevNamedSibling() {
		if prev.Kind() != "attribute_item" {
			return false
		}
		if attributeName(prev, src) == name {
			return true
		}
	}
	return false
}

// attributeName returns the final path segment of an attribute_item's name,
// or "" if it cannot be read.
func attributeName(attrItem *ts.Node, src []byte) string {
	attr := attrItem.NamedChild(0)
	if attr == nil || attr.Kind() != "attribute" {
		return ""
	}
	path := attr.NamedChild(0)
	if path == nil {
		return ""
	}
	text := path.Utf8Text(src)
	if i := strings.LastIndex(text, "::"); i >= 0 {
		text = text[i+2:]
	}
	return strings.TrimSpace(text)
}

// isPub reports whether a function_item is declared `pub`.
func isPub(fn *ts.Node) bool {
	for i := uint(0); i < fn.NamedChildCount(); i++ {
		if fn.NamedChild(i).Kind() == "visibility_modifier" {
			return true
		}
	}
	return false
}
