package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/use-plumbline/plumbline/internal/config"
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

// writeConfig puts a .plumbline.toml in a temporary directory and returns its
// path, for tests that drive configuration through --config.
func writeConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), config.FileName)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// Configuration reaches the exit code, which is the only part of it CI can
// see. Each case here is one of the reasons a team reaches for a config file
// at all: turn a rule down, turn it off, or keep Plumbline out of a directory.
func TestConfigChangesTheRun(t *testing.T) {
	const dirty = "../../testdata/rules/missing-auth/fail.rs"

	tests := []struct {
		name   string
		config string
		want   int
	}{
		// missing-auth is an error by default, and errors fail the run.
		{"no config keeps the defaults", "", exitFindings},
		{"severity lowered below the threshold", "[rules]\nmissing-auth = \"warning\"\n", exitClean},
		{"severity raised is still a failure", "[rules]\nmissing-auth = \"error\"\n", exitFindings},
		{"rule switched off", "[rules]\nmissing-auth = \"off\"\n", exitClean},
		{"path excluded", "exclude = [\"**/missing-auth/**\"]\n", exitClean},
		{"a different path excluded changes nothing", "exclude = [\"**/panic-in-contract/**\"]\n", exitFindings},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			args := []string{dirty}
			if tc.config != "" {
				args = append([]string{"--config", writeConfig(t, tc.config)}, args...)
			}
			var stdout, stderr bytes.Buffer
			if got := run(args, &stdout, &stderr); got != tc.want {
				t.Errorf("exit %d, want %d\nstdout: %s\nstderr: %s", got, tc.want, &stdout, &stderr)
			}
		})
	}
}

// A bad config file must stop the run rather than be ignored: a team that
// thinks it has turned a rule off, and has not, gets the worst of both.
func TestBadConfigIsAUsageError(t *testing.T) {
	const clean = "../../testdata/rules/missing-auth/pass.rs"
	tests := []struct {
		name string
		args []string
	}{
		{"file does not exist", []string{"--config", filepath.Join(t.TempDir(), "nope.toml"), clean}},
		{"unknown rule", []string{"--config", writeConfig(t, "[rules]\nnope = \"off\"\n"), clean}},
		{"unknown severity", []string{"--config", writeConfig(t, "[rules]\nmissing-auth = \"loud\"\n"), clean}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if got := run(tc.args, &stdout, &stderr); got != exitUsage {
				t.Errorf("exit %d, want %d\nstdout: %s", got, exitUsage, &stdout)
			}
			if stderr.Len() == 0 {
				t.Error("nothing was written to stderr explaining the failure")
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
