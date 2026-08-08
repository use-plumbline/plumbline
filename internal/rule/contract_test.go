package rule

import (
	"testing"

	"github.com/use-plumbline/plumbline/internal/syntax"
)

// parse is a test-local parser so that this package's tests do not depend on
// the engine.
func parse(t *testing.T, src string) *Context {
	t.Helper()
	p, err := syntax.NewParser()
	if err != nil {
		t.Fatalf("new parser: %v", err)
	}
	t.Cleanup(p.Close)
	tree, err := p.Parse([]byte(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	t.Cleanup(tree.Close)
	if tree.Root().HasError() {
		t.Fatalf("fixture does not parse cleanly:\n%s", src)
	}
	return &Context{Path: "test.rs", Root: tree.Root()}
}

func names(fns []ContractFn) []string {
	out := make([]string, len(fns))
	for i, f := range fns {
		out[i] = f.Name
	}
	return out
}

func TestContractFns(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want []string
	}{
		{
			name: "pub fns in a contractimpl block",
			src: `#[contractimpl]
impl C {
    pub fn a(env: Env) {}
    pub fn b(env: Env) {}
}`,
			want: []string{"a", "b"},
		},
		{
			name: "private helpers are not entry points",
			src: `#[contractimpl]
impl C {
    pub fn a(env: Env) {}
    fn helper() {}
}`,
			want: []string{"a"},
		},
		{
			name: "plain impl blocks are ignored",
			src: `impl C {
    pub fn not_an_entry_point(env: Env) {}
}`,
			want: nil,
		},
		{
			name: "attribute stacks above the impl",
			src: `#[contractimpl]
#[cfg(feature = "x")]
impl C {
    pub fn a(env: Env) {}
}`,
			want: []string{"a"},
		},
		{
			name: "attribute stacks below other attributes",
			src: `#[cfg(feature = "x")]
#[contractimpl]
impl C {
    pub fn a(env: Env) {}
}`,
			want: []string{"a"},
		},
		{
			name: "path-qualified attribute",
			src: `#[soroban_sdk::contractimpl]
impl C {
    pub fn a(env: Env) {}
}`,
			want: []string{"a"},
		},
		{
			name: "contract inside a module",
			src: `mod inner {
    #[contractimpl]
    impl C {
        pub fn a(env: Env) {}
    }
}`,
			want: []string{"a"},
		},
		{
			name: "a preceding non-attribute item does not leak the attribute",
			src: `#[contractimpl]
impl A {
    pub fn a(env: Env) {}
}

impl B {
    pub fn b(env: Env) {}
}`,
			want: []string{"a"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := names(parse(t, tc.src).ContractFns())
			if len(got) != len(tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("got %v, want %v", got, tc.want)
				}
			}
		})
	}
}

func TestContractFnBodyIsTheBlock(t *testing.T) {
	c := parse(t, `#[contractimpl]
impl C {
    pub fn a(env: Env) { let x = 1; }
}`)
	fns := c.ContractFns()
	if len(fns) != 1 {
		t.Fatalf("got %d contract fns, want 1", len(fns))
	}
	if k := fns[0].Body.Kind(); k != "block" {
		t.Fatalf("Body.Kind() = %q, want %q", k, "block")
	}
}

func TestNewRegistryRejectsBadRules(t *testing.T) {
	ok := Meta{ID: "a", Severity: SeverityError, Summary: "s", Why: "w", Fix: "f"}
	tests := []struct {
		name  string
		metas []Meta
	}{
		{"empty id", []Meta{{Severity: SeverityError, Summary: "s", Why: "w", Fix: "f"}}},
		{"missing why", []Meta{{ID: "a", Severity: SeverityError, Summary: "s", Fix: "f"}}},
		{"unknown severity", []Meta{{ID: "a", Severity: "loud", Summary: "s", Why: "w", Fix: "f"}}},
		{"duplicate id", []Meta{ok, ok}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rs := make([]Rule, len(tc.metas))
			for i, m := range tc.metas {
				rs[i] = stubRule{m}
			}
			if _, err := NewRegistry(rs...); err == nil {
				t.Fatal("want error, got nil")
			}
		})
	}
	if _, err := NewRegistry(stubRule{ok}); err != nil {
		t.Fatalf("valid rule rejected: %v", err)
	}
}

type stubRule struct{ meta Meta }

func (s stubRule) Meta() Meta               { return s.meta }
func (s stubRule) Check(*Context) []Finding { return nil }
