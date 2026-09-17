package rules

import "github.com/use-plumbline/plumbline/internal/rule"

// ttlStorageMutators are the Persistent methods that write an entry, and so
// start its rent clock.
var ttlStorageMutators = map[string]bool{
	"set": true, "update": true, "try_update": true,
}

// ttlExtenders are the two methods that push a persistent entry's expiry out.
//
// Verified against docs.rs/soroban-sdk 27.0.5 on 2026-09-16:
//
//	pub fn extend_ttl<K>(&self, key: &K, threshold: u32, extend_to: u32)
//	pub fn extend_ttl_with_limits<K>(&self, key: &K, extend_to: u32,
//	                                 min_extension: u32, max_extension: u32)
var ttlExtenders = map[string]bool{
	"extend_ttl": true, "extend_ttl_with_limits": true,
}

// MissingTTLExtension reports a contract that writes persistent storage and
// never extends a persistent TTL anywhere in the file.
//
// Found by reading the corpus, not by invention: eleven files across the three
// pinned repositories write persistent storage with no extend_ttl call, and
// for ten of them there is no extend_ttl anywhere in the whole crate. See
// docs/corpus-run.md.
type MissingTTLExtension struct{}

func (MissingTTLExtension) Meta() rule.Meta {
	return rule.Meta{
		ID:       "missing-ttl-extension",
		Severity: rule.SeverityWarning,
		Summary:  "A contract writes persistent storage but never extends its TTL.",
		Why: "Persistent entries are rented. When the rent runs out the entry is " +
			"removed from the ledger, and the SDK is explicit that an expired entry " +
			"\"can be restored and cannot be recreated\" — so the contract cannot " +
			"simply write it again. Until somebody pays to restore it, every read " +
			"of that key fails, and the contract that looked correct in testing is " +
			"stuck. A contract that never extends a TTL is relying on its data " +
			"outliving its rent by luck.",
		Fix: "Call extend_ttl on the persistent entry after writing it, with a " +
			"threshold and a target measured in ledgers. If the data is genuinely " +
			"short-lived, say so by storing it in temporary() instead — a temporary " +
			"entry that expires is deleted outright, which is the honest version of " +
			"the same intent.",
	}
}

// Check reports the contract if it writes persistent storage and extends no
// persistent TTL.
//
// Known blind spot: a storage handle bound to a local before use —
//
//	let store = env.storage().persistent();
//	store.set(&key, &value);
//
// is not recognised as a persistent write, so a contract written that way is
// not reported even with no extension anywhere. Measured rather than guessed:
// that shape appears nowhere in a contract file in the pinned corpus, only in
// fuzz harnesses, so it is recorded here instead of being fixed on
// speculation. See docs/corpus-run.md.
func (MissingTTLExtension) Check(c *rule.Context) []rule.Finding {
	// Only contract files. A library module that writes persistent storage on
	// a caller's behalf is not where the TTL decision belongs, and flagging it
	// reports the same missing extension once per helper.
	if len(c.ContractFns()) == 0 {
		return nil
	}
	if fileExtendsPersistentTTL(c.Root) {
		return nil
	}
	write, ok := firstPersistentWrite(c.Root)
	if !ok {
		return nil
	}
	// One finding per file. The absent extension is a property of the
	// contract, not of each write, and reporting every write would bury the
	// point under repetition.
	return []rule.Finding{rule.At(write,
		"this contract writes persistent storage but never extends a persistent TTL")}
}

// fileExtendsPersistentTTL reports whether anything in the file extends a
// persistent entry's TTL.
//
// The whole file is searched rather than the writing function, because the
// idiomatic shape puts the write and the extension together in one helper that
// entry points call — testdata/sample-contract's write_balance is exactly that.
// The extension must be on a persistent receiver: extending the instance TTL
// says nothing about whether a persistent entry will survive.
func fileExtendsPersistentTTL(root rule.Node) bool {
	found := false
	root.Walk(func(n rule.Node) bool {
		if found {
			return false
		}
		if call, ok := rule.AsMethodCall(n); ok &&
			ttlExtenders[call.Name] && receiverChainHas(call.Recv, "persistent") {
			found = true
			return false
		}
		return true
	})
	return found
}

// firstPersistentWrite returns the first persistent storage write in the file.
func firstPersistentWrite(root rule.Node) (rule.Node, bool) {
	var found rule.Node
	root.Walk(func(n rule.Node) bool {
		if found.Valid() {
			return false
		}
		if call, ok := rule.AsMethodCall(n); ok &&
			ttlStorageMutators[call.Name] && receiverChainHas(call.Recv, "persistent") {
			found = call.Field
			return false
		}
		return true
	})
	return found, found.Valid()
}
