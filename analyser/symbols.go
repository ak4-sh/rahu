// Package analyser implements semantic analysis over the parsed AST.
//
// It is responsible for building and managing symbol tables, resolving names,
// enforcing scoping rules, and reporting semantic diagnostics. The analyser
// operates after parsing and before any type checking or code generation.
package analyser

import (
	"fmt"

	"rahu/parser"
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
)

type Symbol struct {
	Name  string
	Kind  SymbolKind
	Span  parser.Range
	Scope *Scope
	Inner *Scope
}

type ScopeKind int

const (
	ScopeGlobal ScopeKind = iota
	ScopeFunction
	ScopeBlock
)

type Scope struct {
	Parent   *Scope
	Children []*Scope
	Symbols  map[string]*Symbol
	Kind     ScopeKind
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

func NewSymbol(name string, kind SymbolKind, span parser.Range) *Symbol {
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
