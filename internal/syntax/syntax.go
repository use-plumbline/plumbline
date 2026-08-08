// Package syntax is Plumbline's view of a parsed Rust source file.
//
// It wraps tree-sitter behind a small facade so that rules describe what they
// are looking for in terms of nodes, kinds and fields, and never name the
// parser. Swapping or upgrading the parser is then a change to this package
// rather than to every rule.
package syntax

import (
	"errors"
	"fmt"

	ts "github.com/tree-sitter/go-tree-sitter"
	tsrust "github.com/tree-sitter/tree-sitter-rust/bindings/go"
)

// Node is one node of a parsed file.
//
// It is a value carrying the source it was parsed from, so Text needs no
// argument and no caller can read a node against the wrong buffer. An absent
// node is the zero Node, which reports false from Valid and is safe to call
// every method on.
type Node struct {
	n   *ts.Node
	src []byte
}

// Valid reports whether the node exists. Accessors on an invalid node return
// zero values rather than panicking, so a rule can chain lookups and check
// once at the end.
func (n Node) Valid() bool { return n.n != nil }

// Kind is the grammar's name for this node, such as "call_expression".
func (n Node) Kind() string {
	if !n.Valid() {
		return ""
	}
	return n.n.Kind()
}

// Text is the source text the node spans.
func (n Node) Text() string {
	if !n.Valid() {
		return ""
	}
	return n.n.Utf8Text(n.src)
}

// Line is the 1-indexed line the node starts on.
func (n Node) Line() int {
	if !n.Valid() {
		return 0
	}
	return int(n.n.StartPosition().Row) + 1
}

// Column is the 1-indexed column the node starts at.
func (n Node) Column() int {
	if !n.Valid() {
		return 0
	}
	return int(n.n.StartPosition().Column) + 1
}

// Field returns the child stored under a grammar field name, such as "left"
// on a binary_expression. The second result is false when there is none.
func (n Node) Field(name string) (Node, bool) {
	if !n.Valid() {
		return Node{}, false
	}
	c := n.n.ChildByFieldName(name)
	if c == nil {
		return Node{}, false
	}
	return Node{n: c, src: n.src}, true
}

// Children returns the named children, in source order. Anonymous nodes —
// punctuation and keywords — are left out, since no rule needs them.
func (n Node) Children() []Node {
	if !n.Valid() {
		return nil
	}
	out := make([]Node, 0, n.n.NamedChildCount())
	for i := uint(0); i < n.n.NamedChildCount(); i++ {
		out = append(out, Node{n: n.n.NamedChild(i), src: n.src})
	}
	return out
}

// Child returns the i-th named child.
func (n Node) Child(i int) (Node, bool) {
	if !n.Valid() || i < 0 || uint(i) >= n.n.NamedChildCount() {
		return Node{}, false
	}
	return Node{n: n.n.NamedChild(uint(i)), src: n.src}, true
}

// PrevSibling returns the previous named sibling. It is how a rule reaches an
// item's attributes: in Rust's grammar `#[contractimpl]` is a sibling of the
// impl block it decorates, not a child of it.
func (n Node) PrevSibling() (Node, bool) {
	if !n.Valid() {
		return Node{}, false
	}
	p := n.n.PrevNamedSibling()
	if p == nil {
		return Node{}, false
	}
	return Node{n: p, src: n.src}, true
}

// HasError reports whether the node or anything under it failed to parse.
func (n Node) HasError() bool { return n.Valid() && n.n.HasError() }

// Walk visits n and its named descendants in source order. Returning false
// from visit skips that node's children.
func (n Node) Walk(visit func(Node) bool) {
	if !n.Valid() || !visit(n) {
		return
	}
	for _, c := range n.Children() {
		c.Walk(visit)
	}
}

// Tree is a parsed file. Close it when done: the parse tree is held outside
// Go's heap, and every Node taken from it stops being valid afterwards.
type Tree struct {
	t   *ts.Tree
	src []byte
}

// Root returns the file's root node.
func (t *Tree) Root() Node { return Node{n: t.t.RootNode(), src: t.src} }

// Close releases the tree.
func (t *Tree) Close() { t.t.Close() }

// Parser parses Rust source. It is not safe for concurrent use; give each
// goroutine its own.
type Parser struct {
	p *ts.Parser
}

// NewParser returns a parser for Rust.
func NewParser() (*Parser, error) {
	p := ts.NewParser()
	if err := p.SetLanguage(ts.NewLanguage(tsrust.Language())); err != nil {
		p.Close()
		return nil, fmt.Errorf("loading the Rust grammar: %w", err)
	}
	return &Parser{p: p}, nil
}

// Parse builds a tree for src. The caller owns the tree and must Close it.
func (p *Parser) Parse(src []byte) (*Tree, error) {
	t := p.p.Parse(src, nil)
	if t == nil {
		return nil, errors.New("the Rust parser returned no tree")
	}
	return &Tree{t: t, src: src}, nil
}

// Close releases the parser.
func (p *Parser) Close() { p.p.Close() }
