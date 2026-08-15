package config

import "testing"

func TestExcluded(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		path    string
		want    bool
	}{
		// A directory name excludes everything beneath it. This is the
		// case most people reach for first, and getting it wrong would
		// silently lint the directory they asked to skip.
		{"bare directory name", "vendor", "vendor/a/lib.rs", true},
		{"directory with a glob tail", "vendor/**", "vendor/a/lib.rs", true},
		{"directory does not match a sibling prefix", "vendor", "vendored/lib.rs", false},
		{"nested directory path", "contracts/vendor", "contracts/vendor/src/lib.rs", true},
		{"nested directory path is anchored", "contracts/vendor", "other/contracts/vendor/lib.rs", false},
		{"leading ** unanchors it", "**/vendor/**", "other/contracts/vendor/lib.rs", true},

		// `*` stays inside one segment; `**` crosses them. That is the
		// distinction a user relies on to write **/test.rs and not have
		// it mean "any file called test.rs at the top level only".
		{"star matches within a segment", "*.rs", "lib.rs", true},
		{"star does not cross a separator", "*.rs", "src/lib.rs", false},
		{"double star crosses separators", "**/*.rs", "a/b/c/lib.rs", true},
		{"double star matches no segments at all", "**/lib.rs", "lib.rs", true},
		{"star in the middle", "contracts/*/src/lib.rs", "contracts/vault/src/lib.rs", true},
		{"star in the middle spans one segment only", "contracts/*/lib.rs", "contracts/a/b/lib.rs", false},

		{"exact file", "src/test.rs", "src/test.rs", true},
		{"suffix glob under any directory", "**/test.rs", "contracts/vault/src/test.rs", true},
		{"no match", "**/test.rs", "contracts/vault/src/lib.rs", false},

		// Both sides are normalised, so a pattern copied out of a shell
		// with a trailing slash still works.
		{"trailing slash on the pattern", "vendor/", "vendor/lib.rs", true},
		{"leading slash on the pattern", "/vendor", "vendor/lib.rs", true},
		{"dot segments in the path", "vendor", "./vendor/lib.rs", true},
		{"absolute path", "**/vendor/**", "/srv/repo/vendor/lib.rs", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := Excluded([]string{tc.pattern}, tc.path); got != tc.want {
				t.Errorf("Excluded([%q], %q) = %v, want %v", tc.pattern, tc.path, got, tc.want)
			}
		})
	}
}

func TestExcludedTakesAnyPattern(t *testing.T) {
	patterns := []string{"vendor", "**/test.rs"}
	for _, p := range []string{"vendor/lib.rs", "contracts/vault/src/test.rs"} {
		if !Excluded(patterns, p) {
			t.Errorf("%q was not excluded by %v", p, patterns)
		}
	}
	if Excluded(patterns, "contracts/vault/src/lib.rs") {
		t.Error("a file matching neither pattern was excluded")
	}
	if Excluded(nil, "anything.rs") {
		t.Error("an empty pattern list excluded a file")
	}
}
