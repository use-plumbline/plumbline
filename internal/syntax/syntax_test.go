package syntax

import "testing"

func parse(t *testing.T, src string) *Tree {
	t.Helper()
	p, err := NewParser()
	if err != nil {
		t.Fatalf("new parser: %v", err)
	}
	t.Cleanup(p.Close)
	tree, err := p.Parse([]byte(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	t.Cleanup(tree.Close)
	return tree
}

func TestParseAndNavigate(t *testing.T) {
	root := parse(t, "fn f(a: i128) { let b = a + 1; }").Root()
	if root.HasError() {
		t.Fatal("valid Rust reported a parse error")
	}
	if got := root.Kind(); got != "source_file" {
		t.Fatalf("root kind %q, want source_file", got)
	}

	var binary Node
	root.Walk(func(n Node) bool {
		if n.Kind() == "binary_expression" {
			binary = n
		}
		return true
	})
	if !binary.Valid() {
		t.Fatal("no binary_expression found")
	}
	if got := binary.Text(); got != "a + 1" {
		t.Errorf("Text() = %q, want %q", got, "a + 1")
	}
	left, ok := binary.Field("left")
	if !ok || left.Text() != "a" {
		t.Errorf("Field(left) = %q, %v", left.Text(), ok)
	}
	if _, ok := binary.Field("nonexistent"); ok {
		t.Error("Field reported a field that does not exist")
	}
}

func TestPositionsAreOneIndexed(t *testing.T) {
	// tree-sitter counts from zero; every consumer counts from one.
	root := parse(t, "fn a() {}\nfn b() {}").Root()
	fns := root.Children()
	if len(fns) != 2 {
		t.Fatalf("got %d children, want 2", len(fns))
	}
	if got := fns[0].Line(); got != 1 {
		t.Errorf("first function on line %d, want 1", got)
	}
	if got := fns[1].Line(); got != 2 {
		t.Errorf("second function on line %d, want 2", got)
	}
	if got := fns[1].Column(); got != 1 {
		t.Errorf("column %d, want 1", got)
	}
}

func TestBrokenSourceReportsAnError(t *testing.T) {
	if !parse(t, "fn f( {{{").Root().HasError() {
		t.Error("broken Rust did not report a parse error")
	}
}

// Accessors on an absent node must be safe, so a rule can chain lookups and
// check validity once at the end rather than after every step.
func TestZeroNodeIsSafe(t *testing.T) {
	var n Node
	if n.Valid() {
		t.Error("zero Node reports valid")
	}
	if n.Kind() != "" || n.Text() != "" || n.Line() != 0 || n.Column() != 0 {
		t.Error("zero Node returned a non-zero accessor value")
	}
	if _, ok := n.Field("left"); ok {
		t.Error("zero Node returned a field")
	}
	if _, ok := n.Child(0); ok {
		t.Error("zero Node returned a child")
	}
	if _, ok := n.PrevSibling(); ok {
		t.Error("zero Node returned a sibling")
	}
	if n.Children() != nil || n.HasError() {
		t.Error("zero Node misreported children or error state")
	}
	n.Walk(func(Node) bool {
		t.Error("Walk visited the zero Node")
		return true
	})
}

func TestWalkSkipsChildrenWhenVisitReturnsFalse(t *testing.T) {
	root := parse(t, "fn outer() { fn inner() {} }").Root()
	var seen []string
	root.Walk(func(n Node) bool {
		if n.Kind() == "function_item" {
			name, _ := n.Field("name")
			seen = append(seen, name.Text())
			return false // do not descend into the body
		}
		return true
	})
	if len(seen) != 1 || seen[0] != "outer" {
		t.Errorf("visited %v, want [outer] only", seen)
	}
}

// Attributes are preceding siblings of the item they decorate, which is the
// fact the contract-function search depends on.
func TestPrevSiblingReachesAttributes(t *testing.T) {
	root := parse(t, "#[contractimpl]\nimpl C {}").Root()
	var impl Node
	root.Walk(func(n Node) bool {
		if n.Kind() == "impl_item" {
			impl = n
		}
		return true
	})
	if !impl.Valid() {
		t.Fatal("no impl_item found")
	}
	prev, ok := impl.PrevSibling()
	if !ok {
		t.Fatal("impl_item has no previous sibling")
	}
	if prev.Kind() != "attribute_item" {
		t.Errorf("previous sibling is %q, want attribute_item", prev.Kind())
	}
}
