# Releasing Plumbline

Plumbline is consumed as `use-plumbline/plumbline@vX.Y.Z` in someone else's
workflow. A tag is therefore not a bookmark — it is the artifact. This is how
one gets cut and what it promises.

## What a version means

Plumbline is at `0.x`, which means the interfaces below are stable **within** a
minor version and may change between them. `1.0.0` is when that stops.

**Patch** — `v0.1.0` → `v0.1.1`

- Bug fixes, including narrowing a rule that was firing on correct code.
- A rule reporting *fewer* findings.
- Documentation, internals, dependencies.

A patch release never introduces a finding a workflow did not have before.
Someone pinned to `v0.1` can take it without reading anything.

**Minor** — `v0.1.0` → `v0.2.0`

- A new rule. Any new rule can fail a build that was green, so it is always at
  least a minor.
- A rule reporting *more* findings than before.
- A new CLI flag, action input, or output field.
- A change to a configuration key, a rule ID, an exit code, or the JSON schema.
- Raising the minimum Go version.

**Major** — reserved for `1.0.0`, when the rule IDs, the configuration schema,
the JSON schema, the exit codes and the action inputs become things that only
change on a major.

### What is covered

Stable within a minor version:

- **Exit codes.** `0` clean, `1` findings at or above the threshold, `2` could
  not run. This is the action's contract with the workflow.
- **Rule IDs.** A shipped rule ID does not change; it is what `.plumbline.toml`
  and every suppression is written against.
- **Action inputs and outputs.**
- **Configuration keys** in `.plumbline.toml`.
- **The JSON schema**, which additionally carries its own `schemaVersion` — see
  [docs/json-output.md](docs/json-output.md).

Not covered, and deliberately so:

- **The exact wording of a finding.** Messages get clearer over time. Match on
  the rule ID.
- **The text output format.** It is for a person to read.
- **Which lines a rule reports**, beyond the version rules above. That is the
  rule getting better.
- **Every Go package under `internal/`.** Plumbline is a tool, not a library.

## Cutting a release

Releases are cut from `main`, which must be green.

1. **Check `main` is green.** CI runs `gofmt`, `go vet`, `go build`,
   `go test -race`, `golangci-lint`, the action against its own sample
   contract, and `cargo test` on that contract.

   ```sh
   make lint && make test
   ```

2. **Run against real contracts.** The rules exist to be trusted, and the
   only way to know a change did not start firing on idiomatic Soroban is to
   point it at some. Build the binary and run it over a few third-party
   workspaces — [stellar/soroban-examples][examples] and
   [OpenZeppelin/stellar-contracts][oz] are the standing corpus:

   ```sh
   make build
   ./bin/plumbline --fail-on never /path/to/soroban-examples
   ```

   Read every finding. Anything the rules got wrong about correct code is a
   blocker, not a known issue: a false positive is how a linter gets
   uninstalled permanently. Record what you ran it against in the release
   notes.

3. **Tag it.** Annotated, on `main`, never moved once pushed.

   ```sh
   git tag -a v0.1.0 -m "Plumbline v0.1.0"
   git push origin v0.1.0
   ```

   The build stamps `--version` from `git describe`, so the tag is what the
   binary reports.

4. **Publish the release** with notes that say what the rules catch, what they
   do **not** catch, and what is known to be wrong with them.

   ```sh
   gh release create v0.1.0 --title "v0.1.0" --notes-file notes.md
   ```

5. **Verify by tag from outside.** Referencing the action from its own
   repository exercises a different path than referencing it by tag from
   elsewhere. Put a workflow in a scratch repository, run it, and confirm the
   annotations arrive:

   ```yaml
   - uses: use-plumbline/plumbline@v0.1.0
     with:
       paths: contracts/
   ```

6. **Update the `uses:` snippets** in [README.md](README.md) and
   [docs/](docs/) to the new tag.

## Release notes

The notes are the only thing most people will read. They should say:

- **What it catches** — the rules, by ID, in one line each.
- **What it does not catch.** A clean Plumbline run means the rules it has did
  not fire. Anyone who reads it as "this contract is safe" has been misled, and
  the notes are where that is prevented.
- **Known limitations** — the cases where a rule is known to be wrong, or to
  stay quiet when it should not. Every one of these is worth more than a
  paragraph of what does work.
- **Upgrading**, when something changed that a workflow will notice.

Do not describe coverage the rules do not have. Three rules are three rules.

## Moving major tags

Plumbline does not publish or move a floating `v0` or `v1` tag. `@main` moves
and is not a release. Pin to a full version — `@v0.1.0` — or to a commit SHA.

[examples]: https://github.com/stellar/soroban-examples
[oz]: https://github.com/OpenZeppelin/stellar-contracts
