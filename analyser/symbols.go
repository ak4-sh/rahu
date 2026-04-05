// Package analyser implements semantic analysis over the parsed AST.
//
// It is responsible for building and managing symbol tables, resolving names,
// enforcing scoping rules, and reporting semantic diagnostics. The analyser
// operates after parsing and before any type checking or code generation.
package analyser

import (
	"fmt"

	"rahu/lsp"
	"rahu/parser/ast"
)

type (
	SymbolKind int
	SymbolID   uint64
	TypeKind   int
)

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
	SymField
)

const (
	TypeUnknown TypeKind = iota
	TypeInstance
	TypeClass
	TypeModule
	TypeBuiltin
	TypeUnion
	TypeList
	TypeTuple
	TypeDict
	TypeSet
)

type Type struct {
	Kind   TypeKind
	Symbol *Symbol
	Union  []*Type
	Elem   *Type
	Items  []*Type
	Key    *Type
}

type Symbol struct {
	Name       string
	Kind       SymbolKind
	Span       ast.Range
	Scope      *Scope
	Inner      *Scope
	Attrs      *Scope
	Members    *Scope
	Bases      []*Symbol
	InstanceOf *Symbol
	Inferred   *Type
	Returns    *Type
	DocString  string
	Def        ast.NodeID
	ID         SymbolID
	URI        lsp.DocumentURI
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
	for _, name := range []string{"True", "False", "None", "__name__"} {
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

	if listSym, ok := s.LookupLocal("list"); ok {
		listSym.Members = NewScope(nil, ScopeMember)
		listSym.Members.Define(&Symbol{
			Name:  "append",
			Kind:  SymFunction,
			Scope: listSym.Members,
			Span:  ast.Range{},
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

var builtinScope = NewBuiltinScope()

func UnknownType() *Type {
	return &Type{Kind: TypeUnknown}
}

func InstanceType(sym *Symbol) *Type {
	if sym == nil {
		return UnknownType()
	}
	return &Type{Kind: TypeInstance, Symbol: sym}
}

func ClassType(sym *Symbol) *Type {
	if sym == nil {
		return UnknownType()
	}
	return &Type{Kind: TypeClass, Symbol: sym}
}

func ModuleType(sym *Symbol) *Type {
	if sym == nil {
		return UnknownType()
	}
	return &Type{Kind: TypeModule, Symbol: sym}
}

func BuiltinType(sym *Symbol) *Type {
	if sym == nil {
		return UnknownType()
	}
	return &Type{Kind: TypeBuiltin, Symbol: sym}
}

func ListType(elem *Type) *Type {
	if elem == nil {
		elem = UnknownType()
	}
	return &Type{Kind: TypeList, Elem: elem}
}

func TupleType(items ...*Type) *Type {
	return &Type{Kind: TypeTuple, Items: items}
}

func DictType(key, value *Type) *Type {
	if key == nil {
		key = UnknownType()
	}
	if value == nil {
		value = UnknownType()
	}
	return &Type{Kind: TypeDict, Key: key, Elem: value}
}

func SetType(elem *Type) *Type {
	if elem == nil {
		elem = UnknownType()
	}
	return &Type{Kind: TypeSet, Elem: elem}
}

func SameType(a, b *Type) bool {
	if a == nil || b == nil {
		return a == b
	}
	if a.Kind != b.Kind {
		return false
	}
	switch a.Kind {
	case TypeUnknown:
		return true
	case TypeInstance, TypeClass, TypeModule, TypeBuiltin:
		return a.Symbol == b.Symbol
	case TypeList:
		return SameType(a.Elem, b.Elem)
	case TypeTuple:
		if len(a.Items) != len(b.Items) {
			return false
		}
		for i := range a.Items {
			if !SameType(a.Items[i], b.Items[i]) {
				return false
			}
		}
		return true
	case TypeDict:
		return SameType(a.Key, b.Key) && SameType(a.Elem, b.Elem)
	case TypeSet:
		return SameType(a.Elem, b.Elem)
	case TypeUnion:
		if len(a.Union) != len(b.Union) {
			return false
		}
		for i := range a.Union {
			if !SameType(a.Union[i], b.Union[i]) {
				return false
			}
		}
		return true
	default:
		return false
	}
}

func FlattenUnion(t *Type) []*Type {
	if t == nil || IsUnknownType(t) {
		return nil
	}
	if t.Kind != TypeUnion {
		return []*Type{t}
	}
	out := make([]*Type, 0, len(t.Union))
	for _, arm := range t.Union {
		out = append(out, FlattenUnion(arm)...)
	}
	return out
}

func NormalizeUnion(types ...*Type) *Type {
	flat := make([]*Type, 0, len(types))
	for _, t := range types {
		flat = append(flat, FlattenUnion(t)...)
	}
	uniq := make([]*Type, 0, len(flat))
	for _, t := range flat {
		if IsUnknownType(t) {
			continue
		}
		dup := false
		for _, existing := range uniq {
			if SameType(existing, t) {
				dup = true
				break
			}
		}
		if !dup {
			uniq = append(uniq, t)
		}
	}
	switch len(uniq) {
	case 0:
		return UnknownType()
	case 1:
		return uniq[0]
	default:
		return &Type{Kind: TypeUnion, Union: uniq}
	}
}

func UnionType(types ...*Type) *Type {
	return NormalizeUnion(types...)
}

func JoinTypes(types ...*Type) *Type {
	return UnionType(types...)
}

func IsUnknownType(t *Type) bool {
	return t == nil || t.Kind == TypeUnknown
}

func SymbolType(sym *Symbol) *Type {
	if sym == nil {
		return nil
	}
	if sym.Inferred != nil && !IsUnknownType(sym.Inferred) {
		return sym.Inferred
	}
	if sym.Name == "__name__" {
		return BuiltinType(BuiltinSymbol("str"))
	}
	if sym.InstanceOf != nil {
		return InstanceType(sym.InstanceOf)
	}
	switch sym.Kind {
	case SymClass:
		return ClassType(sym)
	case SymModule:
		return ModuleType(sym)
	case SymType, SymConstant:
		return BuiltinType(sym)
	default:
		if sym.Scope != nil && sym.Scope.Kind == ScopeBuiltin {
			return BuiltinType(sym)
		}
		return nil
	}
}

func MemberScopeForType(t *Type) *Scope {
	if IsUnknownType(t) {
		return nil
	}
	if t.Kind == TypeList {
		if listSym := BuiltinSymbol("list"); listSym != nil {
			return listSym.Members
		}
		return nil
	}
	if t.Kind == TypeSet {
		if setSym := BuiltinSymbol("set"); setSym != nil {
			return setSym.Members
		}
		return nil
	}
	if t.Kind == TypeDict {
		if dictSym := BuiltinSymbol("dict"); dictSym != nil {
			return dictSym.Members
		}
		return nil
	}
	if t.Symbol == nil {
		return nil
	}
	switch t.Kind {
	case TypeInstance, TypeClass:
		return t.Symbol.Members
	case TypeBuiltin:
		return t.Symbol.Members
	case TypeUnion:
		merged := NewScope(nil, ScopeMember)
		for _, arm := range t.Union {
			scope := MemberScopeForType(arm)
			if scope == nil {
				continue
			}
			for name, sym := range scope.Symbols {
				if _, exists := merged.Symbols[name]; !exists {
					merged.Symbols[name] = sym
				}
			}
		}
		if len(merged.Symbols) == 0 {
			return nil
		}
		return merged
	default:
		return nil
	}
}

func LookupMemberOnType(t *Type, name string) (*Symbol, bool) {
	if IsUnknownType(t) {
		return nil, false
	}
	if t.Kind == TypeUnion {
		for _, arm := range t.Union {
			if sym, ok := LookupMemberOnType(arm, name); ok {
				return sym, true
			}
		}
		return nil, false
	}
	members := MemberScopeForType(t)
	if members == nil {
		return nil, false
	}
	return members.Lookup(name)
}

func SubscriptResultType(t *Type) *Type {
	if IsUnknownType(t) {
		return nil
	}
	switch t.Kind {
	case TypeList:
		return t.Elem
	case TypeDict:
		return t.Elem
	case TypeTuple:
		if len(t.Items) == 0 {
			return UnknownType()
		}
		return JoinTypes(t.Items...)
	case TypeUnion:
		parts := make([]*Type, 0, len(t.Union))
		for _, arm := range t.Union {
			if result := SubscriptResultType(arm); !IsUnknownType(result) {
				parts = append(parts, result)
			}
		}
		return JoinTypes(parts...)
	default:
		return nil
	}
}

func BuiltinSymbol(name string) *Symbol {
	sym, ok := builtinScope.LookupLocal(name)
	if !ok {
		return nil
	}
	return sym
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
	case SymAttr:
		return "attribute"
	default:
		return "unknown"
	}
}

func (k ScopeKind) String() string {
	switch k {
	case ScopeGlobal:
		return "global"
	case ScopeFunction:
		return "function"
	case ScopeBlock:
		return "block"
	case ScopeBuiltin:
		return "builtin"
	case ScopeClass:
		return "class"
	case ScopeAttr:
		return "attr"
	case ScopeMember:
		return "member"
	default:
		return "unknown"
	}
}
