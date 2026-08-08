package main

import (
	"bytes"
	"strings"
	"testing"
)

// The exit code is the Action's contract with the workflow that runs it, so it
// is worth pinning.
func TestExitCodes(t *testing.T) {
	const (
		clean = "../../testdata/rules/missing-auth/pass.rs"
		dirty = "../../testdata/rules/missing-auth/fail.rs"
	)
	tests := []struct {
		name string
		args []string
		want int
	}{
		{"no findings", []string{clean}, exitClean},
		{"errors fail by default", []string{dirty}, exitFindings},
		{"fail-on never always succeeds", []string{"--fail-on", "never", dirty}, exitClean},
		{"warnings alone do not fail by default", []string{"../../testdata/rules/unchecked-arithmetic/fail.rs"}, exitClean},
		{"fail-on warning catches them", []string{"--fail-on", "warning", "../../testdata/rules/unchecked-arithmetic/fail.rs"}, exitFindings},
		{"no paths is a usage error", nil, exitUsage},
		{"unreadable path is a usage error", []string{"../../testdata/does-not-exist"}, exitUsage},
		{"unknown format is a usage error", []string{"--format", "sarif", clean}, exitUsage},
		{"unknown fail-on is a usage error", []string{"--fail-on", "loud", clean}, exitUsage},
		{"unknown rule is a usage error", []string{"--explain", "nope"}, exitUsage},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if got := run(tc.args, &stdout, &stderr); got != tc.want {
				t.Errorf("exit %d, want %d\nstdout: %s\nstderr: %s", got, tc.want, &stdout, &stderr)
			}
		})
	}
}

func TestListRulesAndExplain(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if got := run([]string{"--list-rules"}, &stdout, &stderr); got != exitClean {
		t.Fatalf("exit %d: %s", got, &stderr)
	}
	listing := stdout.String()
	for _, id := range []string{"missing-auth", "panic-in-contract", "unchecked-arithmetic"} {
		if !strings.Contains(listing, id) {
			t.Errorf("--list-rules omits %q:\n%s", id, listing)
		}

		stdout.Reset()
		if got := run([]string{"--explain", id}, &stdout, &stderr); got != exitClean {
			t.Fatalf("--explain %s: exit %d: %s", id, got, &stderr)
		}
		for _, section := range []string{"Why it matters", "How to fix it"} {
			if !strings.Contains(stdout.String(), section) {
				t.Errorf("--explain %s has no %q section", id, section)
			}
		}
	}
}
