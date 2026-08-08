# Writing a rule

A rule is one file in [`rules/`](../rules/), one line in `all()`, and two
fixtures. It imports `internal/rule` and nothing else from the project.

## The interface

```go
type Rule interface {
    Meta() Meta
    Check(*Context) []Finding
}
```

`Meta` is static — the CLI lists and documents rules without parsing anything:

```go
func (YourRule) Meta() rule.Meta {
    return rule.Meta{
        ID:       "your-rule",          // stable kebab-case; never changes once shipped
        Severity: rule.SeverityWarning, // a DEFAULT; config may override it
        Summary:  "One line: what this looks for.",
        Why:      "What goes wrong on chain if it is ignored.",
        Fix:      "The concrete remedy.",
    }
}
```

`Summary`, `Why` and `Fix` are all required — the registry rejects a rule
missing any of them, so an undocumented rule fails the build rather than
shipping.

`Check` receives one already-parsed file:

```go
func (YourRule) Check(c *rule.Context) []rule.Finding {
    var out []rule.Finding
    for _, fn := range c.ContractFns() {
        fn.Body.Walk(func(n rule.Node) bool {
            if call, ok := rule.AsMethodCall(n); ok && call.Name == "something" {
                out = append(out, rule.At(call.Field, "%s calls something()", fn.Name))
            }
            return true // return false to skip this node's children
        })
    }
    return out
}
```

`Check` returns no error on purpose: a rule inspecting an in-memory tree has
nothing to fail at. I/O and parse failures are the engine's problem.

It also does no I/O, holds no state between files, and never sees another rule.
That is what lets rules run in any order and be tested one at a time.

## What `Context` gives you

| | |
| --- | --- |
| `c.Path` | the file path, for output |
| `c.Root` | the file's root node |
| `c.ContractFns()` | the contract's entry points — computed once, cached |

`ContractFns()` returns every `pub fn` inside an `impl` block annotated
`#[contractimpl]`, as `ContractFn{Name, Node, Body}`.

Use it. Almost every rule starts with "for each contract entry point", and
getting that set right is fiddlier than it looks: in Rust's grammar
`#[contractimpl]` is a **preceding sibling** of the impl block, not a child of
it, attributes stack, and they can be path-qualified. That logic is centralized
and tested once so it is not re-derived (and re-broken) per rule.

Scoping to `ContractFns()` also excludes `#[cfg(test)]` modules and plain `impl`
blocks for free — test code is not contract code.

## Navigating the tree

Rules see `rule.Node`, Plumbline's own view of the syntax tree. Tree-sitter is
not visible and must not be imported.

```go
n.Kind()                  // "call_expression", "binary_expression", ...
n.Text()                  // the source it spans
n.Line(), n.Column()      // 1-indexed
n.Field("left")           // (Node, bool) — child by grammar field name
n.Children()              // named children, in source order
n.PrevSibling()           // (Node, bool) — how you reach attributes
n.Walk(func(rule.Node) bool)
```

An absent node is the zero `Node`. Accessors on it return zero values rather
than panicking, so you can chain lookups and check `Valid()` once at the end.

Helpers in `internal/rule` cover the shapes rules keep needing:

| Helper | For |
| --- | --- |
| `rule.AsMethodCall(n)` | `receiver.name(args)` — gives `Name`, `Recv`, `Field`, `Node` |
| `rule.AsPlainCall(n)` | `name(args)` or `Path::name(args)` |
| `rule.MacroName(n)` | `panic!(...)` → `"panic"` |
| `rule.LocalFns(root)` | every function in the file, by name — for following a call into a helper |
| `rule.HasAttribute(n, "contracttype")` | outer attributes on an item |
| `rule.At(node, format, args...)` | build a `Finding` positioned at a node |

**Anchor findings precisely.** In a chain like
`env.storage().persistent().set(k, v)`, the call expression starts back at
`env`, so reporting at `call.Node` puts the annotation several lines above the
call you mean. Report at `call.Field` — the method name.

To learn what the tree looks like for some Rust, print it:

```go
n.Walk(func(n rule.Node) bool { fmt.Println(n.Kind(), n.Text()); return true })
```

## Fixtures

Every rule needs both:

```
testdata/rules/<rule-id>/pass.rs   # the rule must find nothing
testdata/rules/<rule-id>/fail.rs   # the rule must fire, on exactly the marked lines
```

The harness in [`rules/fixture_test.go`](../rules/fixture_test.go) is driven off
the registry, so a rule added without fixtures fails the build.

In `fail.rs`, mark each expected finding with `//~` and the rule ID **on the
line the finding will be reported at**:

```rust
pub fn set_admin(env: Env, new_admin: Address) { //~ missing-auth
    env.storage().instance().set(&DataKey::Admin, &new_admin);
}
```

The harness asserts the reported lines are exactly the marked lines, so a rule
that fires on the right file for the wrong reason still fails. The convention is
borrowed from rustc's UI tests.

Fixtures must be valid Rust — the engine skips files that do not parse, and the
harness treats a skipped fixture as a failure.

### Make the pass fixture argue

The pass fixture is where a rule's precision is documented. Put the cases a
naive implementation gets wrong in it, with a comment saying why each is not a
finding. `missing-auth`'s pass fixture contains a constructor, a read-only
getter, auth reached through a helper, `require_auth_for_args`, and a TTL
extension — each one a false positive that a careless rewrite would reintroduce.

## Checklist

- [ ] `rules/your_rule.go`, importing only `internal/rule`
- [ ] Registered in `all()` in `rules/rules.go`
- [ ] `Meta` has ID, severity, summary, why, fix
- [ ] `testdata/rules/<id>/pass.rs` with the tricky non-findings, commented
- [ ] `testdata/rules/<id>/fail.rs` with `//~ <id>` markers
- [ ] SDK claims verified against docs.rs, with a comment saying so
- [ ] `make lint && make test`
