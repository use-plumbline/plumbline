package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/use-plumbline/plumbline/internal/engine"
	"github.com/use-plumbline/plumbline/internal/rule"
)

// contractSrc is a minimal contract with one entry point that writes storage.
const contractSrc = `#[contractimpl]
impl C {
    pub fn set(env: Env, v: u32) {
        env.storage().instance().set(&DataKey::V, &v);
    }
}`

// fires reports one finding per contract function, so a test can assert on
// configuration without depending on a real rule's semantics.
type fires struct{}

func (fires) Meta() rule.Meta {
	return rule.Meta{
		ID: "test-rule", Severity: rule.SeverityWarning,
		Summary: "s", Why: "w", Fix: "f",
	}
}

func (fires) Check(c *rule.Context) []rule.Finding {
	var out []rule.Finding
	for _, fn := range c.ContractFns() {
		out = append(out, rule.At(fn.Node, "%s", fn.Name))
	}
	return out
}

func testRegistry(t *testing.T) *rule.Registry {
	t.Helper()
	reg, err := rule.NewRegistry(fires{})
	if err != nil {
		t.Fatalf("registry: %v", err)
	}
	return reg
}

// write creates a file under dir, making parent directories as needed.
func write(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// run lints dir under cfg and returns the findings.
func run(t *testing.T, cfg *Config, dir string) []rule.Finding {
	t.Helper()
	e := engine.New(testRegistry(t))
	cfg.Apply(e)
	res, err := e.Run([]string{dir})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	return res.Findings
}

// A project with no configuration file must behave exactly as Plumbline did
// before configuration existed: every rule on, at its declared severity.
func TestNoConfigFileIsTheDefault(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "lib.rs", contractSrc)

	cfg, err := Discover(dir, testRegistry(t))
	if err != nil {
		t.Fatalf("discover: %v", err)
	}
	if cfg.Path != "" {
		t.Errorf("Path is %q, want empty for the defaults", cfg.Path)
	}

	findings := run(t, cfg, dir)
	if len(findings) != 1 || findings[0].Severity != rule.SeverityWarning {
		t.Fatalf("defaults changed the run: %+v", findings)
	}
}

func TestSeverityOverride(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "lib.rs", contractSrc)
	write(t, dir, FileName, "[rules]\ntest-rule = \"error\"\n")

	cfg, err := Discover(dir, testRegistry(t))
	if err != nil {
		t.Fatalf("discover: %v", err)
	}
	if cfg.Path == "" {
		t.Error("Path is empty, want the file that was read")
	}

	findings := run(t, cfg, dir)
	if len(findings) != 1 {
		t.Fatalf("got %d findings, want 1", len(findings))
	}
	if findings[0].Severity != rule.SeverityError {
		t.Errorf("severity is %q, want error", findings[0].Severity)
	}
}

func TestRuleDisabled(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "lib.rs", contractSrc)
	write(t, dir, FileName, "[rules]\ntest-rule = \"off\"\n")

	cfg, err := Discover(dir, testRegistry(t))
	if err != nil {
		t.Fatalf("discover: %v", err)
	}
	if findings := run(t, cfg, dir); len(findings) != 0 {
		t.Fatalf("a disabled rule still reported %+v", findings)
	}
}

// An excluded file is not linted at all — not parsed, not counted. Anything
// less and `exclude` would only be hiding findings, which is a different and
// much weaker promise.
func TestPathExcluded(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "contracts/vault/src/lib.rs", contractSrc)
	write(t, dir, "vendor/other/src/lib.rs", contractSrc)
	write(t, dir, FileName, "exclude = [\"**/vendor/**\"]\n")

	cfg, err := Discover(dir, testRegistry(t))
	if err != nil {
		t.Fatalf("discover: %v", err)
	}

	e := engine.New(testRegistry(t))
	cfg.Apply(e)
	res, err := e.Run([]string{dir})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if res.Linted != 1 {
		t.Errorf("linted %d files, want 1 — the excluded file was still read", res.Linted)
	}
	if len(res.Findings) != 1 {
		t.Fatalf("got %d findings, want 1", len(res.Findings))
	}
	if strings.Contains(res.Findings[0].Path, "vendor") {
		t.Errorf("excluded file was linted: %s", res.Findings[0].Path)
	}
}

// A misspelt rule name in [rules] would otherwise look exactly like a rule
// that had been turned off successfully, so it has to be an error.
func TestUnknownSettingsAreErrors(t *testing.T) {
	tests := []struct {
		name    string
		content string
		wantIn  string
	}{
		{"unknown rule", "[rules]\nmissing-authz = \"off\"\n", "no such rule"},
		{"unknown severity", "[rules]\ntest-rule = \"loud\"\n", "expected"},
		{"unknown top-level key", "excludes = [\"a\"]\n", "unknown setting"},
		{"unknown section", "[reporting]\nformat = \"json\"\n", "unknown setting"},
		{"empty exclude pattern", "exclude = [\"\"]\n", "empty pattern"},
		{"not toml at all", "exclude: [a]\n", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := write(t, dir, FileName, tc.content)
			_, err := Load(path, testRegistry(t))
			if err == nil {
				t.Fatalf("%q was accepted", tc.content)
			}
			if !strings.Contains(err.Error(), tc.wantIn) {
				t.Errorf("error %q does not mention %q", err, tc.wantIn)
			}
			if !strings.Contains(err.Error(), FileName) {
				t.Errorf("error %q does not name the file it came from", err)
			}
		})
	}
}

// Naming a config file that is not there is a mistake worth failing on; a
// missing default is not.
func TestLoadRequiresTheFileItWasGiven(t *testing.T) {
	if _, err := Load(filepath.Join(t.TempDir(), FileName), testRegistry(t)); err == nil {
		t.Fatal("Load accepted a path that does not exist")
	}
}

func TestEveryShippedSeverityIsAccepted(t *testing.T) {
	for _, value := range []string{"error", "warning", "note", "off"} {
		dir := t.TempDir()
		path := write(t, dir, FileName, "[rules]\ntest-rule = \""+value+"\"\n")
		cfg, err := Load(path, testRegistry(t))
		if err != nil {
			t.Fatalf("%q rejected: %v", value, err)
		}
		if value == "off" {
			if !cfg.Disabled["test-rule"] {
				t.Errorf("%q did not disable the rule", value)
			}
			continue
		}
		if got := cfg.Severity["test-rule"]; string(got) != value {
			t.Errorf("%q became %q", value, got)
		}
	}
}
