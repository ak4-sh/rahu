package analyser

import "rahu/parser"

type NameContext int

const (
	Read NameContext = iota
	Write
)

type Resolver struct {
	current   *Scope
	errors    []SemanticError
	loopDepth int

	Resolved   map[*parser.Name]*Symbol
	inFunction bool
}

type SemanticError struct {
	Span parser.Range
	Msg  string
}

func Resolve(m *parser.Module, global *Scope) ([]SemanticError, map[*parser.Name]*Symbol) {
	r := &Resolver{
		current:  global,
		Resolved: make(map[*parser.Name]*Symbol),
	}
	r.visitModule(m)
	return r.errors, r.Resolved
}

func (r *Resolver) visitModule(m *parser.Module) {
	for _, stmt := range m.Body {
		r.visitStmt(stmt)
	}
}

func (r *Resolver) visitStmt(stmt parser.Statement) {
	switch s := stmt.(type) {
	case *parser.Assign:
		r.visitExpr(s.Value, Read)

		for _, t := range s.Targets {
			r.visitExpr(t, Write)
		}

	case *parser.FunctionDef:
		for _, arg := range s.Args {
			if arg.Default != nil {
				r.visitExpr(arg.Default, Read)
			}
		}

		fnSym := r.current.Symbols[s.Name]
		if fnSym == nil || fnSym.Inner == nil {
			r.error(s.Pos, "internal compiler error: missing function symbol or scope for "+s.Name)
			return
		}

		prevScope := r.current
		prevInFn := r.inFunction

		r.current = fnSym.Inner
		r.inFunction = true

		for _, stmt := range s.Body {
			r.visitStmt(stmt)
		}

		r.current = prevScope
		r.inFunction = prevInFn

	case *parser.ExprStmt:
		r.visitExpr(s.Value, Read)

	case *parser.If:
		r.visitExpr(s.Test, Read)

		for _, st := range s.Body {
			r.visitStmt(st)
		}

		for _, st := range s.Orelse {
			r.visitStmt(st)
		}

	case *parser.For:
		r.visitExpr(s.Iter, Read)
		r.loopDepth++
		r.visitExpr(s.Target, Write)

		for _, st := range s.Body {
			r.visitStmt(st)
		}
		r.loopDepth--

	case *parser.WhileLoop:
		r.loopDepth++
		r.visitExpr(s.Test, Read)

		for _, st := range s.Body {
			r.visitStmt(st)
		}
		r.loopDepth--

	case *parser.Return:
		if !r.inFunction {
			r.error(s.Pos, "return outside function")
		}

		if s.Value != nil {
			r.visitExpr(s.Value, Read)
		}

	case *parser.Break:
		if r.loopDepth == 0 {
			r.error(s.Pos, "break outside loop")
		}

	case *parser.Continue:
		if r.loopDepth == 0 {
			r.error(s.Pos, "continue outside loop")
		}

	}
}

func (r *Resolver) visitExpr(expr parser.Expression, ctx NameContext) {
	switch e := expr.(type) {
	case *parser.Name:
		var sym *Symbol
		// writes must resolve in current scope only
		// if not foun dthen scope builder impl is incorrect
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
		return

	case *parser.Number, *parser.String, *parser.Boolean:
		return
	case *parser.BinOp:
		r.visitExpr(e.Left, Read)
		r.visitExpr(e.Right, Read)

	case *parser.UnaryOp:
		r.visitExpr(e.Operand, Read)

	case *parser.BooleanOp:
		for _, v := range e.Values {
			r.visitExpr(v, Read)
		}

	case *parser.Compare:
		r.visitExpr(e.Left, Read)
		for _, rgt := range e.Right {
			r.visitExpr(rgt, Read)
		}

	case *parser.Call:
		r.visitExpr(e.Func, Read)
		for _, arg := range e.Args {
			r.visitExpr(arg, Read)
		}

	case *parser.Tuple:
		for _, elt := range e.Elts {
			r.visitExpr(elt, ctx)
		}

	case *parser.List:
		for _, elt := range e.Elts {
			r.visitExpr(elt, ctx)
		}
	default:
	}
}

func (r *Resolver) error(span parser.Range, msg string) {
	r.errors = append(r.errors, SemanticError{
		Span: span,
		Msg:  msg,
	})
}
