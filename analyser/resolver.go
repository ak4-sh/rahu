package analyser

import "rahu/parser/ast"

type NameContext int

const (
	Read NameContext = iota
	Write
)

type PendingAttr struct {
	Node     *ast.Attribute
	Class    *Symbol
	SelfName string
}

type Resolver struct {
	current   *Scope
	errors    []SemanticError
	loopDepth int

	Resolved   map[*ast.Name]*Symbol
	inFunction bool

	inClass      bool
	currentClass *Symbol
	ResolvedAttr map[*ast.Attribute]*Symbol
	PendingAttrs []PendingAttr
	selfName     string
}

type SemanticError struct {
	Span ast.Range
	Msg  string
}

func newResolver(global *Scope) *Resolver {
	return &Resolver{
		current:      global,
		errors:       nil,
		loopDepth:    0,
		Resolved:     make(map[*ast.Name]*Symbol),
		inFunction:   false,
		inClass:      false,
		PendingAttrs: make([]PendingAttr, 0),
		ResolvedAttr: make(map[*ast.Attribute]*Symbol),
		selfName:     "",
	}
}

func Resolve(m *ast.Module, global *Scope) (*Resolver, []SemanticError) {
	r := newResolver(global)
	r.visitModule(m)
	r.BindMembers()
	return r, r.errors
}

func (r *Resolver) visitModule(m *ast.Module) {
	for _, stmt := range m.Body {
		r.visitStmt(stmt)
	}
}

func (r *Resolver) visitStmt(stmt ast.Statement) {
	switch s := stmt.(type) {
	case *ast.AugAssign:
		r.visitExpr(s.Target, Read)
		r.visitExpr(s.Value, Read)
		r.visitExpr(s.Target, Write)

	case *ast.Assign:
		r.visitExpr(s.Value, Read)

		for _, t := range s.Targets {
			r.visitExpr(t, Write)
		}

	case *ast.ClassDef:
		for _, base := range s.Bases {
			if base != nil {
				r.visitExpr(base, Read)
			}
		}

		classSym := r.current.Symbols[s.Name.ID]
		r.Resolved[s.Name] = classSym

		if classSym == nil || classSym.Inner == nil {
			r.error(s.Pos, "internal compiler error: missing class symbol or scope for: "+s.Name.ID)
			return
		}

		prevScope := r.current
		prevClass := r.currentClass
		prevInClass := r.inClass
		prevSelf := r.selfName

		r.current = classSym.Inner
		r.currentClass = classSym
		r.inClass = true

		for _, stmt := range s.Body {
			r.visitStmt(stmt)
		}

		r.current = prevScope
		r.currentClass = prevClass
		r.inClass = prevInClass
		r.selfName = prevSelf

	case *ast.FunctionDef:
		for _, arg := range s.Args {
			if arg.Default != nil {
				r.visitExpr(arg.Default, Read)
			}
		}

		fnSym := r.current.Symbols[s.Name.ID]
		r.Resolved[s.Name] = fnSym
		if fnSym == nil || fnSym.Inner == nil {
			r.error(s.Pos, "internal compiler error: missing function symbol or scope for "+s.Name.ID)
			return
		}

		prevScope := r.current
		prevInFn := r.inFunction
		prevSelf := r.selfName

		if r.inClass && len(s.Args) > 0 {
			r.selfName = s.Args[0].Name.ID
		} else {
			r.selfName = ""
		}

		r.current = fnSym.Inner
		r.inFunction = true

		for _, stmt := range s.Body {
			r.visitStmt(stmt)
		}

		r.current = prevScope
		r.inFunction = prevInFn
		r.selfName = prevSelf

	case *ast.ExprStmt:
		r.visitExpr(s.Value, Read)

	case *ast.If:
		r.visitExpr(s.Test, Read)

		for _, st := range s.Body {
			r.visitStmt(st)
		}

		for _, st := range s.Orelse {
			r.visitStmt(st)
		}

	case *ast.For:
		r.visitExpr(s.Iter, Read)
		r.loopDepth++
		r.visitExpr(s.Target, Write)

		for _, st := range s.Body {
			r.visitStmt(st)
		}
		r.loopDepth--

	case *ast.WhileLoop:
		r.loopDepth++
		r.visitExpr(s.Test, Read)

		for _, st := range s.Body {
			r.visitStmt(st)
		}
		r.loopDepth--

	case *ast.Return:
		if !r.inFunction {
			r.error(s.Pos, "return outside function")
		}

		if s.Value != nil {
			r.visitExpr(s.Value, Read)
		}

	case *ast.Break:
		r.checkLoopContext(s.Pos, "break")

	case *ast.Continue:
		r.checkLoopContext(s.Pos, "continue")

	}
}

func (r *Resolver) checkLoopContext(pos ast.Range, keyword string) {
	if r.loopDepth == 0 {
		r.error(pos, keyword+" outside loop")
	}
}

func (r *Resolver) resolveName(e *ast.Name, ctx NameContext) {
	var sym *Symbol
	if ctx == Write {
		sym = r.current.Symbols[e.ID]

		if sym == nil {
			r.error(e.Pos, "internal error: write to undefined local "+e.ID)
			return
		}
	} else {
		var ok bool
		sym, ok = r.current.Lookup(e.ID)
		if !ok || sym == nil {
			r.error(e.Pos, ("undefined name: " + e.ID))
			return
		}
	}
	r.Resolved[e] = sym
}

func (r *Resolver) visitExpr(expr ast.Expression, ctx NameContext) {
	switch e := expr.(type) {
	case *ast.Name:
		r.resolveName(e, ctx)
		return

	case *ast.Number, *ast.String, *ast.Boolean:
		return
	case *ast.BinOp:
		r.visitExpr(e.Left, Read)
		r.visitExpr(e.Right, Read)

	case *ast.UnaryOp:
		r.visitExpr(e.Operand, Read)

	case *ast.BooleanOp:
		for _, v := range e.Values {
			r.visitExpr(v, Read)
		}

	case *ast.Compare:
		r.visitExpr(e.Left, Read)
		for _, rgt := range e.Right {
			r.visitExpr(rgt, Read)
		}

	case *ast.Call:
		r.visitExpr(e.Func, Read)
		for _, arg := range e.Args {
			r.visitExpr(arg, Read)
		}

	case *ast.Tuple:
		for _, elt := range e.Elts {
			r.visitExpr(elt, ctx)
		}

	case *ast.List:
		for _, elt := range e.Elts {
			r.visitExpr(elt, ctx)
		}

	case *ast.Attribute:
		r.visitExpr(e.Value, Read)
		r.PendingAttrs = append(r.PendingAttrs, PendingAttr{
			Node:     e,
			Class:    r.currentClass,
			SelfName: r.selfName,
		})

	default:
	}
}

func (r *Resolver) error(span ast.Range, msg string) {
	r.errors = append(r.errors, SemanticError{
		Span: span,
		Msg:  msg,
	})
}
