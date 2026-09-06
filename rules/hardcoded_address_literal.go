package rules

import (
	"strings"

	"github.com/use-plumbline/plumbline/internal/rule"
)

const (
	standardStrkeyLength = 56
	muxedStrkeyLength    = 69
)

// HardcodedAddressLiteral reports deploy-time Stellar addresses embedded in a
// contract source file.
type HardcodedAddressLiteral struct{}

func (HardcodedAddressLiteral) Meta() rule.Meta {
	return rule.Meta{
		ID:       "hardcoded-address-literal",
		Severity: rule.SeverityWarning,
		Summary:  "A Stellar address is hardcoded in contract source.",
		Why: "A hardcoded address is fixed to one deployment and cannot be rotated " +
			"when an administrator, treasury, token, or target contract changes.",
		Fix: "Take the address as a __constructor parameter and store it in instance " +
			"storage so each deployment can configure and later migrate it deliberately.",
	}
}

func (HardcodedAddressLiteral) Check(c *rule.Context) []rule.Finding {
	// A file without a deployed contract implementation cannot freeze an
	// address into that contract's WASM.
	if len(c.ContractFns()) == 0 {
		return nil
	}

	var out []rule.Finding
	c.Root.Walk(func(n rule.Node) bool {
		// Attribute strings are metadata, not runtime values. Skipping a
		// #[cfg(test)] item also skips its whole subtree, including fixture
		// addresses that are legitimately pinned in tests.
		if n.Kind() == "attribute_item" || hasCfgTestAttribute(n) {
			return false
		}
		if n.Kind() == "string_literal" && isStellarAddressLiteral(n.Text()) {
			out = append(out, rule.At(n, "hardcoded Stellar address cannot vary by deployment; "+
				"take it as a __constructor parameter and store it in instance storage"))
		}
		return true
	})
	return out
}

// isStellarAddressLiteral recognizes the address-shaped subset of SEP-23
// strkeys. Verified against SEP-23 v1.3.0 on 2026-09-06: G and C strkeys are
// 56 characters, while M strkeys include an 8-byte memo ID and are 69. S is a
// private seed rather than an address and deliberately belongs in a secret
// scanner, not this rule. The soroban-sdk 27.0.5 Address docs were also checked:
// from_str, from_string, and from_string_bytes accept G and C strkeys.
func isStellarAddressLiteral(literal string) bool {
	if len(literal) < 2 || literal[0] != '"' || literal[len(literal)-1] != '"' {
		return false
	}
	value := literal[1 : len(literal)-1]
	standardAddress := len(value) == standardStrkeyLength && (value[0] == 'G' || value[0] == 'C')
	muxedAddress := len(value) == muxedStrkeyLength && value[0] == 'M'
	if !standardAddress && !muxedAddress {
		return false
	}
	for _, c := range value {
		if (c < 'A' || c > 'Z') && (c < '2' || c > '7') {
			return false
		}
	}
	return true
}

func hasCfgTestAttribute(item rule.Node) bool {
	for prev, ok := item.PrevSibling(); ok; prev, ok = prev.PrevSibling() {
		if prev.Kind() != "attribute_item" {
			return false
		}
		attr, ok := prev.Child(0)
		if ok && strings.Join(strings.Fields(attr.Text()), "") == "cfg(test)" {
			return true
		}
	}
	return false
}
