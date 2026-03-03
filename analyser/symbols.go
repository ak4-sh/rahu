// Package analyser implements semantic analysis over the parsed AST.
//
// It is responsible for building and managing symbol tables, resolving names,
// enforcing scoping rules, and reporting semantic diagnostics. The analyser
// operates after parsing and before any type checking or code generation.
package analyser

import (
	"fmt"

	"rahu/parser/ast"
)

type SymbolKind int

const (
	SymVariable SymbolKind = iota
	SymFunction
	SymParameter
	SymBuiltin
	SymClass
	SymModule
	SymImport
	SymConstant
	SymType
	SymAttr
)

type Symbol struct {
	Name    string
	Kind    SymbolKind
	Span    ast.Range
	Scope   *Scope
	Inner   *Scope
	Attrs   *Scope
	Members *Scope
}

type ScopeKind int

const (
	ScopeGlobal ScopeKind = iota
	ScopeFunction
	ScopeBlock
	ScopeBuiltin
	ScopeClass
	ScopeAttr
	ScopeMember
)

type Scope struct {
	Parent   *Scope
	Children []*Scope
	Symbols  map[string]*Symbol
	Kind     ScopeKind
	Owner    *Symbol
}

func NewScope(parent *Scope, kind ScopeKind) *Scope {
	scope := &Scope{
		Parent:  parent,
		Kind:    kind,
		Symbols: make(map[string]*Symbol),
	}

	if parent != nil {
		parent.Children = append(parent.Children, scope)
	}

	return scope
}

func NewBuiltinScope() *Scope {
	s := NewScope(nil, ScopeBuiltin)

	// populating constants
	for _, name := range []string{"True", "False", "None"} {
		s.Define(
			&Symbol{
				Name: name,
				Kind: SymConstant,
				Span: ast.Range{},
			})
	}

	// types
	for _, name := range []string{
		"int", "str", "float", "list", "tuple", "dict", "set",
		"frozenset", "bytes", "bytearray", "complex", "object",
	} {
		s.Define(&Symbol{
			Name: name,
			Kind: SymType,
			Span: ast.Range{},
		})
	}

	// populating pure funcs
	for _, name := range []string{
		"abs",
		"aiter",
		"all",
		"anext",
		"any",
		"ascii",
		"bin",
		"breakpoint",
		"callable",
		"chr",
		"classmethod",
		"compile",
		"delattr",
		"dir",
		"divmod",
		"enumerate",
		"eval",
		"exec",
		"filter",
		"format",
		"getattr",
		"hasattr",
		"globals",
		"hash",
		"help",
		"hex",
		"id",
		"input",
		"isinstance",
		"issubclass",
		"iter",
		"len",
		"locals",
		"map",
		"max",
		"memoryview",
		"min",
		"next",
		"oct",
		"open",
		"ord",
		"pow",
		"print",
		"property",
		"range",
		"repr",
		"reversed",
		"round",
		"setattr",
		"slice",
		"sorted",
		"staticmethod",
		"sum",
		"super",
		"type",
		"vars",
		"zip",
		"__import__",
	} {
		s.Define(&Symbol{
			Name: name,
			Kind: SymFunction,
			Span: ast.Range{},
		})
	}

	return s
}

func NewSymbol(name string, kind SymbolKind, span ast.Range) *Symbol {
	return &Symbol{
		Name: name,
		Kind: kind,
		Span: span,
	}
}

func (s *Scope) Define(sym *Symbol) error {
	if _, exists := s.Symbols[sym.Name]; exists {
		return fmt.Errorf("duplicate symbol: %s", sym.Name)
	}
	sym.Scope = s
	s.Symbols[sym.Name] = sym
	return nil
}

func (s *Scope) Lookup(name string) (*Symbol, bool) {
	for scope := s; scope != nil; scope = scope.Parent {
		if sym, ok := scope.Symbols[name]; ok {
			return sym, true
		}
	}
	return nil, false
}

func (s *Scope) LookupLocal(name string) (*Symbol, bool) {
	sym, ok := s.Symbols[name]
	return sym, ok
}

func (k SymbolKind) String() string {
	switch k {
	case SymBuiltin:
		return "builtin"
	case SymClass:
		return "class"
	case SymFunction:
		return "function"
	case SymImport:
		return "import"
	case SymParameter:
		return "parameter"
	case SymModule:
		return "module"
	case SymVariable:
		return "variable"
	case SymConstant:
		return "constant"
	case SymType:
		return "type"
	default:
		return "unknown"
	}
}

func (k ScopeKind) String() string {
	switch k {
	case ScopeBlock:
		return "block"
	case ScopeFunction:
		return "function"
	case ScopeGlobal:
		return "global"
	default:
		return "unknown"
	}
}
