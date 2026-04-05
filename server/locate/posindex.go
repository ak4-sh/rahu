// Package locate provides position-to-node lookup functionality.
// PositionIndex provides O(log n) lookup of AST nodes at a given byte offset.
package locate

import (
	"rahu/parser/ast"
	"sort"
)

// PosEntry represents a single node that can be located by position.
// We store both Name nodes and Attribute nodes (for attribute access).
type PosEntry struct {
	Start uint32
	End   uint32
	Node  ast.NodeID
	Kind  ResultKind
}

// PositionIndex maintains a sorted slice of position entries for binary search lookup.
// Entries are sorted by (Start ASC, End DESC) to handle nested ranges correctly:
// when searching, we find entries with Start <= pos, and among those with the same
// Start, larger ranges come first, allowing us to find the innermost match.
type PositionIndex struct {
	entries []PosEntry
}

// NewPositionIndex creates a new empty PositionIndex.
func NewPositionIndex() *PositionIndex {
	return &PositionIndex{}
}

// Build constructs a PositionIndex from an AST by traversing all nodes
// and collecting Name and Attribute nodes.
func Build(tree *ast.AST) *PositionIndex {
	if tree == nil || tree.Root == ast.NoNode {
		return &PositionIndex{}
	}

	// Pre-allocate with estimated capacity (most files have many names)
	entries := make([]PosEntry, 0, len(tree.Nodes)/4)

	// Traverse all nodes - we use a simple iteration since AST stores
	// nodes in a flat slice (much faster than recursive tree walk)
	for id := ast.NodeID(1); int(id) < len(tree.Nodes); id++ {
		node := tree.Nodes[id]

		switch node.Kind {
		case ast.NodeName:
			entries = append(entries, PosEntry{
				Start: node.Start,
				End:   node.End,
				Node:  id,
				Kind:  NameResult,
			})

		case ast.NodeAttribute:
			// For attributes, we store the attribute node itself
			// The position should be the attribute name part (second child)
			base := node.FirstChild
			if base != ast.NoNode {
				attrName := tree.Nodes[base].NextSibling
				if attrName != ast.NoNode {
					attrNode := tree.Nodes[attrName]
					entries = append(entries, PosEntry{
						Start: attrNode.Start,
						End:   attrNode.End,
						Node:  id, // Store the Attribute node, not the name
						Kind:  AttributeResult,
					})
				}
			}
		}
	}

	// Sort by (Start ASC, End DESC) - for same start position, larger ranges first
	// This ensures when we binary search, nested nodes are ordered correctly
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Start != entries[j].Start {
			return entries[i].Start < entries[j].Start
		}
		// For same start, larger end comes first (outer before inner)
		return entries[i].End > entries[j].End
	})

	return &PositionIndex{entries: entries}
}

// Lookup finds the node at the given byte offset using binary search.
// Returns the most specific (innermost) matching node.
// For NodeAttribute, returns AttributeResult when pos is on the attribute name.
// For NodeName, returns NameResult.
func (pi *PositionIndex) Lookup(pos int) Result {
	if pi == nil || len(pi.entries) == 0 {
		return Result{}
	}

	p := uint32(pos)

	// Binary search to find rightmost entry where Start <= pos
	// We want the entry with the largest Start that is still <= pos
	idx := sort.Search(len(pi.entries), func(i int) bool {
		return pi.entries[i].Start > p
	})

	// idx is now the first entry where Start > pos, so we check idx-1 and earlier
	// We need to find the innermost (smallest range) that contains pos

	var best *PosEntry
	for i := idx - 1; i >= 0; i-- {
		e := &pi.entries[i]

		// If this entry's Start is too far before pos, and we already have a match,
		// we can stop (since entries are sorted by Start)
		if best != nil && e.Start < best.Start {
			break
		}

		// Check if pos is within this entry's range
		if e.Start <= p && p <= e.End {
			// This entry contains pos
			if best == nil {
				best = e
			} else {
				// Prefer the innermost (smaller range)
				// Since we sorted by (Start ASC, End DESC), among entries with same Start,
				// smaller End means more specific
				if e.End-e.Start < best.End-best.Start {
					best = e
				}
			}
		}
	}

	if best == nil {
		return Result{}
	}

	return Result{Kind: best.Kind, Node: best.Node}
}

// LookupWithMode finds a node at pos, filtered by mode.
func (pi *PositionIndex) LookupWithMode(pos int, mode locateMode) Result {
	if pi == nil || len(pi.entries) == 0 {
		return Result{}
	}

	p := uint32(pos)

	idx := sort.Search(len(pi.entries), func(i int) bool {
		return pi.entries[i].Start > p
	})

	var best *PosEntry
	for i := idx - 1; i >= 0; i-- {
		e := &pi.entries[i]

		if best != nil && e.Start < best.Start {
			break
		}

		// Filter by mode
		if mode == locateNameOnly && e.Kind != NameResult {
			continue
		}
		if mode == locateAttrOnly && e.Kind != AttributeResult {
			continue
		}

		if e.Start <= p && p <= e.End {
			if best == nil {
				best = e
			} else if e.End-e.Start < best.End-best.Start {
				best = e
			}
		}
	}

	if best == nil {
		return Result{}
	}

	return Result{Kind: best.Kind, Node: best.Node}
}

// Size returns the number of entries in the index.
func (pi *PositionIndex) Size() int {
	if pi == nil {
		return 0
	}
	return len(pi.entries)
}
