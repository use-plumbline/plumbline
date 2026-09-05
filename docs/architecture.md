# Architecture

Plumbline is written in Go. It *reads* Soroban contracts, which are Rust; it is
not itself Rust. The only Rust in this repository is test fixtures and the
sample contract.

## The pieces

```
cmd/plumbline/      CLI: flags, exit codes
  ├── internal/config/    .plumbline.toml — severity, enablement, excludes
  ├── internal/engine/    find files, parse, run rules, collect findings
  │     ├── internal/syntax/   the parser, wrapped
  │     └── internal/rule/     the Rule interface, Context, registry
  │           └── rules/       one file per rule
  └── internal/report/    render findings as text, JSON or GitHub annotations
```

Dependencies point one way. `rules/` imports `internal/rule` and nothing else
from the project; no rule imports another rule.

## Why tree-sitter

Rules need to answer structural questions. "Does any path through this function
reach a `require_auth` call?" is not a question you can ask a regular
expression, and neither is "is this `.set()` a storage write or some other
`.set()` that happens to be nearby".

Plumbline parses with [tree-sitter](https://tree-sitter.github.io/)'s Rust
grammar through the official Go bindings. tree-sitter is error-tolerant, needs
no Rust toolchain at lint time, and is fast enough to run on every push.

It is a *syntactic* parser, not a type checker. Plumbline sees names and shapes,
not resolved types — which is why `unchecked-arithmetic` infers integer widths
from parameter annotations and `let` bindings rather than asking a compiler.
Rules are written to be useful within that limit, and to say so where it bites.

## The parser boundary

Rules never see tree-sitter. `internal/syntax` wraps it and exposes `Node`:

```go
type Node interface-ish {
    Valid() bool
    Kind() string                    // "call_expression"
    Text() string                    // the source it spans
    Line() int                       // 1-indexed
    Column() int                     // 1-indexed
    Field(name string) (Node, bool)  // child by grammar field
    Children() []Node                // named children, source order
    PrevSibling() (Node, bool)
    Walk(visit func(Node) bool)
}
```

`rule.Node` aliases it, so a rule imports one package. Three things follow:

- **Swapping or upgrading the parser is a change to one package**, not to every
  rule. A test asserts tree-sitter is named nowhere else.
- **`Text()` takes no arguments.** A `Node` carries the source buffer it was
  parsed from, so no rule can read a node against the wrong bytes.
- **Positions are converted once.** tree-sitter counts rows and columns from
  zero; terminals and GitHub annotations count from one. The boundary converts,
  so no rule does off-by-one arithmetic.

An absent node is the zero `Node` rather than a nil pointer. Accessors on it
return zero values instead of panicking, so a rule can chain lookups and check
`Valid()` once at the end.

## The engine

`internal/engine` owns everything that can fail, so rules do not have to:

1. **Discovery.** Walk the given paths for `.rs` files, skipping `target/`,
   `node_modules/`, `.git/`, and Rust's test sources — `tests/`, `test/`,
   `test.rs`, `tests.rs`. The mock contracts in those are written to exercise
   one path and are never compiled into a deployed wasm, so holding them to the
   rules reports defects that cannot reach a ledger; they were 44 of the 174
   findings in the [first corpus run](corpus-run.md). A path naming a file is
   taken as given, so `plumbline src/test.rs` still does what it says.
2. **Parse.** One parser, reused across files.
3. **Check.** Build one `rule.Context` per file and hand it to every enabled
   rule, so parsing happens once regardless of rule count.
4. **Stamp.** Fill in each finding's path, rule ID and severity.
5. **Sort.** By path, line, column, rule ID, so output is deterministic.

**A file that does not parse is skipped, not linted.** A rule reading a broken
tree reports nonsense. Skips are recorded in the result and surfaced in output
rather than swallowed.

### Configuration

`.plumbline.toml` is implemented in [`internal/config`](../internal/config/), and
it landed through the seam this section used to describe as future work: the
engine holds severity overrides, a disabled set and a file-exclusion predicate,
all keyed by rule ID, and stamps severity onto findings itself.

```go
e := engine.New(registry)
e.SetSeverity("unchecked-arithmetic", rule.SeverityError)
e.Disable("panic-in-contract")
e.SetExcluded(func(path string) bool { ... })
```

The prediction held: **not one rule changed when configuration shipped.** A
rule's `Meta.Severity` is only a *default*, and no rule has an opinion about
where its settings came from.

The dependency runs one way — `Config.Apply(*engine.Engine)`, so config knows
about the engine and the engine knows nothing about config. Two consequences
worth stating:

- **Configuration cannot change what a rule looks for.** Severity, enablement
  and file selection are policy and live in `internal/config`. What counts as a
  finding stays in the rule. That boundary is why adding a config key never
  needs a rule to be re-reviewed.
- **An unknown rule ID in `[rules]` is a hard error**, not a silent no-op. A
  misspelt key would otherwise look exactly like a rule successfully switched
  off.

User-facing reference: [configuration.md](configuration.md). How the schema is
extended is [issue #34](https://github.com/use-plumbline/plumbline/issues/34).

## Severity

`internal/rule` defines three severities and deliberately does not rank them.
Ranking is a policy question about when to fail a build, so it lives in the CLI
where `--fail-on` is interpreted, and in `internal/report` where severities are
mapped to GitHub's annotation levels (which spell the lowest one `notice`).
