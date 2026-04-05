package analyser

import "rahu/parser/ast"

type NameContext int

const (
	Read NameContext = iota
	Write
)

type PendingAttr struct {
	Node     ast.NodeID
	Class    *Symbol
	SelfName string
}

type Resolver struct {
	tree       *ast.AST
	current    *Scope
	errors     []SemanticError
	loopDepth  int
	Resolved   map[ast.NodeID]*Symbol
	inFunction bool

	inClass      bool
	currentClass *Symbol
	ResolvedAttr map[ast.NodeID]*Symbol
	PendingAttrs []PendingAttr
	selfName     string
	ExprTypes    map[ast.NodeID]*Symbol
}

type SemanticError struct {
	Span ast.Range
	Msg  string
}

func newResolver(tree *ast.AST, global *Scope) *Resolver {
	resolvedCap := len(tree.Nodes) / 4
	if resolvedCap < 8 {
		resolvedCap = 8
	}
	attrCap := len(tree.Nodes) / 16
	if attrCap < 4 {
		attrCap = 4
	}
	exprTypeCap := len(tree.Nodes) / 32
	if exprTypeCap < 4 {
		exprTypeCap = 4
	}
	return &Resolver{
		tree:         tree,
		current:      global,
		errors:       nil,
		loopDepth:    0,
		Resolved:     make(map[ast.NodeID]*Symbol, resolvedCap),
		inFunction:   false,
		inClass:      false,
		PendingAttrs: make([]PendingAttr, 0, attrCap),
		ResolvedAttr: make(map[ast.NodeID]*Symbol, attrCap),
		selfName:     "",
		ExprTypes:    make(map[ast.NodeID]*Symbol, exprTypeCap),
	}
}

func Resolve(tree *ast.AST, global *Scope) (*Resolver, []SemanticError) {
	r := newResolver(tree, global)
	r.visitModule()
	PromoteClassMembers(global)
	r.BindMembers()
	return r, r.errors
}

func (r *Resolver) visitModule() {
	if r.tree == nil {
		return
	}

	for stmt := r.tree.Nodes[r.tree.Root].FirstChild; stmt != ast.NoNode; stmt = r.tree.Nodes[stmt].NextSibling {
		if stmt != ast.NoNode {
			r.visitStmt(stmt)
		}
	}
}

func (r *Resolver) visitStmt(stmt ast.NodeID) {
	switch r.tree.Node(stmt).Kind {
	case ast.NodeAugAssign:
		target := r.tree.Nodes[stmt].FirstChild
		value := ast.NoNode
		if target != ast.NoNode {
			value = r.tree.Nodes[target].NextSibling
		}
		r.visitExpr(target, Read)
		r.visitExpr(value, Read)
		r.visitExpr(target, Write)

	case ast.NodeAssign:
		value := r.tree.Nodes[stmt].FirstChild
		if value == ast.NoNode {
			return
		}

		r.visitExpr(value, Read)
		class := r.ExprTypes[value]

		for target := r.tree.Nodes[value].NextSibling; target != ast.NoNode; target = r.tree.Nodes[target].NextSibling {
			r.visitExpr(target, Write)

			if r.tree.Node(target).Kind == ast.NodeName && class != nil {
				sym := r.Resolved[target]
				if sym != nil {
					sym.InstanceOf = class
				}
			}
		}

	case ast.NodeClassDef:
		nameID, bases, body := r.tree.ClassParts(stmt)
		nameText, _ := r.tree.NameText(nameID)

		for base := r.tree.Nodes[bases].FirstChild; base != ast.NoNode; base = r.tree.Nodes[base].NextSibling {
			r.visitExpr(base, Read)
		}

		classSym := r.current.Symbols[nameText]
		if doc, ok := r.tree.DocString(stmt); ok && classSym != nil {
			classSym.DocString = doc
		}
		r.Resolved[nameID] = classSym

		for baseExpr := r.tree.Nodes[bases].FirstChild; baseExpr != ast.NoNode; baseExpr = r.tree.Nodes[baseExpr].NextSibling {
			if baseExpr == ast.NoNode {
				continue
			}

			if r.tree.Node(baseExpr).Kind != ast.NodeName {
				r.error(r.tree.RangeOf(baseExpr), "unsupported base class expression")
				continue
			}

			baseName, _ := r.tree.NameText(baseExpr)
			baseSym, ok := r.current.Lookup(baseName)
			if !ok || baseSym == nil {
				r.error(r.tree.RangeOf(baseExpr), "undefined base class: "+baseName)
				continue
			}

			if baseSym.Kind != SymClass {
				r.error(r.tree.RangeOf(baseExpr), baseName+" is not a class")
				continue
			}

			classSym.Bases = append(classSym.Bases, baseSym)
		}

		if classSym == nil || classSym.Inner == nil {
			r.error(r.tree.RangeOf(nameID), "internal compiler error: missing class symbol or scope for: "+nameText)
			return
		}

		prevScope := r.current
		prevClass := r.currentClass
		prevInClass := r.inClass
		prevSelf := r.selfName

		r.current = classSym.Inner
		r.currentClass = classSym
		r.inClass = true

		for inner := r.tree.Nodes[body].FirstChild; inner != ast.NoNode; inner = r.tree.Nodes[inner].NextSibling {
			r.visitStmt(inner)
		}

		r.current = prevScope
		r.currentClass = prevClass
		r.inClass = prevInClass
		r.selfName = prevSelf

	case ast.NodeFunctionDef:
		nameID, args, body := r.tree.FunctionParts(stmt)
		nameText, _ := r.tree.NameText(nameID)

		for arg := r.tree.Nodes[args].FirstChild; arg != ast.NoNode; arg = r.tree.Nodes[arg].NextSibling {
			paramName := r.tree.Nodes[arg].FirstChild
			if def := r.tree.Nodes[paramName].NextSibling; def != ast.NoNode {
				r.visitExpr(def, Read)
			}
		}

		fnSym := r.current.Symbols[nameText]
		if doc, ok := r.tree.DocString(stmt); ok && fnSym != nil {
			fnSym.DocString = doc
		}
		r.Resolved[nameID] = fnSym
		if fnSym == nil || fnSym.Inner == nil {
			r.error(r.tree.RangeOf(nameID), "internal compiler error: missing function symbol or scope for "+nameText)
			return
		}

		prevScope := r.current
		prevInFn := r.inFunction
		prevSelf := r.selfName

		if r.inClass && args != ast.NoNode && r.tree.Nodes[args].FirstChild != ast.NoNode {
			selfParam := r.tree.Nodes[r.tree.Nodes[args].FirstChild].FirstChild
			r.selfName, _ = r.tree.NameText(selfParam)
		} else {
			r.selfName = ""
		}

		r.current = fnSym.Inner
		r.inFunction = true

		for inner := r.tree.Nodes[body].FirstChild; inner != ast.NoNode; inner = r.tree.Nodes[inner].NextSibling {
			r.visitStmt(inner)
		}

		r.current = prevScope
		r.inFunction = prevInFn
		r.selfName = prevSelf

	case ast.NodeExprStmt:
		r.visitExpr(r.tree.Nodes[stmt].FirstChild, Read)

	case ast.NodeIf:
		test := r.tree.Nodes[stmt].FirstChild
		body := ast.NoNode
		orelse := ast.NoNode
		if test != ast.NoNode {
			body = r.tree.Nodes[test].NextSibling
		}
		if body != ast.NoNode {
			orelse = r.tree.Nodes[body].NextSibling
		}
		r.visitExpr(test, Read)

		for inner := r.tree.Nodes[body].FirstChild; inner != ast.NoNode; inner = r.tree.Nodes[inner].NextSibling {
			r.visitStmt(inner)
		}

		for inner := r.tree.Nodes[orelse].FirstChild; inner != ast.NoNode; inner = r.tree.Nodes[inner].NextSibling {
			r.visitStmt(inner)
		}

	case ast.NodeFor:
		target := r.tree.Nodes[stmt].FirstChild
		iter := ast.NoNode
		body := ast.NoNode
		orelse := ast.NoNode
		if target != ast.NoNode {
			iter = r.tree.Nodes[target].NextSibling
		}
		if iter != ast.NoNode {
			body = r.tree.Nodes[iter].NextSibling
		}
		if body != ast.NoNode {
			orelse = r.tree.Nodes[body].NextSibling
		}
		r.visitExpr(iter, Read)
		r.loopDepth++
		r.visitExpr(target, Write)

		for inner := r.tree.Nodes[body].FirstChild; inner != ast.NoNode; inner = r.tree.Nodes[inner].NextSibling {
			r.visitStmt(inner)
		}
		for inner := r.tree.Nodes[orelse].FirstChild; inner != ast.NoNode; inner = r.tree.Nodes[inner].NextSibling {
			r.visitStmt(inner)
		}
		r.loopDepth--

	case ast.NodeWhile:
		test := r.tree.Nodes[stmt].FirstChild
		body := ast.NoNode
		if test != ast.NoNode {
			body = r.tree.Nodes[test].NextSibling
		}
		r.loopDepth++
		r.visitExpr(test, Read)

		for inner := r.tree.Nodes[body].FirstChild; inner != ast.NoNode; inner = r.tree.Nodes[inner].NextSibling {
			r.visitStmt(inner)
		}
		r.loopDepth--

	case ast.NodeReturn:
		if !r.inFunction {
			r.error(r.tree.RangeOf(stmt), "return outside function")
		}

		if value := r.tree.Nodes[stmt].FirstChild; value != ast.NoNode {
			r.visitExpr(value, Read)
		}

	case ast.NodeBreak:
		r.checkLoopContext(r.tree.RangeOf(stmt), "break")

	case ast.NodeContinue:
		r.checkLoopContext(r.tree.RangeOf(stmt), "continue")
	}
}

func (r *Resolver) checkLoopContext(pos ast.Range, keyword string) {
	if r.loopDepth == 0 {
		r.error(pos, keyword+" outside loop")
	}
}

func (r *Resolver) resolveName(id ast.NodeID, ctx NameContext) {
	name, _ := r.tree.NameText(id)
	span := r.tree.RangeOf(id)

	var sym *Symbol
	if ctx == Write {
		sym = r.current.Symbols[name]

		if sym == nil {
			r.error(span, "internal error: write to undefined local "+name)
			return
		}
	} else {
		var ok bool
		sym, ok = r.current.Lookup(name)
		if !ok || sym == nil {
			r.error(span, "undefined name: "+name)
			return
		}
	}
	r.Resolved[id] = sym
}

func (r *Resolver) visitExpr(expr ast.NodeID, ctx NameContext) {
	if expr == ast.NoNode {
		return
	}

	switch r.tree.Node(expr).Kind {
	case ast.NodeName:
		r.resolveName(expr, ctx)
		return

	case ast.NodeNumber, ast.NodeString, ast.NodeBoolean, ast.NodeNone, ast.NodeErrExp:
		return

	case ast.NodeBinOp:
		left := r.tree.Nodes[expr].FirstChild
		right := ast.NoNode
		if left != ast.NoNode {
			right = r.tree.Nodes[left].NextSibling
		}
		r.visitExpr(left, Read)
		r.visitExpr(right, Read)

	case ast.NodeUnaryOp:
		r.visitExpr(r.tree.Nodes[expr].FirstChild, Read)

	case ast.NodeBooleanOp:
		for child := r.tree.Nodes[expr].FirstChild; child != ast.NoNode; child = r.tree.Nodes[child].NextSibling {
			r.visitExpr(child, Read)
		}

	case ast.NodeCompare:
		left := r.tree.Nodes[expr].FirstChild
		if left == ast.NoNode {
			return
		}
		r.visitExpr(left, Read)
		for cmp := r.tree.Nodes[left].NextSibling; cmp != ast.NoNode; cmp = r.tree.Nodes[cmp].NextSibling {
			r.visitExpr(r.tree.Nodes[cmp].FirstChild, Read)
		}

	case ast.NodeCall:
		funcID := r.tree.Nodes[expr].FirstChild
		for child := funcID; child != ast.NoNode; child = r.tree.Nodes[child].NextSibling {
			r.visitExpr(child, Read)
		}

		if r.tree.Node(funcID).Kind == ast.NodeName {
			sym := r.Resolved[funcID]
			if sym != nil && sym.Kind == SymClass {
				r.ExprTypes[expr] = sym
			}
		}

	case ast.NodeTuple, ast.NodeList:
		for child := r.tree.Nodes[expr].FirstChild; child != ast.NoNode; child = r.tree.Nodes[child].NextSibling {
			r.visitExpr(child, ctx)
		}

	case ast.NodeAttribute:
		r.visitExpr(r.tree.Nodes[expr].FirstChild, Read)
		r.PendingAttrs = append(r.PendingAttrs, PendingAttr{
			Node:     expr,
			Class:    r.currentClass,
			SelfName: r.selfName,
		})
	}
}

func (r *Resolver) error(span ast.Range, msg string) {
	r.errors = append(r.errors, SemanticError{
		Span: span,
		Msg:  msg,
	})
}
