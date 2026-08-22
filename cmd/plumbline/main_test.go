package main

import (
	"bytes"
	"encoding/json"
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
	const dirty = "../../testdata/cmd/missing-auth-config/dirty.rs"
	tests := []struct {
		name   string
		config string
		want   int
	}{
		{"no config keeps the defaults", "", exitFindings},
		{"severity lowered below the threshold", "[rules]\nmissing-auth = \"warning\"\nmissing-reinit-guard = \"off\"\n", exitClean},
		{"severity raised is still a failure", "[rules]\nmissing-auth = \"error\"\nmissing-reinit-guard = \"off\"\n", exitFindings},
		{"rule switched off", "[rules]\nmissing-auth = \"off\"\nmissing-reinit-guard = \"off\"\n", exitClean},
		{"path excluded", "exclude = [\"**/missing-auth-config/**\"]\n", exitClean},
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

// The JSON report is a contract with whatever consumes it, so the test asserts
// on the parsed document rather than on the text of it.
func TestJSONOutput(t *testing.T) {
	var stdout, stderr bytes.Buffer
	args := []string{"--format", "json", "--fail-on", "never", "../../testdata/rules/missing-auth/fail.rs"}
	if got := run(args, &stdout, &stderr); got != exitClean {
		t.Fatalf("exit %d: %s", got, &stderr)
	}

	var doc struct {
		SchemaVersion int `json:"schemaVersion"`
		Findings      []struct {
			Rule     string `json:"rule"`
			Severity string `json:"severity"`
			File     string `json:"file"`
			Line     int    `json:"line"`
			Message  string `json:"message"`
		} `json:"findings"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &doc); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\n%s", err, &stdout)
	}
	if doc.SchemaVersion == 0 {
		t.Error("no schemaVersion in the output")
	}
	if len(doc.Findings) == 0 {
		t.Fatalf("no findings reported for a fixture that should fail:\n%s", &stdout)
	}
	var f *struct {
		Rule     string `json:"rule"`
		Severity string `json:"severity"`
		File     string `json:"file"`
		Line     int    `json:"line"`
		Message  string `json:"message"`
	}

	for i := range doc.Findings {
		if doc.Findings[i].Rule == "missing-auth" {
			f = &doc.Findings[i]
			break
		}
	}

	if f == nil {
		t.Fatal("missing-auth finding not present in JSON output")
	}

	if f.Severity != "error" || f.Line == 0 || f.Message == "" {
		t.Errorf("finding is missing information: %+v", *f)
	}
	if !strings.HasSuffix(f.File, "fail.rs") {
		t.Errorf("file is %q, want the path that was linted", f.File)
	}
}

// Nothing but the report may reach stdout in JSON mode

// Nothing but the report may reach stdout in JSON mode, or `plumbline | jq`
// breaks the first time a file cannot be parsed.
func TestJSONOutputIsTheOnlyThingOnStdout(t *testing.T) {
	var stdout, stderr bytes.Buffer
	args := []string{"--format", "json", "../../testdata/rules/missing-auth/pass.rs"}
	if got := run(args, &stdout, &stderr); got != exitClean {
		t.Fatalf("exit %d: %s", got, &stderr)
	}
	if err := json.Unmarshal(stdout.Bytes(), &map[string]any{}); err != nil {
		t.Fatalf("stdout is not exactly one JSON document: %v\n%s", err, &stdout)
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
