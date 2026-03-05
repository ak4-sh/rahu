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
	ExprTypes    map[ast.Expression]*Symbol
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
		ExprTypes:    make(map[ast.Expression]*Symbol),
	}
}

func Resolve(m *ast.Module, global *Scope) (*Resolver, []SemanticError) {
	r := newResolver(global)
	r.visitModule(m)
	PromoteClassMembers(global)
	r.BindMembers()
	return r, r.errors
}

func (r *Resolver) visitModule(m *ast.Module) {
	if m != nil {
		for _, stmt := range m.Body {
			if stmt != nil {
				r.visitStmt(stmt)
			}
		}
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
		class := r.ExprTypes[s.Value]

		for _, t := range s.Targets {
			r.visitExpr(t, Write)

			if name, ok := t.(*ast.Name); ok && class != nil {
				sym := r.Resolved[name]
				if sym != nil {
					sym.InstanceOf = class
				}
			}
		}

	case *ast.ClassDef:
		for _, base := range s.Bases {
			if base != nil {
				r.visitExpr(base, Read)
			}
		}

		classSym := r.current.Symbols[s.Name.ID]
		if s.DocString != "" {
			classSym.DocString = s.DocString
		}
		r.Resolved[s.Name] = classSym

		for _, baseExpr := range s.Bases {
			if baseExpr == nil {
				continue
			}

			name, ok := baseExpr.(*ast.Name)
			if !ok {
				r.error(baseExpr.Position(), "unsupported base class expression")
				continue
			}

			baseSym, ok := r.current.Lookup(name.ID)
			if !ok || baseSym == nil {
				r.error(name.Pos, "undefined base class: "+name.ID)
			}

			if baseSym.Kind != SymClass {
				r.error(name.Pos, name.ID+" is not a class")
				continue
			}

			classSym.Bases = append(classSym.Bases, baseSym)
		}

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
		if s.DocString != "" {
			fnSym.DocString = s.DocString
		}
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

		if name, ok := e.Func.(*ast.Name); ok {
			sym := r.Resolved[name]

			if sym != nil && sym.Kind == SymClass {
				r.ExprTypes[e] = sym
			}
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
