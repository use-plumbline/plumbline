package syntax

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestTreeSitterConfined verifies the architecture invariant: tree-sitter is
// only imported in internal/syntax. Rules see rule.Node and never name the
// parser, which is what lets us swap or upgrade it without touching a single
// rule.
func TestTreeSitterConfined(t *testing.T) {
	// Walk every .go file in the repository, skipping vendor and testdata.
	var goFiles []string
	err := filepath.Walk("../../", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip unreadable paths
		}
		if info.IsDir() {
			name := info.Name()
			if name == "vendor" || name == "testdata" || name == ".git" || name == "bin" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go") {
			goFiles = append(goFiles, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking repo: %v", err)
	}

	for _, path := range goFiles {
		if strings.HasSuffix(path, "internal/syntax/syntax.go") {
			continue // the one file that is allowed to import tree-sitter
		}
		f, err := os.Open(path)
		if err != nil {
			continue
		}
		scanner := bufio.NewScanner(f)
		inImport := false
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if strings.HasPrefix(line, "import (") {
				inImport = true
				continue
			}
			if inImport && line == ")" {
				inImport = false
				continue
			}
			if strings.HasPrefix(line, "import \"") {
				// single-line import
				if strings.Contains(line, "tree-sitter") {
					t.Errorf("tree-sitter imported outside internal/syntax: %s", path)
				}
				continue
			}
			if inImport && strings.Contains(line, "tree-sitter") {
				t.Errorf("tree-sitter imported outside internal/syntax: %s", path)
			}
		}
		_ = f.Close()
	}
}
