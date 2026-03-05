package analyser

import (
	"fmt"

	"rahu/parser/ast"
)

type ScopeBuilder struct {
	currentClass *Symbol
	current      *Scope
	inFunction   bool
	selfName     string
	nextSymID    SymbolID
	Defs         map[ast.NodeID]*Symbol
}

func (b *ScopeBuilder) newSymID() SymbolID {
	b.nextSymID++
	return b.nextSymID
}

func (b *ScopeBuilder) define(scope *Scope, n *ast.Name, kind SymbolKind, span ast.Range) {
	sym := &Symbol{
		ID:   b.newSymID(),
		Name: n.Text,
		Kind: kind,
		Span: span,
		Def:  n.ID,
	}
	_ = scope.Define(sym)
	b.Defs[n.ID] = sym
}

func BuildScopes(module *ast.Module) (*Scope, map[ast.NodeID]*Symbol) {
	builtins := NewBuiltinScope()
	global := NewScope(builtins, ScopeGlobal)
	b := &ScopeBuilder{
		current: global,
		Defs:    make(map[ast.NodeID]*Symbol),
	}
	b.visitModule(module)
	return global, b.Defs
}

func (b *ScopeBuilder) visitModule(m *ast.Module) {
	for _, stmt := range m.Body {
		if stmt != nil {
			b.visitStmt(stmt)
		}
	}
}

func (b *ScopeBuilder) visitStmt(stmt ast.Statement) {
	switch s := stmt.(type) {
	case *ast.Assign:
		b.visitAssign(s)
	case *ast.FunctionDef:
		b.visitFunctionDef(s)
	case *ast.If:
		b.visitIf(s)
	case *ast.For:
		b.visitFor(s)

	case *ast.WhileLoop:
		b.visitWhile(s)
	case *ast.ClassDef:
		b.visitClassDef(s)

	case *ast.ExprStmt:
		b.visitExpr(s.Value)
	case *ast.Return:
		if s.Value != nil {
			b.visitExpr(s.Value)
		}
	case *ast.AugAssign:
		b.visitAugAssign(s)
	case *ast.Break:
	case *ast.Continue:
	default:
		panic(fmt.Sprintf("unhandled statement type %T", s))
	}
}

func (b *ScopeBuilder) visitAugAssign(a *ast.AugAssign) {
	switch tt := a.Target.(type) {
	case *ast.Name:
		b.define(b.current, tt, SymVariable, tt.Pos)

	case *ast.Attribute:
		if b.currentClass != nil && b.inFunction {
			base, ok := tt.Value.(*ast.Name)
			if ok && base.Text == b.selfName {
				if b.currentClass.Attrs == nil {
					b.currentClass.Attrs = NewScope(nil, ScopeAttr)
				}
				sym := &Symbol{
					Name: tt.Attr.Text,
					Kind: SymAttr,
					Span: tt.Attr.Pos,
					Def:  tt.Attr.ID,
					ID:   b.newSymID(),
				}
				_ = b.currentClass.Attrs.Define(sym)
				b.Defs[tt.Attr.ID] = sym
			}
		}
	}

	b.visitExpr(a.Value)
}

func (b *ScopeBuilder) visitExpr(value ast.Expression) {
	switch v := value.(type) {
	case *ast.Name:
	case *ast.Call:
		b.visitExpr(v.Func)
		for _, arg := range v.Args {
			b.visitExpr(arg)
		}

	case *ast.Attribute:
		b.visitExpr(v.Value)

	case *ast.BinOp:
		b.visitExpr(v.Left)
		b.visitExpr(v.Right)

	case *ast.UnaryOp:
		b.visitExpr(v.Operand)

	case *ast.Compare:
		b.visitExpr(v.Left)
		for _, c := range v.Right {
			b.visitExpr(c)
		}

	}
}

func (b *ScopeBuilder) visitWhile(w *ast.WhileLoop) {
	for _, stmt := range w.Body {
		b.visitStmt(stmt)
	}
}

func (b *ScopeBuilder) visitFor(f *ast.For) {
	if name, ok := f.Target.(*ast.Name); ok {
		b.define(b.current, name, SymVariable, name.Pos)
	}

	for _, stmt := range f.Body {
		b.visitStmt(stmt)
	}
}

func (b *ScopeBuilder) visitIf(i *ast.If) {
	for _, stm := range i.Body {
		b.visitStmt(stm)
	}

	for _, stm := range i.Orelse {
		b.visitStmt(stm)
	}
}

func (b *ScopeBuilder) visitAssign(a *ast.Assign) {
	for _, t := range a.Targets {
		switch tt := t.(type) {
		case *ast.Name:
			b.define(b.current, tt, SymVariable, tt.Pos)

		case *ast.Attribute:
			if b.currentClass == nil || !b.inFunction {
				break
			}
			base, ok := tt.Value.(*ast.Name)

			if !ok || base.Text != b.selfName {
				break
			}

			if b.currentClass.Attrs == nil {
				b.currentClass.Attrs = NewScope(nil, ScopeAttr)
			}

			sym := &Symbol{
				Name: tt.Attr.Text,
				Kind: SymAttr,
				Span: tt.Attr.Pos,
				Def:  tt.Attr.ID,
				ID:   b.newSymID(),
			}
			_ = b.currentClass.Attrs.Define(sym)
			b.Defs[tt.Attr.ID] = sym

		}
	}

	b.visitExpr(a.Value)
}

func (b *ScopeBuilder) visitClassDef(c *ast.ClassDef) {
	if c.Name.Text == "<incomplete>" {
		return
	}

	classScope := NewScope(b.current, ScopeClass)

	classSym := &Symbol{
		Name: c.Name.Text,
		Kind: SymClass,
		Span: c.Name.Pos,
		ID:   b.newSymID(),
		Def:  c.Name.ID,
	}
	classScope.Owner = classSym
	_ = b.current.Define(classSym)
	b.Defs[c.Name.ID] = classSym

	classSym.Inner = classScope
	prev := b.current
	prevClass := b.currentClass
	prevSelf := b.selfName
	b.current = classScope
	b.currentClass = classSym
	b.selfName = ""

	for _, stmt := range c.Body {
		b.visitStmt(stmt)
	}
	b.current = prev
	b.currentClass = prevClass
	b.selfName = prevSelf
}

func (b *ScopeBuilder) visitFunctionDef(f *ast.FunctionDef) {
	if f.Name.Text == "<incomplete>" {
		return
	}

	fnScope := NewScope(b.current, ScopeFunction)

	fnSym := &Symbol{
		Name: f.Name.Text,
		Kind: SymFunction,
		Span: f.NamePos,
		ID:   b.newSymID(),
		Def:  f.Name.ID,
	}

	fnScope.Owner = fnSym

	_ = b.current.Define(fnSym)
	b.Defs[f.Name.ID] = fnSym

	fnSym.Inner = fnScope
	prevSelf := b.selfName
	if b.current.Kind == ScopeClass && len(f.Args) > 0 {
		b.selfName = f.Args[0].Name.Text
	} else {
		b.selfName = ""
	}
	prev := b.current
	b.current = fnScope
	prevInFunc := b.inFunction
	b.inFunction = true

	for _, arg := range f.Args {
		b.define(b.current, arg.Name, SymParameter, arg.Pos)
	}

	for _, stmt := range f.Body {
		b.visitStmt(stmt)
	}

	b.current = prev
	b.selfName = prevSelf
	b.inFunction = prevInFunc
}
