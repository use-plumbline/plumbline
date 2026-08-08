package rules

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/use-plumbline/plumbline/internal/engine"
	"github.com/use-plumbline/plumbline/internal/rule"
)

// fixtureDir is where every rule's pass/fail pair lives, relative to this
// package.
const fixtureDir = "../testdata/rules"

// expectMarker flags a line in a fail fixture where a finding is expected:
//
//	pub fn set_admin(env: Env, who: Address) { //~ missing-auth
//
// Writing the expectation on the offending line keeps the fixture readable as
// Rust and makes a wrong line number a test failure rather than a detail
// nobody checks. The convention is borrowed from rustc's UI tests.
const expectMarker = "//~"

// TestRuleFixtures runs every registered rule against its own pass and fail
// fixtures.
//
// It is table-driven off the registry rather than a hand-written list, so a
// rule contributed without both fixtures fails here instead of shipping
// untested.
func TestRuleFixtures(t *testing.T) {
	for _, r := range Default().Rules() {
		id := r.Meta().ID
		t.Run(id, func(t *testing.T) {
			t.Run("pass", func(t *testing.T) {
				path := filepath.Join(fixtureDir, id, "pass.rs")
				got := runRule(t, r, path)
				for _, f := range got {
					t.Errorf("%s:%d: rule fired on the pass fixture: %s", path, f.Line, f.Message)
				}
			})

			t.Run("fail", func(t *testing.T) {
				path := filepath.Join(fixtureDir, id, "fail.rs")
				want := expectedLines(t, path, id)
				if len(want) == 0 {
					t.Fatalf("%s: no %s %s markers; a fail fixture must say where it expects findings", path, expectMarker, id)
				}
				got := foundLines(runRule(t, r, path))
				if !sameLines(got, want) {
					t.Errorf("findings on lines %v, expected %v", got, want)
				}
			})
		})
	}
}

// TestDefaultRegistry asserts the shipped rule set is well formed, so that a
// bad rule is a test failure rather than a panic in a user's CI.
func TestDefaultRegistry(t *testing.T) {
	if _, err := rule.NewRegistry(all()...); err != nil {
		t.Fatalf("built-in rule set is invalid: %v", err)
	}
	if len(Default().Rules()) == 0 {
		t.Fatal("no rules registered")
	}
}

// runRule lints one fixture with exactly one rule enabled, so a finding can
// only have come from the rule under test.
func runRule(t *testing.T, r rule.Rule, path string) []rule.Finding {
	t.Helper()
	reg, err := rule.NewRegistry(r)
	if err != nil {
		t.Fatalf("registry: %v", err)
	}
	res, err := engine.New(reg).Run([]string{path})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	for _, s := range res.Skipped {
		t.Fatalf("%s was skipped: %s", s.Path, s.Reason)
	}
	if res.Linted != 1 {
		t.Fatalf("linted %d files, want 1", res.Linted)
	}
	return res.Findings
}

// expectedLines reads the `//~ <rule-id>` markers out of a fixture.
func expectedLines(t *testing.T, path, ruleID string) []int {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("opening fixture: %v", err)
	}
	defer func() { _ = f.Close() }()

	var lines []int
	scan := bufio.NewScanner(f)
	for n := 1; scan.Scan(); n++ {
		_, marker, ok := strings.Cut(scan.Text(), expectMarker)
		if !ok {
			continue
		}
		if strings.Fields(marker)[0] != ruleID {
			t.Fatalf("%s:%d: marker names %q, but this fixture belongs to %q",
				path, n, strings.Fields(marker)[0], ruleID)
		}
		lines = append(lines, n)
	}
	if err := scan.Err(); err != nil {
		t.Fatalf("reading fixture: %v", err)
	}
	sort.Ints(lines)
	return lines
}

func foundLines(fs []rule.Finding) []int {
	lines := make([]int, len(fs))
	for i, f := range fs {
		lines[i] = f.Line
	}
	sort.Ints(lines)
	return lines
}

func sameLines(a, b []int) bool {
	return fmt.Sprint(a) == fmt.Sprint(b)
}
