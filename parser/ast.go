package parser

type Node any 

type Operator int

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
}

type Number struct {
	Value string
}

func (n *Number) expressionNode() {}

type Name struct {
	Id string
}

func (n *Name) expressionNode() {}

type BinOp struct {
	Left  Expression
	Op    Operator
	Right Expression
}

func (b *BinOp) expressionNode() {}

type Assign struct {
	Targets []Expression
	Value   Expression
}

func (a *Assign) statementNode() {}

type Module struct {
	Body []Statement
}

type FunctionDef struct{
	Name string
	Args []string
	Body []Statement
}

func (f *FunctionDef) statementNode(){}

type Return struct{
	Value Expression
}

func (r *Return) statementNode(){}

type If struct {
	Test Expression
	Body []Statement
	Orelse []Statement
}

func (i *If) statementNode() {}

type For struct{
	Target Expression
	Iter Expression
	Body []Statement
}

func (f *For) statementNode(){}


type Call struct {
	Func Expression
	Args []Expression
}

func (c *Call) expressionNode(){}


type CompareOp int
const (
	Eq CompareOp = iota // ==
	NotEq // !=
	Lt // <
	LtE // <=
	Gt // >
	GtE // >=
)

type UnaryOperator int

const (
	UAdd UnaryOperator = iota // +x
	USub                      // -x
	Not                       // not x
)

func (u *UnaryOperator) expressionNode() {}

type UnaryOp struct {
	Op      UnaryOperator
	Operand Expression
}

func (u *UnaryOp) expressionNode() {}

type String struct{
	Value string
}

func (s *String) expressionNode(){}
