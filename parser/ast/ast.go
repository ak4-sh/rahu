package ast

import (
	"strings"
)

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
		LastChild   NodeID
		NextSibling NodeID
	}
	AST struct {
		Root  NodeID
		Nodes []Node

		Names     []string
		Strings   []string
		Numbers   []string
		nameIndex map[string]uint32
	}
	Operator        uint8
	CompareOp       uint8
	BooleanOperator uint8
	BooleanVal      uint8
	AugAssignOp     uint8
)

const (
	And BooleanOperator = iota
	Or
)

const (
	TRUE BooleanVal = iota
	FALSE
)

const (
	Eq    CompareOp = iota // ==
	NotEq                  // !=
	Lt                     // <
	LtE                    // <=
	Gt                     // >
	GtE                    // >=
)

const (
	AugInvalid AugAssignOp = iota
	AugAdd
	AugSub
	AugMul
	AugDiv
	AugFloorDiv
	AugPow
	AugAnd
	AugLShift
	AugRShift
	AugMod
	AugOr
	AugXor
	AugMatMul
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
	// NodeModule
	// Children:
	//   0..n -> top-level statements
	// Data:
	//   unused
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
	NodeArgs
	NodeErrExp
	NodeSubScript
	NodeBaseList
	NodeErrStmt
	NodeParam
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

	a := AST{
		Nodes:     make([]Node, 1, nodeCap),
		Names:     make([]string, 0, nameCap),
		Strings:   make([]string, 0, stringCap),
		Numbers:   make([]string, 0, numberCap),
		nameIndex: map[string]uint32{},
	}
	a.Names = append(a.Names, "")
	a.Names = append(a.Names, "None")
	a.Numbers = append(a.Numbers, "")
	a.Strings = append(a.Strings, "")
	a.nameIndex[""] = 0
	a.nameIndex["None"] = 1

	return &a
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
		LastChild:   NoNode,
	})

	return id
}

func (a *AST) Reset() {
	a.Root = NoNode
	a.Nodes = a.Nodes[:1]

	a.Names = a.Names[:0]
	a.Strings = a.Strings[:0]
	a.Numbers = a.Numbers[:0]

	a.Names = append(a.Names, "", "None")
	a.Strings = append(a.Strings, "")
	a.Numbers = append(a.Numbers, "")
	a.nameIndex = map[string]uint32{
		"":     0,
		"None": 1,
	}
}

func (a *AST) Node(id NodeID) Node {
	return a.Nodes[id]
}

// AddChild attaches child as the last child of parent.
//
// Children are stored as a singly linked sibling list.
// FirstChild points to the first child, LastChild to the last child,
// and each child links to the next through NextSibling.
func (a *AST) AddChild(parent, child NodeID) {
	if parent == NoNode || child == NoNode {
		return
	}

	p := &a.Nodes[parent]
	a.Nodes[child].NextSibling = NoNode

	if p.FirstChild == NoNode {
		p.FirstChild = child
		p.LastChild = child
		return
	}

	a.Nodes[p.LastChild].NextSibling = child
	p.LastChild = child
}

func (a *AST) internName(name string) uint32 {
	if idx, ok := a.nameIndex[name]; ok {
		return idx
	}

	name = strings.Clone(name) // detach from lexer input
	idx := uint32(len(a.Names))
	a.Names = append(a.Names, name)
	a.nameIndex[name] = idx
	return idx
}

func (a *AST) NewNameNode(start, end uint32, name string) NodeID {
	id := a.NewNode(NodeName, start, end)
	a.Nodes[id].Data = a.internName(name)
	return id
}

func (a *AST) NewStringNode(start, end uint32, s string) NodeID {
	id := a.NewNode(NodeString, start, end)
	idx := uint32(len(a.Strings))
	a.Strings = append(a.Strings, s)
	a.Nodes[id].Data = idx
	return id
}

func (a *AST) LastChild(id NodeID) NodeID {
	if id == NoNode {
		return NoNode
	}
	return a.Nodes[id].LastChild
}

// NodeAssign invariant
// Child0 -> value
// Child 1 ... n -> targets
