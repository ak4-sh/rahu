package ast

import l "rahu/lexer"

//go:generate stringer -type=NodeKind

type (
	Range struct {
		Start uint32
		End   uint32
	}

	NodeID   uint32
	NodeKind uint8

	Node struct {
		Kind  NodeKind
		Data  uint32
		Start uint32
		End   uint32

		FirstChild  NodeID
		NextSibling NodeID
	}
	AST struct {
		Root  NodeID
		Nodes []Node

		Names      []string
		Strings    []string
		Numbers    []string
		TokenTypes []l.TokenType
	}
	Operator        uint8
	CompareOp       uint8
	BooleanOperator uint8
)

const (
	And BooleanOperator = iota
	Or
)

const (
	Eq    CompareOp = iota // ==
	NotEq                  // !=
	Lt                     // <
	LtE                    // <=
	Gt                     // >
	GtE                    // >=
)

type UnaryOperator uint8

const (
	UAdd      UnaryOperator = iota // +x
	USub                           // -x
	Not                            // not x
	Increment                      // x++ / ++x
	Decrement                      // x-- / --x
)

const (
	Add Operator = iota
	Sub
	Mult
	Div
	FloorDiv
	Mod
	Pow
)

const (
	NodeModule NodeKind = iota
	NodeAssign
	NodeAugAssign
	NodeName
	NodeNumber
	NodeString
	NodeBinOp
	NodeUnaryOp
	NodeCall
	NodeAttribute
	NodeCompare
	NodeCompareOp
	NodeBooleanOp
	NodeBoolean
	NodeTuple
	NodeNone
	NodeList
	NodeIf
	NodeFor
	NodeWhile
	NodeReturn
	NodeBreak
	NodeContinue
	NodeFunctionDef
	NodeClassDef
	NodeExprStmt
	NodeBlock
	NodeArg
	NodeErrExp
	NodeSubScript
)

const NoNode NodeID = 0

func Valid(id NodeID) bool {
	return id != NoNode
}

func New(numTokens int) *AST {
	if numTokens < 8 {
		numTokens = 8
	}

	nameCap := min(numTokens/4, 400)
	stringCap := min(numTokens/8, 200)
	numberCap := min(numTokens/8, 200)
	nodeCap := min(numTokens*2, 800)

	return &AST{
		Nodes:   make([]Node, 1, nodeCap),
		Names:   make([]string, 0, nameCap),
		Strings: make([]string, 0, stringCap),
		Numbers: make([]string, 0, numberCap),
	}
}

func (n Node) Range() (uint32, uint32) {
	return n.Start, n.End
}

func (r *Range) IsEmpty() bool {
	return r.Start == r.End
}

func (a *AST) NewNode(kind NodeKind, start, end uint32) NodeID {
	id := NodeID(len(a.Nodes))

	a.Nodes = append(a.Nodes, Node{
		Kind:        kind,
		Start:       start,
		End:         end,
		FirstChild:  NoNode,
		NextSibling: NoNode,
	})

	return id
}

func (a *AST) Reset() {
	a.Nodes = a.Nodes[:1]
	a.Names = a.Names[:0]
	a.Strings = a.Strings[:0]
	a.Numbers = a.Numbers[:0]
}

func (a *AST) Node(id NodeID) Node {
	return a.Nodes[id]
}

// AddChild attaches `child` as the last child of `parent`.
//
// The AST stores children as a singly-linked list using the `FirstChild` and
// `NextSibling` fields. `FirstChild` points to the first child node and each
// child links to the next through `NextSibling`.
//
// Behavior:
//
//   - If either `parent` or `child` is `NoNode`, the call is ignored.
//   - If the parent has no children (`FirstChild == NoNode`), `child` becomes
//     the first child.
//   - Otherwise the existing sibling chain is traversed and `child` is appended
//     at the end.
//
// This preserves insertion order of children while keeping the node structure
func (a *AST) AddChild(parent, child NodeID) {
	if parent == NoNode || child == NoNode {
		return
	}

	p := &a.Nodes[parent]

	// first child
	if p.FirstChild == NoNode {
		p.FirstChild = child
		return
	}

	// walk sibling chain
	cur := p.FirstChild
	for {
		next := a.Nodes[cur].NextSibling
		if next == NoNode {
			a.Nodes[cur].NextSibling = child
			return
		}
		cur = next
	}
}

func (a *AST) LastChild(id NodeID) NodeID {
	c := a.Nodes[id].FirstChild
	if c == NoNode {
		return NoNode
	}

	for a.Nodes[c].NextSibling != NoNode {
		c = a.Nodes[c].NextSibling
	}

	return c
}

// NodeAssign invariant
// Child 0 ... n -1 -> targets
// ChildN -> value
