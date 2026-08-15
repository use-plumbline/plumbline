package config

import (
	"path"
	"path/filepath"
	"strings"
)

// Excluded reports whether any pattern excludes the file at p.
//
// Patterns are matched against the path as Plumbline discovered it — relative
// to the working directory when the command line said `contracts/`, absolute
// when it said `/srv/contracts` — using forward slashes on every platform.
//
//   - `*` matches any run of characters within one path segment.
//   - `**` matches any number of segments, including none.
//   - A pattern that matches a directory excludes everything beneath it, so
//     `vendor` and `vendor/**` both do the obvious thing.
//
// The last rule is why this is not one call to [path.Match]: excluding a
// directory is the common case, and a user who writes `exclude = ["vendor"]`
// and gets every file under vendor/ linted anyway has been told the feature
// works when it did not.
func Excluded(patterns []string, p string) bool {
	segments := strings.Split(filepath.ToSlash(filepath.Clean(p)), "/")
	for _, pattern := range patterns {
		pat := strings.Split(strings.Trim(filepath.ToSlash(pattern), "/"), "/")
		// A pattern may name the file itself or any directory above it.
		for i := 1; i <= len(segments); i++ {
			if matchSegments(pat, segments[:i]) {
				return true
			}
		}
	}
	return false
}

// matchSegments matches a split pattern against a split path.
//
// It recurses only on `**`, where the pattern has to try every split point of
// what is left; every other segment consumes exactly one.
func matchSegments(pattern, segments []string) bool {
	for len(pattern) > 0 {
		if pattern[0] == "**" {
			// Trailing `**` matches whatever remains, including nothing.
			if len(pattern) == 1 {
				return true
			}
			for i := 0; i <= len(segments); i++ {
				if matchSegments(pattern[1:], segments[i:]) {
					return true
				}
			}
			return false
		}
		if len(segments) == 0 {
			return false
		}
		// path.Match's error case is a malformed pattern, which can only
		// fail to match — there is no third answer to report.
		if ok, _ := path.Match(pattern[0], segments[0]); !ok {
			return false
		}
		pattern, segments = pattern[1:], segments[1:]
	}
	return len(segments) == 0
}
