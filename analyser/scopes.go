package analyser

import (
	"rahu/parser/ast"
)

type ScopeBuilder struct {
	current *Scope
}

func BuildScopes(module *ast.Module) *Scope {
	builtins := NewBuiltinScope()
	global := NewScope(builtins, ScopeGlobal)
	b := &ScopeBuilder{current: global}
	b.visitModule(module)
	return global
}

func (b *ScopeBuilder) visitModule(m *ast.Module) {
	for _, stmt := range m.Body {
		b.visitStmt(stmt)
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
	}
}

func (b *ScopeBuilder) visitWhile(w *ast.WhileLoop) {
	for _, stmt := range w.Body {
		b.visitStmt(stmt)
	}
}

func (b *ScopeBuilder) visitFor(f *ast.For) {
	if name, ok := f.Target.(*ast.Name); ok {
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
		if name, ok := t.(*ast.Name); ok {
			_ = b.current.Define(&Symbol{
				Name: name.ID,
				Kind: SymVariable,
				Span: name.Pos,
			})
		}
	}
}

func (b *ScopeBuilder) visitClassDef(c *ast.ClassDef) {
	classSym := &Symbol{
		Name: c.Name.ID,
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

func (b *ScopeBuilder) visitFunctionDef(f *ast.FunctionDef) {
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
