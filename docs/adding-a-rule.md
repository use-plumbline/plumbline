# Adding a rule

Most contributions to Plumbline are new rules, and a rule is deliberately a small
change: **one file, one line in a list, two fixtures.** You should not need to
read the rest of the codebase.

This guide is everything you need. If something here is wrong or missing, that is
a bug worth reporting.

## Before you start

```sh
make build   # binary into bin/
make test    # the suite, with the race detector
make lint    # gofmt, go vet, golangci-lint
make run     # lint the sample contract in testdata/
```

You need Go 1.23+ and a C compiler (the Rust grammar is linked through cgo).
**No Rust toolchain is required** to build or test Plumbline — it reads Rust, it
does not compile it.

## Work an existing rule as your template

**Start by reading [`rules/missing_auth.go`](../rules/missing_auth.go).** It is
the most complete of the three and shows nearly every technique you will need:
walking a function body, recognising a method call, following a call chain, and
following calls into helper functions.

Pick whichever shipped rule is closest to what you are writing:

| If your rule… | Copy the shape of |
| --- | --- |
| inspects storage reads/writes, or follows calls into helpers | [`rules/missing_auth.go`](../rules/missing_auth.go) |
| matches specific method or macro names | [`rules/panic_in_contract.go`](../rules/panic_in_contract.go) |
| needs to know an expression's type or width | [`rules/unchecked_arithmetic.go`](../rules/unchecked_arithmetic.go) |

Rules never import one another — see the package doc on
[`rules/rules.go`](../rules/rules.go). Copy the shape, not the import.

## 1. The rule file

Rules live in [`rules/`](../rules/), one per file, named after the rule ID:
`missing-reinit-guard` → `rules/missing_reinit_guard.go`.

A rule implements two methods:

```go
type Rule interface {
    Meta() Meta
    Check(*Context) []Finding
}
```

### `Meta` — the static half

```go
type MissingReinitGuard struct{}

func (MissingReinitGuard) Meta() rule.Meta {
    return rule.Meta{
        ID:       "missing-reinit-guard",   // stable kebab-case; never changes once shipped
        Severity: rule.SeverityError,       // a DEFAULT — configuration may override it
        Summary:  "An initializer can be called more than once.",
        Why:      "What goes wrong on chain if this is ignored.",
        Fix:      "The concrete remedy.",
    }
}
```

`Summary`, `Why` and `Fix` are **all required**. `NewRegistry` in
[`internal/rule/registry.go`](../internal/rule/registry.go) rejects a rule
missing any of them, so an undocumented rule fails the build rather than
shipping. They are what `plumbline --explain <rule>` prints.

### `Check` — the working half

```go
func (MissingReinitGuard) Check(c *rule.Context) []rule.Finding {
    var out []rule.Finding
    for _, fn := range c.ContractFns() {
        fn.Body.Walk(func(n rule.Node) bool {
            if call, ok := rule.AsMethodCall(n); ok && call.Name == "set" {
                out = append(out, rule.At(call.Field,
                    "%s writes %s without checking whether it is already set", fn.Name, "…"))
            }
            return true // return false to skip this node's children
        })
    }
    return out
}
```

`Check` returns **no error** on purpose: a rule inspecting an in-memory tree has
nothing to fail at. I/O and parse failures are the engine's problem. It also does
no I/O, holds no state between files, and never sees another rule — that is what
lets rules run in any order and be tested one at a time.

## 2. Register it

One line in `all()` in [`rules/rules.go`](../rules/rules.go):

```go
func all() []rule.Rule {
	return []rule.Rule{
		MissingAuth{},
		MissingReinitGuard{},   // <- yours
		PanicInContract{},
		UncheckedArithmetic{},
	}
}
```

That is the whole registration mechanism. It is an explicit list rather than
`init()` side effects, so adding a rule stays a diff a reviewer can read in full.
`Default()` builds the registry from `all()`, the CLI and the engine use that
registry, and `TestRuleFixtures` iterates it — so this one line is what makes the
engine run your rule *and* what makes the test harness demand your fixtures.

## 3. What `Context` gives you

| | |
| --- | --- |
| `c.Path` | the file path, for output |
| `c.Root` | the file's root node |
| `c.ContractFns()` | the contract's entry points — computed once, cached |

`ContractFns()` returns every `pub fn` inside an `impl` block annotated
`#[contractimpl]`, as `ContractFn{Name, Node, Body}`.

**Use it.** Almost every rule starts with "for each contract entry point", and
getting that set right is fiddlier than it looks: in Rust's grammar
`#[contractimpl]` is a **preceding sibling** of the impl block rather than a
child, attributes stack, and they can be path-qualified
(`#[soroban_sdk::contractimpl]`). That is centralized and tested once in
[`internal/rule/contract.go`](../internal/rule/contract.go) so it is not
re-derived and re-broken per rule.

Scoping to `ContractFns()` also excludes `#[cfg(test)]` modules and plain `impl`
blocks for free — test code is not contract code.

## 4. Navigating the tree

Rules see `rule.Node`, Plumbline's own view of the syntax tree. **tree-sitter is
not visible and must not be imported** — a test asserts it stays confined to
[`internal/syntax`](../internal/syntax/).

```go
n.Kind()                  // "call_expression", "binary_expression", ...
n.Text()                  // the source it spans
n.Line(), n.Column()      // 1-indexed
n.Field("left")           // (Node, bool) — child by grammar field name
n.Children()              // named children, in source order
n.Child(0)                // (Node, bool) — i-th named child
n.PrevSibling()           // (Node, bool) — how you reach attributes
n.Walk(func(rule.Node) bool)
```

An absent node is the zero `Node`. Accessors on it return zero values rather than
panicking, so you can chain lookups and check `Valid()` once at the end.

Helpers in [`internal/rule/ast.go`](../internal/rule/ast.go) cover the shapes
rules keep needing:

| Helper | For |
| --- | --- |
| `rule.AsMethodCall(n)` | `receiver.name(args)` — gives `Name`, `Recv`, `Field`, `Node` |
| `rule.AsPlainCall(n)` | `name(args)` or `Path::name(args)` |
| `rule.MacroName(n)` | `panic!(...)` → `"panic"` |
| `rule.LocalFns(root)` | every function in the file, by name — for following a call into a helper |
| `rule.HasAttribute(n, "contracttype")` | outer attributes on an item |
| `rule.At(node, format, args...)` | build a `Finding` positioned at a node |

**Anchor findings precisely.** In a chain like
`env.storage().persistent().set(k, v)` the call expression starts back at `env`,
so reporting at `call.Node` puts the annotation several lines above the call you
mean. Report at `call.Field` — the method name.

## 5. Seeing the AST you are matching

You cannot write a matcher without knowing the node kinds and field names. Do not
guess them — print the tree.

Drop this in `rules/scratch_test.go`, edit `src` to the Rust you care about, run
it, then **delete the file before committing**:

```go
package rules

import (
	"fmt"
	"testing"

	"github.com/use-plumbline/plumbline/internal/syntax"
)

func TestDumpTree(t *testing.T) {
	src := `#[contractimpl]
impl C {
    pub fn f(env: Env, amount: i128) {
        env.storage().instance().set(&DataKey::Total, &amount);
    }
}`
	p, err := syntax.NewParser()
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	tree, err := p.Parse([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	defer tree.Close()

	var dump func(n syntax.Node, depth int)
	dump = func(n syntax.Node, depth int) {
		fmt.Printf("%*s%-24s %q\n", depth*2, "", n.Kind(), n.Text())
		for _, c := range n.Children() {
			dump(c, depth+1)
		}
	}
	dump(tree.Root(), 0)
}
```

```sh
go test ./rules/ -run TestDumpTree -v
```

which prints:

```
source_file              "#[contractimpl]\nimpl C {\n    pub fn f…"
  attribute_item           "#[contractimpl]"
    attribute                "contractimpl"
      identifier               "contractimpl"
  impl_item                "impl C {\n    pub fn f(env: Env, amount: i128) {…"
    type_identifier          "C"
    declaration_list         "{\n    pub fn f…"
      function_item            "pub fn f(env: Env, amount: i128) {…"
        visibility_modifier      "pub"
        identifier               "f"
        parameters               "(env: Env, amount: i128)"
          parameter                "env: Env"
            identifier               "env"
            type_identifier          "Env"
```

Note what this shows: `attribute_item` is a **sibling** of `impl_item`, not a
child. That is the kind of thing you only learn by looking.

`Children()` shows named nodes only. Field names (`left`, `operator`, `right`,
`pattern`, `type`, `value`, `function`, `arguments`) do not appear in this dump —
find them in the
[tree-sitter-rust grammar](https://github.com/tree-sitter/tree-sitter-rust/blob/master/grammar.js),
or try `n.Field("left")` and check the `bool`.

## 6. Fixtures

Every rule needs both:

```
testdata/rules/<rule-id>/pass.rs   # the rule must find nothing
testdata/rules/<rule-id>/fail.rs   # the rule must fire, on exactly the marked lines
```

`TestRuleFixtures` in [`rules/fixture_test.go`](../rules/fixture_test.go) is
driven off the registry, so **a rule added without both fixtures fails the
build.** There is no way to skip this.

In `fail.rs`, mark each expected finding with `//~` and the rule ID **on the line
the finding will be reported at**:

```rust
pub fn set_admin(env: Env, new_admin: Address) { //~ missing-auth
    env.storage().instance().set(&DataKey::Admin, &new_admin);
}
```

The harness asserts the reported lines are **exactly** the marked lines, so a
rule that fires on the right file for the wrong reason still fails. The
convention is borrowed from rustc's UI tests.

Fixtures must be valid Rust — the engine skips files that do not parse, and the
harness treats a skipped fixture as a failure.

### Run just your rule's fixtures

```sh
go test ./rules/ -run 'TestRuleFixtures/missing-reinit-guard' -v
```

and just one side of it:

```sh
go test ./rules/ -run 'TestRuleFixtures/missing-reinit-guard/fail' -v
```

The failure message tells you which lines fired versus which you marked:

```
fixture_test.go:55: findings on lines [26 32], expected [30 32]
```

### Make the pass fixture argue

The pass fixture is where a rule's precision is documented. Put the cases a naive
implementation gets wrong in it, with a comment saying why each is **not** a
finding.

[`testdata/rules/missing-auth/pass.rs`](../testdata/rules/missing-auth/pass.rs)
contains a constructor, a read-only getter, auth reached through a helper,
`require_auth_for_args`, and a TTL extension — each one a false positive that a
careless rewrite would reintroduce. That file is the rule's real specification.

## 7. Verify SDK behaviour, do not recall it

Rules encode claims about `soroban-sdk`. Before you encode one, check it against
[docs.rs/soroban-sdk](https://docs.rs/soroban-sdk) (currently **27.0.5**) or the
[Stellar docs](https://developers.stellar.org/docs/build/smart-contracts), and
say in a comment what you checked.

This matters more than it sounds. `missing-auth` does **not** treat `extend_ttl`
as a state mutation, because any account can extend any entry's TTL with
`ExtendFootprintTTLOp` without the contract's involvement — so a TTL bump carries
no authority. Getting that wrong would have made the rule fire on every
well-written contract.

Previously verified facts are recorded with dates in
[docs/sessions/2026-08-08-session-1.md](sessions/2026-08-08-session-1.md).

## 8. Checklist

- [ ] `rules/your_rule.go`, importing only `internal/rule`
- [ ] Registered in `all()` in `rules/rules.go`
- [ ] `Meta` has ID, severity, summary, why, fix
- [ ] `testdata/rules/<id>/pass.rs` with the tricky non-findings, commented
- [ ] `testdata/rules/<id>/fail.rs` with `//~ <id>` markers on the right lines
- [ ] Finding message is actionable and names what to do
- [ ] SDK claims verified against docs.rs, with a comment saying so
- [ ] Any scratch/debug file deleted
- [ ] `make lint && make test`

## A rule that earns its noise

One thing worth saying plainly: **a rule that fires on idiomatic contracts is
worse than no rule**, because it teaches people to ignore Plumbline.

CI enforces this. The `action` job in
[`.github/workflows/ci.yml`](../.github/workflows/ci.yml) lints
[`testdata/sample-contract`](../testdata/sample-contract/) — a real, compiling
Soroban vault — and **fails if any rule reports anything at all** on it. If your
rule fires there, either the sample contract has a genuine defect or your rule
does. Work out which before working around it.

If you cannot keep the false-positive rate low, narrow the rule's scope until you
can, and say in the PR what you narrowed it to and why.
