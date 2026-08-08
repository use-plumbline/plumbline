# Contributing to Plumbline

Most contributions to Plumbline are **new rules**, and a rule is deliberately a
small, self-contained change: one file, one line in a list, two fixtures. This
guide is mostly about that path.

## Getting set up

You need Go 1.23 or newer. The Rust grammar is compiled through cgo, so you
also need a C compiler (`gcc` or `clang`) — no Rust toolchain is required to
build or test Plumbline itself.

```sh
make build   # binary into bin/
make test    # the suite, with the race detector
make lint    # gofmt, go vet, golangci-lint
make run     # lint the sample contract in testdata/
```

## Adding a rule

Rules live in [`rules/`](rules/), one per file, and no rule may import another.
Adding one means three things:

1. **`rules/your_rule.go`** — a type implementing [`rule.Rule`](internal/rule/rule.go):

   ```go
   type YourRule struct{}

   func (YourRule) Meta() rule.Meta { /* ID, Severity, Summary, Why, Fix */ }
   func (YourRule) Check(c *rule.Context) []rule.Finding { /* ... */ }
   ```

2. **One line in `all()`** in [`rules/rules.go`](rules/rules.go).

3. **Two fixtures**, at `testdata/rules/<your-rule-id>/pass.rs` and
   `fail.rs`. These are not optional — the test harness is driven off the
   registry, so a rule without both fixtures fails the build.

See [docs/adding-a-rule.md](docs/adding-a-rule.md) for the details: what
`Context` gives you, how to navigate the syntax tree, and the conventions the
fixtures follow.

### What a good rule looks like

- **It earns its noise.** A rule that fires on idiomatic contracts is worse
  than no rule, because it teaches people to ignore Plumbline. If you cannot
  keep the false-positive rate low, narrow the rule's scope until you can, and
  say what you narrowed it to.
- **It says why, and how to fix it.** `Meta.Why` explains what goes wrong on
  chain; `Meta.Fix` names the concrete remedy. Both are required.
- **Its fixtures argue the case.** The pass fixture should contain the cases a
  naive implementation would get wrong — that is what stops the next person
  from "simplifying" the rule back into a false-positive generator.

## Verify SDK behaviour, do not recall it

Rules encode claims about `soroban-sdk`. Before you encode one, check it
against [docs.rs/soroban-sdk](https://docs.rs/soroban-sdk) or the
[Stellar docs](https://developers.stellar.org/docs/build/smart-contracts), and
say in a comment what you checked and when.

This matters more than it sounds. `missing-auth` does not treat `extend_ttl` as
a state mutation, because any account can extend any entry's TTL with
`ExtendFootprintTTLOp` without the contract's involvement. That is the kind of
detail that separates a useful rule from an annoying one, and you only get it
from the source.

## Working on the parser boundary

Rules never import tree-sitter. [`internal/syntax`](internal/syntax/) wraps it
and exposes `Node`; `rule.Node` aliases that type. If you find yourself wanting
a tree-sitter API inside a rule, add the accessor you need to `internal/syntax`
instead — a test asserts the parser stays confined there.

## Commits and pull requests

- Commit messages explain **why**, not just what. If a decision has a
  non-obvious reason, that reason belongs in the message.
- Keep commits incremental and honest. One logical change per commit.
- Put the pull request description in the pull request, not in a committed
  file.
- `make lint && make test` must pass before you open a PR. CI runs the same
  things.

## License

Contributions are accepted under the [Apache-2.0](LICENSE) license.
