package analyser

import (
	"rahu/parser/ast"
)

type ScopeBuilder struct {
	currentClass *Symbol
	current      *Scope
	inFunction   bool
	selfName     string
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
		switch tt := t.(type) {
		case *ast.Name:
			_ = b.current.Define(&Symbol{
				Name: tt.ID,
				Kind: SymVariable,
				Span: tt.Pos,
			})

		case *ast.Attribute:
			if b.currentClass == nil || !b.inFunction {
				break
			}
			base, ok := tt.Value.(*ast.Name)

			if !ok || base.ID != b.selfName {
				break
			}

			if b.currentClass.Attrs == nil {
				b.currentClass.Attrs = NewScope(nil, ScopeAttr)
			}

			_ = b.currentClass.Attrs.Define(&Symbol{
				Name: tt.Attr.ID,
				Kind: SymAttr,
				Span: tt.Attr.Pos,
			})

		}
	}
}

func (b *ScopeBuilder) visitClassDef(c *ast.ClassDef) {
	classScope := NewScope(b.current, ScopeClass)

	classSym := &Symbol{
		Name: c.Name.ID,
		Kind: SymClass,
		Span: c.Pos,
	}
	classScope.Owner = classSym
	_ = b.current.Define(classSym)

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
	fnScope := NewScope(b.current, ScopeFunction)

	fnSym := &Symbol{
		Name: f.Name.ID,
		Kind: SymFunction,
		Span: f.NamePos,
	}

	fnScope.Owner = fnSym

	_ = b.current.Define(fnSym)

	fnSym.Inner = fnScope
	prevSelf := b.selfName
	if b.current.Kind == ScopeClass && len(f.Args) > 0 {
		b.selfName = f.Args[0].Name.ID
	} else {
		b.selfName = ""
	}
	prev := b.current
	b.current = fnScope
	prevInFunc := b.inFunction
	b.inFunction = true

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
	b.selfName = prevSelf
	b.inFunction = prevInFunc
}
