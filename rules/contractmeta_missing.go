package rules

import "github.com/use-plumbline/plumbline/internal/rule"

const contractMetaMacro = "contractmeta"

// ContractmetaMissing reports contract declaration files without authored
// contract metadata.
type ContractmetaMissing struct{}

func (ContractmetaMissing) Meta() rule.Meta {
	return rule.Meta{
		ID:       "contractmeta-missing",
		Severity: rule.SeverityNote,
		Summary:  "A contract declaration has no authored contract metadata.",
		Why: "Authored contractmeta! entries are serialized into the deployed WASM's " +
			"contractmetav0 custom section, so they travel with the contract and help " +
			"someone identify an unfamiliar contract later. This rule is deliberately " +
			"scoped to files with the #[contract] declaration, not #[contractimpl] alone, " +
			"to avoid false positives in multi-file crates that keep metadata elsewhere.",
		Fix: "Add an authored metadata entry near the contract declaration, for example:\n\n" +
			"contractmeta!(\n" +
			"    key = \"desc\",\n" +
			"    val = \"Describe this contract\"\n" +
			");",
	}
}

func (ContractmetaMissing) Check(c *rule.Context) []rule.Finding {
	contract, ok := contractDeclaration(c.Root)
	if !ok || hasContractMeta(c.Root) {
		return nil
	}
	return []rule.Finding{
		rule.At(contract, "contract declaration has no authored contractmeta! entry"),
	}
}

// contractDeclaration returns the first actual #[contract] type declaration in
// the file. Verified with docs.rs/soroban-sdk 27.0.5 on 2026-08-14: authored
// contractmeta! adds SCMetaV0 entries, while SDK build/version metadata is
// injected separately, so this rule checks for source-level presence only.
func contractDeclaration(root rule.Node) (rule.Node, bool) {
	var found rule.Node
	root.Walk(func(n rule.Node) bool {
		if found.Valid() {
			return false
		}
		if n.Kind() == "struct_item" && rule.HasAttribute(n, "contract") {
			found = n
			return false
		}
		return true
	})
	return found, found.Valid()
}

func hasContractMeta(root rule.Node) bool {
	found := false
	root.Walk(func(n rule.Node) bool {
		if found {
			return false
		}
		if name, ok := rule.MacroName(n); ok && name == contractMetaMacro {
			found = true
			return false
		}
		return true
	})
	return found
}
