// Package rules holds Plumbline's lint rules, one per file.
//
// A rule never imports another rule. Each is a self-contained check over a
// parsed file, so it can be reviewed, tested, and contributed on its own.
package rules

import (
	"fmt"

	"github.com/use-plumbline/plumbline/internal/rule"
)

// all is the complete set of rules Plumbline ships. Adding a rule means adding
// its file and one line here — a diff a reviewer can read end to end.
func all() []rule.Rule {
	return []rule.Rule{
		MissingAuth{},
		PanicInContract{},
	}
}

// Default returns the registry of every rule Plumbline ships.
//
// It panics on an invalid rule set. The list is a compile-time constant, so a
// duplicate ID or an undocumented rule is a build mistake, not a runtime
// condition a caller could recover from — and TestDefaultRegistry catches it
// before it ever reaches a user.
func Default() *rule.Registry {
	reg, err := rule.NewRegistry(all()...)
	if err != nil {
		panic(fmt.Sprintf("plumbline: built-in rule set is invalid: %v", err))
	}
	return reg
}
