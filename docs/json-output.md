# JSON output

`--format json` writes one JSON document to stdout, for tooling that wants to
do something with the findings other than print them.

```sh
plumbline --format json --fail-on never contracts/ | jq '.summary'
```

Nothing else goes to stdout in this mode, so piping into `jq` is safe even when
a file fails to parse — errors and usage messages go to stderr.

## The document

```json
{
  "schemaVersion": 1,
  "findings": [
    {
      "rule": "missing-auth",
      "severity": "error",
      "file": "contracts/vault/src/lib.rs",
      "line": 42,
      "column": 12,
      "message": "set_admin writes storage but no path through it calls require_auth (write at line 44)"
    }
  ],
  "skipped": [
    { "file": "contracts/vault/src/broken.rs", "reason": "could not be parsed as Rust" }
  ],
  "summary": {
    "filesLinted": 12,
    "findings": 1,
    "errors": 1,
    "warnings": 0,
    "notes": 0
  }
}
```

| Field | Meaning |
| --- | --- |
| `schemaVersion` | The shape of this document. See [compatibility](#compatibility). |
| `findings[].rule` | Rule ID, as in `plumbline --list-rules`. |
| `findings[].severity` | `error`, `warning` or `note`, after any `.plumbline.toml` override. |
| `findings[].file` | Path as Plumbline was given it. |
| `findings[].line`, `.column` | 1-indexed position. |
| `findings[].message` | The finding, in one sentence. |
| `skipped[]` | Files Plumbline declined to lint, and why. |
| `summary.filesLinted` | Files parsed and checked. Excludes skipped files. |
| `summary.findings` | Length of `findings`. |
| `summary.errors`, `.warnings`, `.notes` | Counts by severity. |

`findings` and `skipped` are always arrays. An empty run gives `[]`, never
`null`.

**Read `skipped`.** A run that could not parse the contract it was pointed at
is not a clean run, and a consumer counting only `findings` would read it as
one.

Findings come back sorted by file, then line, then column, then rule ID, so
diffing two runs shows what changed rather than what moved.

## Exit codes

`--format json` does not change them: `0` clean, `1` findings at or above the
`--fail-on` threshold, `2` Plumbline could not run. Pass `--fail-on never` when
you want the document and not the verdict.

## Compatibility

`schemaVersion` describes this document, not Plumbline. The two move for
different reasons — new rules do not change these field names, and a rename
here would matter to a parser even in a release that shipped no rules.

It is incremented when an existing field changes meaning or disappears.
**Adding a field does not increment it**, so ignore fields you do not
recognise.

## From the action

```yaml
- uses: use-plumbline/plumbline@v0.1.0
  id: lint
  with:
    paths: contracts/
    format: json
    fail-on: never
```

The `findings` output is only populated for `format: github`; in JSON mode,
read the document from the step log or write it to a file in a later step.

SARIF — which would put findings in GitHub's security tab — is a separate
format and is not implemented.
