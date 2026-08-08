package rule

import (
	"fmt"
	"sort"
)

// Registry is the set of rules the engine will run.
//
// Registration is explicit — the rules package builds one from a literal list
// rather than rules self-registering in init(). Adding a rule is then a
// two-file diff a reviewer can read in full: the rule, and one line naming it.
type Registry struct {
	rules []Rule
	byID  map[string]Rule
}

// NewRegistry validates and returns a registry over rs.
//
// It fails on a missing or duplicated ID, on an unknown severity, and on
// missing documentation. Those are build-time mistakes in a static list, and
// a rule that ships without a "why" and a "fix" is not finished.
func NewRegistry(rs ...Rule) (*Registry, error) {
	reg := &Registry{byID: make(map[string]Rule, len(rs))}
	for _, r := range rs {
		m := r.Meta()
		switch {
		case m.ID == "":
			return nil, fmt.Errorf("rule %T: empty ID", r)
		case m.Summary == "" || m.Why == "" || m.Fix == "":
			return nil, fmt.Errorf("rule %q: Summary, Why and Fix are all required", m.ID)
		}
		switch m.Severity {
		case SeverityError, SeverityWarning, SeverityNote:
		default:
			return nil, fmt.Errorf("rule %q: unknown severity %q", m.ID, m.Severity)
		}
		if _, dup := reg.byID[m.ID]; dup {
			return nil, fmt.Errorf("duplicate rule ID %q", m.ID)
		}
		reg.byID[m.ID] = r
		reg.rules = append(reg.rules, r)
	}
	sort.Slice(reg.rules, func(i, j int) bool {
		return reg.rules[i].Meta().ID < reg.rules[j].Meta().ID
	})
	return reg, nil
}

// Rules returns the registered rules, ordered by ID so that output is stable.
func (r *Registry) Rules() []Rule { return r.rules }

// Get returns the rule with the given ID.
func (r *Registry) Get(id string) (Rule, bool) {
	x, ok := r.byID[id]
	return x, ok
}
