# The GitHub Action

Plumbline runs as a composite action. It builds the linter from its own checkout
and runs it against your workspace, so there is nothing to install and no
container to pull.

```yaml
name: Lint contracts

on: [pull_request]

permissions:
  contents: read

jobs:
  plumbline:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v7
      - uses: use-plumbline/plumbline@main
        with:
          paths: contracts/
```

Findings appear as inline annotations on the changed files in the pull request.

## Inputs

| Input | Default | Description |
| --- | --- | --- |
| `paths` | `.` | Files or directories to lint, whitespace or newline separated. Directories are walked for `.rs` sources; `target/`, `node_modules/` and `.git/` are skipped. |
| `fail-on` | `error` | Lowest severity that fails the run: `error`, `warning`, `note`, or `never`. Findings below the threshold are still annotated. |
| `format` | `github` | `github` emits the workflow commands that become annotations. `text` is easier to read when debugging a workflow. |

## Outputs

| Output | Description |
| --- | --- |
| `findings` | Number of findings reported. |
| `exit-code` | The linter's exit code. |

## Exit codes

| Code | Meaning |
| --- | --- |
| `0` | Nothing at or above the `fail-on` threshold |
| `1` | Findings at or above the threshold |
| `2` | Plumbline could not run — a bad flag, or a path that does not exist |

Exit `2` is a workflow problem rather than a finding, and is passed through
unchanged so a mistyped path fails loudly instead of looking like a clean run.

## Recipes

**Annotate everything, fail on nothing.** Useful when adopting Plumbline on an
existing contract: you get the full picture without a red build on day one.

```yaml
- uses: use-plumbline/plumbline@main
  with:
    paths: contracts/
    fail-on: never
```

**Hold a new contract to a higher bar** by failing on warnings too:

```yaml
- uses: use-plumbline/plumbline@main
  with:
    paths: contracts/vault
    fail-on: warning
```

**Several directories:**

```yaml
- uses: use-plumbline/plumbline@main
  with:
    paths: |
      contracts/vault
      contracts/router
```

**Act on the result in a later step:**

```yaml
- uses: use-plumbline/plumbline@main
  id: lint
  with:
    paths: contracts/
    fail-on: never

- run: echo "Plumbline reported ${{ steps.lint.outputs.findings }} finding(s)."
```

## Notes

- **Pin the version.** `@main` moves. Once Plumbline tags releases, prefer a tag
  or a commit SHA.
- **Annotations need the workflow's own log.** The action writes workflow
  commands to stdout, which the runner turns into annotations. It does not call
  the GitHub API and needs no token beyond the default `contents: read`.
- **Annotations only render inline on lines the pull request touched.** Findings
  elsewhere in the file still appear in the run's annotation list and in the
  step log.
- **No Rust toolchain is required.** Plumbline parses contract source rather
  than compiling it.
