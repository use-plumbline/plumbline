package rules

import (
	"testing"

	"github.com/use-plumbline/plumbline/internal/rule"
	"github.com/use-plumbline/plumbline/internal/syntax"
)

func TestContractmetaMissingFocusedCases(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want int
	}{
		{
			name: "helper only",
			src:  `fn helper() {}`,
			want: 0,
		},
		{
			name: "implementation only",
			src: `#[contractimpl]
impl External {
    pub fn ping() {}
}`,
			want: 0,
		},
		{
			name: "authored metadata",
			src: `contractmeta!(key = "desc", val = "Example contract");

#[contract]
pub struct Example;`,
			want: 0,
		},
		{
			name: "declaration and implementation produce one finding",
			src: `#[contract]
pub struct Example;

#[contractimpl]
impl Example {
    pub fn ping() {}
}`,
			want: 1,
		},
		{
			name: "misleading comment and string do not suppress finding",
			src: `// contractmeta!(key = "desc", val = "comment")
const TEXT: &str = "contractmeta!";

#[contract]
pub struct Example;`,
			want: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := runContractmetaMissing(t, tt.src)
			if len(got) != tt.want {
				t.Fatalf("got %d findings, want %d: %#v", len(got), tt.want, got)
			}
		})
	}
}

func runContractmetaMissing(t *testing.T, src string) []rule.Finding {
	t.Helper()
	p, err := syntax.NewParser()
	if err != nil {
		t.Fatalf("new parser: %v", err)
	}
	defer p.Close()
	tree, err := p.Parse([]byte(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	defer tree.Close()

	return ContractmetaMissing{}.Check(&rule.Context{Path: "test.rs", Root: tree.Root()})
}
