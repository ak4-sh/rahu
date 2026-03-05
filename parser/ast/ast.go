package ast

import "rahu/lexer"

type Node any

type NodeID uint64

type Operator int

type Range struct {
	Start int
	End   int
}

func (r Range) IsEmpty() bool {
	return r.Start == 0 && r.End == 0
}

const (
	Add Operator = iota
	Sub
	Mult
	Div
	FloorDiv
	Mod
	Pow
)

type Statement interface {
	Node
	statementNode()
}

type Expression interface {
	Node
	expressionNode()
	Position() Range
}

type Number struct {
	Value string
	Pos   Range
}

func (n *Number) expressionNode() {}
func (n *Number) Position() Range { return n.Pos }

type Name struct {
	Text string
	Pos  Range
	ID   NodeID
}

func (n *Name) expressionNode() {}
func (n *Name) Position() Range { return n.Pos }

type BinOp struct {
	Left  Expression
	Op    Operator
	Right Expression
	Pos   Range
}

func (b *BinOp) expressionNode() {}
func (b *BinOp) Position() Range { return b.Pos }

type Assign struct {
	Targets []Expression
	Value   Expression
	Pos     Range
}

func (a *Assign) statementNode() {}

type AugAssign struct {
	Target Expression
	Op     lexer.TokenType
	Value  Expression
	Pos    Range
}

func (a *AugAssign) statementNode() {}

type Module struct {
	Body []Statement
}

type FuncArg struct {
	Name    *Name
	Default Expression
	Pos     Range
}

type FunctionDef struct {
	Name      *Name
	NamePos   Range
	Args      []FuncArg
	Body      []Statement
	Pos       Range
	DocString string
}

func (f *FunctionDef) statementNode() {}

type Return struct {
	Value Expression
	Pos   Range
}

func (r *Return) statementNode() {}

type If struct {
	Test   Expression
	Body   []Statement
	Orelse []Statement
	Pos    Range
}

func (i *If) statementNode() {}

type Tuple struct {
	Elts []Expression
	Pos  Range
}

func (t *Tuple) expressionNode() {}
func (t *Tuple) Position() Range { return t.Pos }

type For struct {
	Target Expression
	Iter   Expression
	Body   []Statement
	Orelse []Statement
	Pos    Range
}

func (f *For) statementNode() {}

type WhileLoop struct {
	Test Expression
	Body []Statement
	Pos  Range
}

func (w *WhileLoop) statementNode() {}

type Call struct {
	Func Expression
	Args []Expression
	Pos  Range
}

func (c *Call) expressionNode() {}
func (c *Call) Position() Range { return c.Pos }

type Compare struct {
	Left  Expression
	Ops   []CompareOp
	Right []Expression
	Pos   Range
}

func (c *Compare) expressionNode() {}
func (c *Compare) Position() Range { return c.Pos }

type CompareOp int

const (
	Eq    CompareOp = iota // ==
	NotEq                  // !=
	Lt                     // <
	LtE                    // <=
	Gt                     // >
	GtE                    // >=
)

type UnaryOperator int

const (
	UAdd      UnaryOperator = iota // +x
	USub                           // -x
	Not                            // not x
	Increment                      // x++ / ++x
	Decrement                      // x-- / --x
)

type UnaryOp struct {
	Op      UnaryOperator
	Operand Expression
	Pos     Range
}

func (u *UnaryOp) expressionNode() {}
func (u *UnaryOp) Position() Range { return u.Pos }

type String struct {
	Value string
	Pos   Range
}

func (s *String) expressionNode() {}
func (s *String) Position() Range { return s.Pos }

type ExprStmt struct {
	Value Expression
	Pos   Range
}

func (e *ExprStmt) statementNode() {}

type Boolean struct {
	Value bool
	Pos   Range
}

func (b *Boolean) expressionNode() {}
func (b *Boolean) Position() Range { return b.Pos }

type BooleanOperator int

const (
	And BooleanOperator = iota
	Or
)

type BooleanOp struct {
	Operator BooleanOperator
	Values   []Expression
	Pos      Range
}

func (bo *BooleanOp) expressionNode() {}
func (bo *BooleanOp) Position() Range { return bo.Pos }

type List struct {
	Elts []Expression
	Pos  Range
}

func (l *List) expressionNode() {}
func (l *List) Position() Range { return l.Pos }

type Break struct {
	Pos Range
}

func (b *Break) statementNode() {}

type Continue struct {
	Pos Range
}

func (c *Continue) statementNode() {}

type KeywordArg struct {
	Name  *Name
	Value Expression
	Pos   Range
}

type ClassDef struct {
	Pos        Range
	Name       *Name
	Bases      []Expression
	Keywords   []KeywordArg
	Body       []Statement
	Decorators []Expression
	DocString  string
}

func (c *ClassDef) statementNode() {}

type Attribute struct {
	ID    NodeID
	Pos   Range
	Value Expression
	Attr  *Name
}

func (a *Attribute) expressionNode() {}
func (a *Attribute) Position() Range { return a.Pos }
