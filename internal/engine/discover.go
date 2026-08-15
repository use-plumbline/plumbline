package engine

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// skippedDirs are never descended into. Cargo's build output contains vendored
// and generated Rust that is not the contract under review, and linting it is
// slow and pure noise. `tests/` is Cargo's integration-test directory and
// `test/` is the usual name for a `#[cfg(test)] mod test;` submodule: the
// contracts declared in either are mocks that exist to be called by a test,
// and they are never compiled into a deployed wasm.
var skippedDirs = map[string]bool{
	"target":       true,
	"node_modules": true,
	".git":         true,
	"tests":        true,
	"test":         true,
}

// skippedFiles are the conventional names of a crate's unit-test module.
//
// The mock contracts in them are the largest single source of noise in a real
// workspace — they are written to exercise one path, not to be safe on chain,
// so a mock with no require_auth and an .unwrap() in it is correct code that
// every rule would report. Naming a file on the command line still lints it,
// so `plumbline src/test.rs` does what it says.
var skippedFiles = map[string]bool{
	"test.rs":  true,
	"tests.rs": true,
}

// DiscoverRust expands paths into a sorted, deduplicated list of .rs files.
//
// A path that names a file is taken as given, whatever its extension, so that
// `plumbline path/to/thing.rs` always does what it says. Directories are
// walked and filtered to .rs.
func DiscoverRust(paths []string) ([]string, error) {
	seen := map[string]bool{}
	var out []string
	add := func(p string) {
		if p = filepath.Clean(p); !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}

	for _, p := range paths {
		info, err := os.Stat(p)
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", p, err)
		}
		if !info.IsDir() {
			add(p)
			continue
		}
		err = filepath.WalkDir(p, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				if skippedDirs[d.Name()] {
					return fs.SkipDir
				}
				return nil
			}
			if strings.HasSuffix(d.Name(), ".rs") && !skippedFiles[d.Name()] {
				add(path)
			}
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("walking %s: %w", p, err)
		}
	}

	sort.Strings(out)
	return out, nil
}
