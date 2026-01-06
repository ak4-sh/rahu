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
