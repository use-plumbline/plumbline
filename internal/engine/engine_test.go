package engine

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/use-plumbline/plumbline/internal/rule"
)

// contractSrc is a minimal contract with one entry point that writes storage.
const contractSrc = `#[contractimpl]
impl C {
    pub fn set(env: Env, v: u32) {
        env.storage().instance().set(&DataKey::V, &v);
    }
}`

// writesStorage fires on any contract function, so tests can assert on
// findings without depending on a real rule's semantics.
type writesStorage struct{ severity rule.Severity }

func (w writesStorage) Meta() rule.Meta {
	return rule.Meta{
		ID: "test-rule", Severity: w.severity,
		Summary: "s", Why: "w", Fix: "f",
	}
}

func (writesStorage) Check(c *rule.Context) []rule.Finding {
	var out []rule.Finding
	for _, fn := range c.ContractFns() {
		out = append(out, rule.At(fn.Node, "%s", fn.Name))
	}
	return out
}

func registry(t *testing.T, rs ...rule.Rule) *rule.Registry {
	t.Helper()
	reg, err := rule.NewRegistry(rs...)
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

func TestDiscoverRust(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "b.rs", "")
	write(t, dir, "a.rs", "")
	write(t, dir, "notes.md", "")
	write(t, dir, "nested/c.rs", "")
	// Cargo build output is vendored and generated; linting it is slow and
	// is never about the contract under review.
	write(t, dir, "target/debug/generated.rs", "")
	write(t, dir, "node_modules/pkg/d.rs", "")
	// Rust's test sources hold mock contracts written to exercise one path.
	// They are never deployed, so holding them to the rules would report
	// defects that cannot reach a ledger.
	write(t, dir, "src/test.rs", "")
	write(t, dir, "src/tests.rs", "")
	write(t, dir, "tests/integration.rs", "")
	write(t, dir, "src/test/helpers.rs", "")

	got, err := DiscoverRust([]string{dir})
	if err != nil {
		t.Fatalf("discover: %v", err)
	}
	want := []string{
		filepath.Join(dir, "a.rs"),
		filepath.Join(dir, "b.rs"),
		filepath.Join(dir, "nested/c.rs"),
	}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

// A path naming a file is taken as given, so `plumbline thing.txt` does what
// it says rather than silently finding nothing — and so the test sources that
// walking skips can still be linted on request.
func TestDiscoverRustTakesNamedFilesAsGiven(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"contract.txt", "src/test.rs"} {
		path := write(t, dir, name, "")
		got, err := DiscoverRust([]string{path})
		if err != nil {
			t.Fatalf("discover: %v", err)
		}
		if len(got) != 1 || got[0] != path {
			t.Fatalf("got %v, want [%s]", got, path)
		}
	}
}

func TestDiscoverRustDeduplicates(t *testing.T) {
	dir := t.TempDir()
	path := write(t, dir, "a.rs", "")
	got, err := DiscoverRust([]string{dir, path, path})
	if err != nil {
		t.Fatalf("discover: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %v, want one entry", got)
	}
}

func TestDiscoverRustRejectsMissingPaths(t *testing.T) {
	if _, err := DiscoverRust([]string{filepath.Join(t.TempDir(), "nope")}); err == nil {
		t.Fatal("want an error for a path that does not exist, got nil")
	}
}

func TestRunStampsFindings(t *testing.T) {
	dir := t.TempDir()
	path := write(t, dir, "lib.rs", contractSrc)

	res, err := New(registry(t, writesStorage{rule.SeverityWarning})).Run([]string{path})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if res.Linted != 1 {
		t.Fatalf("linted %d files, want 1", res.Linted)
	}
	if len(res.Findings) != 1 {
		t.Fatalf("got %d findings, want 1", len(res.Findings))
	}
	f := res.Findings[0]
	if f.Path != path || f.RuleID != "test-rule" || f.Severity != rule.SeverityWarning {
		t.Errorf("finding not stamped: %+v", f)
	}
}

// A rule reading a broken parse tree reports nonsense, so the file is skipped
// and the skip is surfaced rather than swallowed.
func TestRunSkipsUnparseableFiles(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "broken.rs", "fn f( {{{ this is not rust")
	write(t, dir, "fine.rs", contractSrc)

	res, err := New(registry(t, writesStorage{rule.SeverityError})).Run([]string{dir})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if res.Linted != 1 {
		t.Errorf("linted %d files, want 1", res.Linted)
	}
	if len(res.Skipped) != 1 {
		t.Fatalf("got %d skipped, want 1", len(res.Skipped))
	}
	if filepath.Base(res.Skipped[0].Path) != "broken.rs" {
		t.Errorf("skipped %q, want broken.rs", res.Skipped[0].Path)
	}
}

// The seam .plumbline.toml will use: severity and enablement are the engine's,
// keyed by rule ID, so no rule ever consults configuration.
func TestSeverityOverrideAndDisable(t *testing.T) {
	dir := t.TempDir()
	path := write(t, dir, "lib.rs", contractSrc)

	e := New(registry(t, writesStorage{rule.SeverityWarning}))
	e.SetSeverity("test-rule", rule.SeverityError)
	res, err := e.Run([]string{path})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if len(res.Findings) != 1 || res.Findings[0].Severity != rule.SeverityError {
		t.Fatalf("severity override ignored: %+v", res.Findings)
	}
	if !res.HasSeverity(rule.SeverityError) || res.HasSeverity(rule.SeverityNote) {
		t.Error("HasSeverity disagrees with the findings")
	}

	e = New(registry(t, writesStorage{rule.SeverityWarning}))
	e.Disable("test-rule")
	res, err = e.Run([]string{path})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if len(res.Findings) != 0 {
		t.Errorf("disabled rule still reported %d findings", len(res.Findings))
	}
	if res.Linted != 1 {
		t.Errorf("disabling a rule changed the file count")
	}
}

// An excluded file is dropped before it is read, so it costs nothing and
// cannot contribute a skip or a finding.
func TestExcludedFilesAreNotRead(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "keep.rs", contractSrc)
	write(t, dir, "drop.rs", contractSrc)

	e := New(registry(t, writesStorage{rule.SeverityError}))
	e.SetExcluded(func(path string) bool { return filepath.Base(path) == "drop.rs" })
	res, err := e.Run([]string{dir})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if res.Linted != 1 {
		t.Errorf("linted %d files, want 1", res.Linted)
	}
	if len(res.Findings) != 1 || filepath.Base(res.Findings[0].Path) != "keep.rs" {
		t.Errorf("excluded file was linted: %+v", res.Findings)
	}
	if len(res.Skipped) != 0 {
		t.Errorf("an excluded file was reported as skipped: %+v", res.Skipped)
	}
}

func TestFindingsAreSortedDeterministically(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "b.rs", contractSrc)
	write(t, dir, "a.rs", contractSrc)

	res, err := New(registry(t, writesStorage{rule.SeverityError})).Run([]string{dir})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if len(res.Findings) != 2 {
		t.Fatalf("got %d findings, want 2", len(res.Findings))
	}
	if filepath.Base(res.Findings[0].Path) != "a.rs" {
		t.Errorf("findings not sorted by path: %v", res.Findings)
	}
}
