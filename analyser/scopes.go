package analyser

import (
	"rahu/parser"
)

type ScopeBuilder struct {
	current *Scope
}

func BuildScopes(module *parser.Module) *Scope {
	builtins := NewBuiltinScope()
	global := NewScope(builtins, ScopeGlobal)
	b := &ScopeBuilder{current: global}
	b.visitModule(module)
	return global
}

func (b *ScopeBuilder) visitModule(m *parser.Module) {
	for _, stmt := range m.Body {
		b.visitStmt(stmt)
	}
}

func (b *ScopeBuilder) visitStmt(stmt parser.Statement) {
	switch s := stmt.(type) {
	case *parser.Assign:
		b.visitAssign(s)
	case *parser.FunctionDef:
		b.visitFunctionDef(s)
	case *parser.If:
		b.visitIf(s)
	case *parser.For:
		b.visitFor(s)

	case *parser.WhileLoop:
		b.visitWhile(s)
	case *parser.ClassDef:
		b.visitClassDef(s)
	}
}

func (b *ScopeBuilder) visitWhile(w *parser.WhileLoop) {
	for _, stmt := range w.Body {
		b.visitStmt(stmt)
	}
}

func (b *ScopeBuilder) visitFor(f *parser.For) {
	if name, ok := f.Target.(*parser.Name); ok {
		_ = b.current.Define(&Symbol{
			Name: name.ID,
			Kind: SymVariable,
			Span: name.Pos,
		})
	}

	for _, stmt := range f.Body {
		b.visitStmt(stmt)
	}
}

func (b *ScopeBuilder) visitIf(i *parser.If) {
	for _, stm := range i.Body {
		b.visitStmt(stm)
	}

	for _, stm := range i.Orelse {
		b.visitStmt(stm)
	}
}

func (b *ScopeBuilder) visitAssign(a *parser.Assign) {
	for _, t := range a.Targets {
		if name, ok := t.(*parser.Name); ok {
			_ = b.current.Define(&Symbol{
				Name: name.ID,
				Kind: SymVariable,
				Span: name.Pos,
			})
		}
	}
}

func (b *ScopeBuilder) visitClassDef(c *parser.ClassDef) {
	classSym := &Symbol{
		Name: c.Name,
		Kind: SymClass,
		Span: c.Pos,
	}
	_ = b.current.Define(classSym)

	classScope := NewScope(b.current, ScopeClass)
	classSym.Inner = classScope
	prev := b.current
	b.current = classScope

	for _, stmt := range c.Body {
		b.visitStmt(stmt)
	}
	b.current = prev
}

func (b *ScopeBuilder) visitFunctionDef(f *parser.FunctionDef) {
	fnSym := &Symbol{
		Name: f.Name.ID,
		Kind: SymFunction,
		Span: f.NamePos,
	}

	_ = b.current.Define(fnSym)

	fnScope := NewScope(b.current, ScopeFunction)
	fnSym.Inner = fnScope
	prev := b.current
	b.current = fnScope

	for _, arg := range f.Args {
		_ = b.current.Define(&Symbol{
			Name: arg.Name.ID,
			Kind: SymParameter,
			Span: arg.Pos,
		})
	}

	for _, stmt := range f.Body {
		b.visitStmt(stmt)
	}

	b.current = prev
}
