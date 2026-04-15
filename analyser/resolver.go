package analyser

import (
	"strings"

	"rahu/parser"
	"rahu/parser/ast"
)

type NameContext int

const (
	Read NameContext = iota
	Write
)

type PendingAttr struct {
	Node      ast.NodeID
	Class     *Symbol
	SelfName  string
	ValueType *Type // inferred type from assignment RHS (nil if not an assignment target)
}

type Resolver struct {
	tree       *ast.AST
	current    *Scope
	errors     []SemanticError
	loopDepth  int
	Resolved   map[ast.NodeID]*Symbol
	inFunction bool

	inClass      bool
	currentClass *Symbol
	ResolvedAttr map[ast.NodeID]*Symbol
	PendingAttrs []PendingAttr
	selfName     string
	ExprTypes    map[ast.NodeID]*Type

	// Cache for resolved stringified type annotations (forward references)
	// Maps annotation text to resolved type to avoid re-parsing and re-resolving
	stringAnnotCache map[string]*Type

	// Type constraints from isinstance() checks for type narrowing
	// Maps variable name to narrowed type within the current scope
	typeConstraints map[string]*Type

	// Inferred instance attributes for each class
	// Maps class SymbolID to map of attribute name -> union type
	classInstanceAttrs map[SymbolID]map[string]*Type
}

type SemanticError struct {
	Span ast.Range
	Msg  string
}

func newResolver(tree *ast.AST, global *Scope) *Resolver {
	resolvedCap := len(tree.Nodes) / 4
	if resolvedCap < 8 {
		resolvedCap = 8
	}
	attrCap := len(tree.Nodes) / 16
	if attrCap < 4 {
		attrCap = 4
	}
	exprTypeCap := len(tree.Nodes) / 32
	if exprTypeCap < 4 {
		exprTypeCap = 4
	}
	return &Resolver{
		tree:               tree,
		current:            global,
		errors:             nil,
		loopDepth:          0,
		Resolved:           make(map[ast.NodeID]*Symbol, resolvedCap),
		inFunction:         false,
		PendingAttrs:       make([]PendingAttr, 0, attrCap),
		ResolvedAttr:       make(map[ast.NodeID]*Symbol, attrCap),
		selfName:           "",
		ExprTypes:          make(map[ast.NodeID]*Type, exprTypeCap),
		stringAnnotCache:   make(map[string]*Type),
		typeConstraints:    make(map[string]*Type),
		classInstanceAttrs: make(map[SymbolID]map[string]*Type),
	}
}

func Resolve(tree *ast.AST, global *Scope) (*Resolver, []SemanticError) {
	r := newResolver(tree, global)
	r.visitModule()
	PromoteClassMembers(global)
	r.BindMembers()
	return r, r.errors
}

func (r *Resolver) visitModule() {
	if r.tree == nil {
		return
	}

	for stmt := r.tree.Nodes[r.tree.Root].FirstChild; stmt != ast.NoNode; stmt = r.tree.Nodes[stmt].NextSibling {
		if stmt != ast.NoNode {
			r.visitStmt(stmt)
		}
	}
}

func (r *Resolver) visitStmt(stmt ast.NodeID) {
	switch r.tree.Node(stmt).Kind {
	case ast.NodeAugAssign:
		target := r.tree.Nodes[stmt].FirstChild
		value := ast.NoNode
		if target != ast.NoNode {
			value = r.tree.Nodes[target].NextSibling
		}
		r.visitExpr(target, Read)
		r.visitExpr(value, Read)
		r.visitExpr(target, Write)

	case ast.NodeAssign:
		value := r.tree.Nodes[stmt].FirstChild
		if value == ast.NoNode {
			return
		}

		r.visitExpr(value, Read)
		valueType := r.ExprTypes[value]

		for target := r.tree.Nodes[value].NextSibling; target != ast.NoNode; target = r.tree.Nodes[target].NextSibling {
			r.visitExpr(target, Write)

			targetKind := r.tree.Node(target).Kind
			if targetKind == ast.NodeName {
				sym := r.Resolved[target]
				if sym != nil {
					if !IsUnknownType(valueType) {
						if sym.Inferred != nil && !IsUnknownType(sym.Inferred) {
							sym.Inferred = UnionType(sym.Inferred, valueType)
						} else {
							sym.Inferred = valueType
						}
						if valueType.Kind == TypeInstance && valueType.Symbol != nil {
							sym.InstanceOf = valueType.Symbol
						} else if valueType.Kind == TypeUnion {
							sym.InstanceOf = nil
						} else {
							sym.InstanceOf = nil
						}
						// Also set the expression type for this occurrence
						r.setExprType(target, valueType)
					}
				}
			} else if targetKind == ast.NodeAttribute && !IsUnknownType(valueType) {
				// Attribute was just added to PendingAttrs by visitExpr
				r.PendingAttrs[len(r.PendingAttrs)-1].ValueType = valueType

				// Infer instance attribute for class-level attribute tracking
				// When we see obj.attr = value, record that the class of obj has 'attr'
				base := r.tree.ChildAt(target, 0)
				attrNode := r.tree.ChildAt(target, 1)
				if base != ast.NoNode && attrNode != ast.NoNode {
					attrName, _ := r.tree.NameText(attrNode)
					if attrName != "" {
						baseType := r.exprType(base)
						if baseType != nil {
							// Get the class symbol from the type
							var classSym *Symbol
							switch baseType.Kind {
							case TypeInstance:
								classSym = baseType.Symbol
							case TypeClass:
								classSym = baseType.Symbol
							}
							if classSym != nil {
								r.recordInstanceAttr(classSym, attrName, valueType)
							}
						}
					}
				}
			}
		}

	case ast.NodeAnnAssign:
		target, annotation, value := r.tree.AnnAssignParts(stmt)
		annotType := r.resolveAnnotation(annotation)
		if value != ast.NoNode {
			r.visitExpr(value, Read)
			valueType := r.ExprTypes[value]
			if IsUnknownType(annotType) && !IsUnknownType(valueType) {
				annotType = valueType
			}
		}
		r.visitExpr(target, Write)
		targetKind := r.tree.Node(target).Kind
		if targetKind == ast.NodeName {
			sym := r.Resolved[target]
			if sym != nil && !IsUnknownType(annotType) {
				sym.Inferred = annotType
				if annotType.Kind == TypeInstance && annotType.Symbol != nil {
					sym.InstanceOf = annotType.Symbol
				} else {
					sym.InstanceOf = nil
				}
			}
		} else if targetKind == ast.NodeAttribute && !IsUnknownType(annotType) {
			// Attribute was just added to PendingAttrs by visitExpr
			r.PendingAttrs[len(r.PendingAttrs)-1].ValueType = annotType
		}

	case ast.NodeClassDef:
		for _, decorator := range r.tree.Decorators(stmt) {
			r.visitExpr(r.tree.DecoratorExpr(decorator), Read)
		}

		nameID, bases, body := r.tree.ClassParts(stmt)
		nameText, _ := r.tree.NameText(nameID)

		for base := r.tree.Nodes[bases].FirstChild; base != ast.NoNode; base = r.tree.Nodes[base].NextSibling {
			r.visitExpr(base, Read)
		}

		classSym := r.current.Symbols[nameText]
		if doc, ok := r.tree.DocString(stmt); ok && classSym != nil {
			classSym.DocString = doc
		}
		r.Resolved[nameID] = classSym

		for baseExpr := r.tree.Nodes[bases].FirstChild; baseExpr != ast.NoNode; baseExpr = r.tree.Nodes[baseExpr].NextSibling {
			baseSym, ok := r.resolveBaseClassSymbol(baseExpr)
			if !ok {
				continue
			}

			classSym.Bases = append(classSym.Bases, baseSym)
		}

		if classSym == nil || classSym.Inner == nil {
			r.error(r.tree.RangeOf(nameID), "internal compiler error: missing class symbol or scope for: "+nameText)
			return
		}

		prevScope := r.current
		prevClass := r.currentClass
		prevInClass := r.inClass
		prevSelf := r.selfName

		r.current = classSym.Inner
		r.currentClass = classSym
		r.inClass = true

		for inner := r.tree.Nodes[body].FirstChild; inner != ast.NoNode; inner = r.tree.Nodes[inner].NextSibling {
			r.visitStmt(inner)
		}

		r.current = prevScope
		r.currentClass = prevClass
		r.inClass = prevInClass
		r.selfName = prevSelf

	case ast.NodeFunctionDef:
		for _, decorator := range r.tree.Decorators(stmt) {
			r.visitExpr(r.tree.DecoratorExpr(decorator), Read)
		}

		nameID, args, returnAnnotation, body := r.tree.FunctionPartsWithReturn(stmt)
		nameText, _ := r.tree.NameText(nameID)

		fnSym := r.current.Symbols[nameText]
		if doc, ok := r.tree.DocString(stmt); ok && fnSym != nil {
			fnSym.DocString = doc
		}
		r.Resolved[nameID] = fnSym
		if fnSym == nil || fnSym.Inner == nil {
			r.error(r.tree.RangeOf(nameID), "internal compiler error: missing function symbol or scope for "+nameText)
			return
		}

		if args != ast.NoNode {
			for arg := r.tree.Nodes[args].FirstChild; arg != ast.NoNode; arg = r.tree.Nodes[arg].NextSibling {
				paramName, annotation, def := r.tree.ParamParts(arg)
				paramNameText, _ := r.tree.NameText(paramName)
				if annotation != ast.NoNode {
					if paramSym := fnSym.Inner.Symbols[paramNameText]; paramSym != nil {
						paramSym.Inferred = r.resolveAnnotation(annotation)
					}
				}
				if def != ast.NoNode {
					r.visitExpr(def, Read)
				}
			}
		}
		if returnAnnotation != ast.NoNode {
			fnSym.Returns = r.resolveAnnotation(returnAnnotation)
		}

		prevScope := r.current
		prevInFn := r.inFunction
		prevSelf := r.selfName

		if r.inClass && args != ast.NoNode && r.tree.Nodes[args].FirstChild != ast.NoNode {
			selfParam, _, _ := r.tree.ParamParts(r.tree.Nodes[args].FirstChild)
			selfName, _ := r.tree.NameText(selfParam)
			r.selfName = selfName

			// Set self parameter's type to instance of the current class
			if selfName != "" && r.currentClass != nil {
				if selfSym := fnSym.Inner.Symbols[selfName]; selfSym != nil {
					selfSym.Inferred = InstanceType(r.currentClass)
					selfSym.InstanceOf = r.currentClass
				}
			}
		} else {
			r.selfName = ""
		}

		r.current = fnSym.Inner
		r.inFunction = true

		for inner := r.tree.Nodes[body].FirstChild; inner != ast.NoNode; inner = r.tree.Nodes[inner].NextSibling {
			r.visitStmt(inner)
		}

		r.current = prevScope
		r.inFunction = prevInFn
		r.selfName = prevSelf

	case ast.NodeExprStmt:
		r.visitExpr(r.tree.Nodes[stmt].FirstChild, Read)

	case ast.NodeIf:
		test := r.tree.Nodes[stmt].FirstChild
		body := ast.NoNode
		orelse := ast.NoNode
		if test != ast.NoNode {
			body = r.tree.Nodes[test].NextSibling
		}
		if body != ast.NoNode {
			orelse = r.tree.Nodes[body].NextSibling
		}
		r.visitExpr(test, Read)

		// Check for isinstance() type narrowing
		narrowedVars := make(map[string]struct{})
		if varName, narrowedType, ok := r.extractIsinstanceCheck(test); ok {
			r.typeConstraints[varName] = narrowedType
			narrowedVars[varName] = struct{}{}
		}

		for inner := r.tree.Nodes[body].FirstChild; inner != ast.NoNode; inner = r.tree.Nodes[inner].NextSibling {
			r.visitStmt(inner)
		}

		// Remove type constraints after visiting the if-body
		for varName := range narrowedVars {
			delete(r.typeConstraints, varName)
		}

		for inner := r.tree.Nodes[orelse].FirstChild; inner != ast.NoNode; inner = r.tree.Nodes[inner].NextSibling {
			r.visitStmt(inner)
		}

	case ast.NodeAssert:
		test, msg := r.tree.AssertParts(stmt)
		r.visitExpr(test, Read)
		r.visitExpr(msg, Read)

	case ast.NodeDel:
		for _, target := range r.tree.DelTargets(stmt) {
			r.visitExpr(target, Read)
		}

	case ast.NodeGlobal, ast.NodeNonlocal:

	case ast.NodeFor:
		target := r.tree.Nodes[stmt].FirstChild
		iter := ast.NoNode
		body := ast.NoNode
		orelse := ast.NoNode
		if target != ast.NoNode {
			iter = r.tree.Nodes[target].NextSibling
		}
		if iter != ast.NoNode {
			body = r.tree.Nodes[iter].NextSibling
		}
		if body != ast.NoNode {
			orelse = r.tree.Nodes[body].NextSibling
		}
		r.visitExpr(iter, Read)
		r.loopDepth++
		r.visitExpr(target, Write)

		for inner := r.tree.Nodes[body].FirstChild; inner != ast.NoNode; inner = r.tree.Nodes[inner].NextSibling {
			r.visitStmt(inner)
		}
		for inner := r.tree.Nodes[orelse].FirstChild; inner != ast.NoNode; inner = r.tree.Nodes[inner].NextSibling {
			r.visitStmt(inner)
		}
		r.loopDepth--

	case ast.NodeWhile:
		test := r.tree.Nodes[stmt].FirstChild
		body := ast.NoNode
		if test != ast.NoNode {
			body = r.tree.Nodes[test].NextSibling
		}
		r.loopDepth++
		r.visitExpr(test, Read)

		for inner := r.tree.Nodes[body].FirstChild; inner != ast.NoNode; inner = r.tree.Nodes[inner].NextSibling {
			r.visitStmt(inner)
		}
		r.loopDepth--

	case ast.NodeReturn:
		if !r.inFunction {
			r.error(r.tree.RangeOf(stmt), "return outside function")
		}

		if value := r.tree.Nodes[stmt].FirstChild; value != ast.NoNode {
			r.visitExpr(value, Read)
		}

	case ast.NodeYield:
		if value := r.tree.Nodes[stmt].FirstChild; value != ast.NoNode {
			r.visitExpr(value, Read)
		}

	case ast.NodeRaise:
		exc, cause := r.tree.RaiseParts(stmt)
		r.visitExpr(exc, Read)
		r.visitExpr(cause, Read)

	case ast.NodePass:

	case ast.NodeBreak:
		r.checkLoopContext(r.tree.RangeOf(stmt), "break")

	case ast.NodeContinue:
		r.checkLoopContext(r.tree.RangeOf(stmt), "continue")

	case ast.NodeImport, ast.NodeFromImport:
	case ast.NodeTry:
		body, excepts, elseBlock, finallyBlock := r.tree.TryParts(stmt)
		for inner := r.tree.Nodes[body].FirstChild; inner != ast.NoNode; inner = r.tree.Nodes[inner].NextSibling {
			r.visitStmt(inner)
		}
		for _, exceptClause := range excepts {
			r.visitStmt(exceptClause)
		}
		for inner := r.tree.Nodes[elseBlock].FirstChild; inner != ast.NoNode; inner = r.tree.Nodes[inner].NextSibling {
			r.visitStmt(inner)
		}
		for inner := r.tree.Nodes[finallyBlock].FirstChild; inner != ast.NoNode; inner = r.tree.Nodes[inner].NextSibling {
			r.visitStmt(inner)
		}

	case ast.NodeWith:
		items, body := r.tree.WithParts(stmt)
		for _, item := range items {
			contextExpr, asTarget := r.tree.WithItemParts(item)
			r.visitExpr(contextExpr, Read)
			r.visitExpr(asTarget, Write)
		}
		for inner := r.tree.Nodes[body].FirstChild; inner != ast.NoNode; inner = r.tree.Nodes[inner].NextSibling {
			r.visitStmt(inner)
		}

	case ast.NodeExcept:
		excType, _, body := r.tree.ExceptParts(stmt)
		r.visitExpr(excType, Read)
		for inner := r.tree.Nodes[body].FirstChild; inner != ast.NoNode; inner = r.tree.Nodes[inner].NextSibling {
			r.visitStmt(inner)
		}
	}
}

func (r *Resolver) checkLoopContext(pos ast.Range, keyword string) {
	if r.loopDepth == 0 {
		r.error(pos, keyword+" outside loop")
	}
}

func (r *Resolver) resolveBaseClassSymbol(baseExpr ast.NodeID) (*Symbol, bool) {
	if baseExpr == ast.NoNode {
		return nil, false
	}

	var baseSym *Symbol
	var ok bool
	switch r.tree.Node(baseExpr).Kind {
	case ast.NodeName:
		baseSym = r.Resolved[baseExpr]
		ok = baseSym != nil
	case ast.NodeAttribute:
		baseSym, ok = r.resolveAttributeExpr(baseExpr)
	default:
		r.error(r.tree.RangeOf(baseExpr), "unsupported base class expression")
		return nil, false
	}

	baseName := r.baseExprName(baseExpr)
	if !ok || baseSym == nil {
		r.error(r.tree.RangeOf(baseExpr), "undefined base class: "+baseName)
		return nil, false
	}
	if baseSym.Kind == SymClass {
		return baseSym, true
	}
	if typ := SymbolType(baseSym); !IsUnknownType(typ) {
		switch typ.Kind {
		case TypeClass:
			if typ.Symbol != nil {
				return typ.Symbol, true
			}
		case TypeInstance:
			if typ.Symbol != nil && typ.Symbol.Kind == SymClass {
				return typ.Symbol, true
			}
		}
	}
	r.error(r.tree.RangeOf(baseExpr), baseName+" is not a class")
	return nil, false
}

func (r *Resolver) resolveAttributeExpr(expr ast.NodeID) (*Symbol, bool) {
	if expr == ast.NoNode || r.tree.Node(expr).Kind != ast.NodeAttribute {
		return nil, false
	}
	if sym := r.ResolvedAttr[expr]; sym != nil {
		return sym, true
	}

	base := r.tree.ChildAt(expr, 0)
	attrNameNode := r.tree.ChildAt(expr, 1)
	if base == ast.NoNode || attrNameNode == ast.NoNode {
		return nil, false
	}
	attrName, ok := r.tree.NameText(attrNameNode)
	if !ok {
		return nil, false
	}

	if baseType := r.exprType(base); baseType != nil {
		if sym, ok := LookupMemberOnType(baseType, attrName); ok {
			typ := SymbolType(sym)
			if !IsUnknownType(typ) {
				r.ResolvedAttr[expr] = sym
				r.setExprType(expr, typ)
				return sym, true
			}
			// Symbol found but has no type - check inferred attrs as fallback
		}

		// Check for inferred instance attributes (from dynamic attribute assignment)
		// When we see obj.attr = value in a method, we record that the class has 'attr'
		var classSym *Symbol
		switch baseType.Kind {
		case TypeInstance:
			classSym = baseType.Symbol
		case TypeClass:
			classSym = baseType.Symbol
		}
		if classSym != nil {
			if inferredType := r.getInferredInstanceAttr(classSym, attrName); inferredType != nil {
				// Create a synthetic symbol for the inferred attribute
				attrSym := &Symbol{
					Name:     attrName,
					Kind:     SymAttr,
					Inferred: inferredType,
					Scope:    classSym.Inner,
				}
				r.ResolvedAttr[expr] = attrSym
				r.setExprType(expr, inferredType)
				return attrSym, true
			}
		}

		// If we found a symbol earlier but it had no type, return it now as fallback
		if sym, ok := LookupMemberOnType(baseType, attrName); ok {
			r.ResolvedAttr[expr] = sym
			return sym, true
		}
	}

	baseSym := r.Resolved[base]
	if baseSym != nil && baseSym.InstanceOf != nil && baseSym.InstanceOf.Members != nil {
		if sym, ok := baseSym.InstanceOf.Members.Lookup(attrName); ok {
			r.ResolvedAttr[expr] = sym
			if typ := SymbolType(sym); !IsUnknownType(typ) {
				r.setExprType(expr, typ)
			}
			return sym, true
		}
	}

	return nil, false
}

func (r *Resolver) baseExprName(expr ast.NodeID) string {
	if expr == ast.NoNode {
		return ""
	}
	switch r.tree.Node(expr).Kind {
	case ast.NodeName:
		name, _ := r.tree.NameText(expr)
		return name
	case ast.NodeAttribute:
		base := r.tree.ChildAt(expr, 0)
		attr := r.tree.ChildAt(expr, 1)
		baseName := r.baseExprName(base)
		attrName, _ := r.tree.NameText(attr)
		if baseName == "" {
			return attrName
		}
		if attrName == "" {
			return baseName
		}
		return baseName + "." + attrName
	default:
		return ""
	}
}

func (r *Resolver) setExprType(id ast.NodeID, t *Type) {
	if id == ast.NoNode || IsUnknownType(t) {
		return
	}
	r.ExprTypes[id] = t
}

func (r *Resolver) exprType(id ast.NodeID) *Type {
	if id == ast.NoNode {
		return nil
	}
	return r.ExprTypes[id]
}

// InferReceiverTypeFromMethod attempts to determine the receiver type based on
// a method name. This enables backward type inference for builtin methods.
func InferReceiverTypeFromMethod(methodName string) *Type {
	switch methodName {
	case "split", "join", "lower", "upper", "strip", "replace", "find", "startswith", "endswith":
		// These are all str methods
		if sym := BuiltinSymbol("str"); sym != nil {
			return BuiltinType(sym)
		}
	case "append", "extend", "insert", "remove", "sort", "reverse":
		// These are list-only methods
		if sym := BuiltinSymbol("list"); sym != nil {
			return BuiltinType(sym)
		}
	case "get", "keys", "values", "items", "update":
		// These are dict-only methods
		if sym := BuiltinSymbol("dict"); sym != nil {
			return BuiltinType(sym)
		}
	case "pop":
		// pop exists on both list and dict - prefer dict for backward inference
		// (more common to call pop() on unknown dict than unknown list)
		if sym := BuiltinSymbol("dict"); sym != nil {
			return BuiltinType(sym)
		}
	case "clear":
		// clear exists on dict, list, and set - prefer dict
		if sym := BuiltinSymbol("dict"); sym != nil {
			return BuiltinType(sym)
		}
	}
	return nil
}

// recordInstanceAttr records an inferred instance attribute for a class.
// When we see `obj.attr = value` in a method, we infer that the class of `obj`
// has an instance attribute `attr`. The types are unioned if the attribute
// is set multiple times.
func (r *Resolver) recordInstanceAttr(classSym *Symbol, attrName string, typ *Type) {
	if classSym == nil || attrName == "" || typ == nil || IsUnknownType(typ) {
		return
	}

	if r.classInstanceAttrs[classSym.ID] == nil {
		r.classInstanceAttrs[classSym.ID] = make(map[string]*Type)
	}

	// If attribute already exists, union the types
	if existingType, ok := r.classInstanceAttrs[classSym.ID][attrName]; ok {
		r.classInstanceAttrs[classSym.ID][attrName] = JoinTypes(existingType, typ)
	} else {
		r.classInstanceAttrs[classSym.ID][attrName] = typ
	}
}

// getInferredInstanceAttr retrieves an inferred instance attribute type
// for a given class and attribute name.
func (r *Resolver) getInferredInstanceAttr(classSym *Symbol, attrName string) *Type {
	if classSym == nil || attrName == "" {
		return nil
	}

	// Check the class itself
	if attrs, ok := r.classInstanceAttrs[classSym.ID]; ok {
		if typ, ok := attrs[attrName]; ok {
			return typ
		}
	}

	// Check parent classes (inheritance)
	for _, base := range classSym.Bases {
		if typ := r.getInferredInstanceAttr(base, attrName); typ != nil {
			return typ
		}
	}

	return nil
}

// extractIsinstanceCheck attempts to extract type narrowing information from
// an isinstance() call. Returns (variableName, narrowedType, true) if the
// expression is isinstance(var, Type), otherwise returns ("", nil, false).
func (r *Resolver) extractIsinstanceCheck(expr ast.NodeID) (string, *Type, bool) {
	if expr == ast.NoNode || r.tree.Node(expr).Kind != ast.NodeCall {
		return "", nil, false
	}

	// Get the function being called
	funcExpr := r.tree.Node(expr).FirstChild
	if funcExpr == ast.NoNode {
		return "", nil, false
	}

	// Check if it's isinstance
	if r.tree.Node(funcExpr).Kind != ast.NodeName {
		return "", nil, false
	}

	funcName, _ := r.tree.NameText(funcExpr)
	if funcName != "isinstance" {
		return "", nil, false
	}

	// Get arguments - first should be variable name, second should be type
	// NodeCall children: func, arg1, arg2, ...
	arg1 := r.tree.Node(funcExpr).NextSibling
	if arg1 == ast.NoNode {
		return "", nil, false
	}

	// First arg must be a name node (the variable being checked)
	if r.tree.Node(arg1).Kind != ast.NodeName {
		return "", nil, false
	}

	varName, _ := r.tree.NameText(arg1)
	if varName == "" {
		return "", nil, false
	}

	// Get second argument (the type to check against)
	arg2 := r.tree.Node(arg1).NextSibling
	if arg2 == ast.NoNode {
		return "", nil, false
	}

	// Resolve the type argument
	narrowedType := r.resolveTypeFromExpr(arg2)
	if narrowedType == nil {
		return "", nil, false
	}

	return varName, narrowedType, true
}

// resolveTypeFromExpr resolves a type from an expression node.
// Used for extracting the type argument from isinstance() calls.
func (r *Resolver) resolveTypeFromExpr(expr ast.NodeID) *Type {
	if expr == ast.NoNode {
		return nil
	}

	switch r.tree.Node(expr).Kind {
	case ast.NodeName:
		name, _ := r.tree.NameText(expr)
		if name == "" {
			return nil
		}

		// Look up the name in scope
		if sym, ok := r.current.Lookup(name); ok && sym != nil {
			if sym.Kind == SymClass {
				return InstanceType(sym)
			}
			// For builtin types like str, int, etc.
			if typ := SymbolType(sym); !IsUnknownType(typ) {
				return typ
			}
		}

		// Try builtin symbols for types like str, int, etc.
		if builtin := BuiltinSymbol(name); builtin != nil {
			return BuiltinType(builtin)
		}

		return nil

	case ast.NodeSubScript:
		// Handle generic types like list[int], dict[str, int]
		return r.resolveSubscriptTypeExpr(expr)

	case ast.NodeAttribute:
		// Handle qualified names like typing.List, collections.abc.Sequence
		base := r.tree.ChildAt(expr, 0)
		attrNode := r.tree.ChildAt(expr, 1)
		if base == ast.NoNode || attrNode == ast.NoNode {
			return nil
		}

		baseName, _ := r.tree.NameText(base)
		_, _ = r.tree.NameText(attrNode) // attrName reserved for future use

		// Common patterns: typing.List, typing.Dict, etc.
		if baseName == "typing" || baseName == "collections" {
			// For now, return unknown type - full typing module support is complex
			// This could be enhanced to look up in imported modules
			return nil
		}

		return nil

	default:
		return nil
	}
}

// resolveSubscriptTypeExpr resolves a subscript expression as a type.
// Handles generic types like list[int], dict[str, int], etc.
func (r *Resolver) resolveSubscriptTypeExpr(expr ast.NodeID) *Type {
	base := r.tree.ChildAt(expr, 0)
	index := r.tree.ChildAt(expr, 1)
	if base == ast.NoNode || index == ast.NoNode || r.tree.Node(base).Kind != ast.NodeName {
		return nil
	}

	baseName, _ := r.tree.NameText(base)
	switch baseName {
	case "list":
		elemType := r.resolveTypeFromExpr(index)
		return ListType(elemType)
	case "tuple":
		if r.tree.Node(index).Kind == ast.NodeTuple {
			// tuple[int, str, ...] - multiple type arguments
			items := make([]*Type, 0, r.tree.ChildCount(index))
			for child := r.tree.Node(index).FirstChild; child != ast.NoNode; child = r.tree.Node(child).NextSibling {
				items = append(items, r.resolveTypeFromExpr(child))
			}
			return TupleType(items...)
		}
		return TupleType(r.resolveTypeFromExpr(index))
	case "dict":
		if r.tree.Node(index).Kind != ast.NodeTuple || r.tree.ChildCount(index) != 2 {
			return nil
		}
		key := r.tree.ChildAt(index, 0)
		value := r.tree.ChildAt(index, 1)
		return DictType(r.resolveTypeFromExpr(key), r.resolveTypeFromExpr(value))
	case "set":
		return SetType(r.resolveTypeFromExpr(index))
	case "type":
		// type[T] - represents the type T itself, not an instance
		// For isinstance checks, this is typically used as the second arg
		return r.resolveTypeFromExpr(index)
	default:
		return nil
	}
}

func (r *Resolver) resolveAnnotation(expr ast.NodeID) *Type {
	if expr == ast.NoNode {
		return nil
	}

	r.visitExpr(expr, Read)

	switch r.tree.Node(expr).Kind {
	case ast.NodeName:
		sym := r.Resolved[expr]
		if sym == nil {
			return nil
		}
		if sym.Kind == SymClass {
			return InstanceType(sym)
		}
		return SymbolType(sym)
	case ast.NodeString:
		// Handle stringified type annotations (forward references)
		return r.resolveStringAnnotation(expr)
	case ast.NodeSubScript:
		return r.resolveSubscriptAnnotation(expr)
	case ast.NodeTuple:
		items := make([]*Type, 0, r.tree.ChildCount(expr))
		for child := r.tree.Node(expr).FirstChild; child != ast.NoNode; child = r.tree.Node(child).NextSibling {
			items = append(items, r.resolveAnnotation(child))
		}
		return TupleType(items...)
	default:
		return nil
	}
}

// resolveStringAnnotation handles stringified type annotations (forward references)
// by parsing the string content and resolving the resulting type expression.
// Errors during parsing are silently ignored to match Python runtime behavior.
func (r *Resolver) resolveStringAnnotation(expr ast.NodeID) *Type {
	if expr == ast.NoNode {
		return nil
	}

	// Extract the string content
	text, ok := r.tree.StringText(expr)
	if !ok || text == "" {
		return nil
	}

	// Check cache first to avoid re-parsing the same annotation
	if cachedType, ok := r.stringAnnotCache[text]; ok {
		return cachedType
	}

	// Parse the string content by wrapping it in a dummy annotated assignment
	// This lets us reuse the existing parser infrastructure
	dummySrc := "_: " + text + "\n"
	subParser := parser.New(dummySrc)
	subTree := subParser.Parse()

	// Extract the annotation from the parsed dummy statement
	// The structure will be: NodeModule -> NodeAnnAssign -> [target, annotation]
	var parsedExpr ast.NodeID = ast.NoNode
	if subTree.Root != ast.NoNode {
		moduleNode := subTree.Root
		firstStmt := subTree.Node(moduleNode).FirstChild
		if firstStmt != ast.NoNode && subTree.Node(firstStmt).Kind == ast.NodeAnnAssign {
			_, annotation, _ := subTree.AnnAssignParts(firstStmt)
			parsedExpr = annotation
		}
	}

	if parsedExpr == ast.NoNode {
		// Parsing failed - cache nil and return
		// This matches Python's behavior where invalid string annotations
		// are ignored at runtime
		r.stringAnnotCache[text] = nil
		return nil
	}

	// Resolve the parsed expression
	// Note: The parsed expression comes from a different AST (subTree),
	// so we need special handling to resolve names in the current scope
	result := r.resolveParsedAnnotation(parsedExpr, subTree)

	// Cache the resolved type (even if nil, to avoid re-parsing invalid annotations)
	r.stringAnnotCache[text] = result

	return result
}

// resolveParsedAnnotation resolves a type from a parsed expression in a sub-AST.
// This handles expressions parsed from string annotations.
func (r *Resolver) resolveParsedAnnotation(expr ast.NodeID, subTree *ast.AST) *Type {
	if expr == ast.NoNode {
		return nil
	}

	switch subTree.Node(expr).Kind {
	case ast.NodeName:
		name, _ := subTree.NameText(expr)
		// Look up the name in the current scope
		if sym, ok := r.current.Lookup(name); ok && sym != nil {
			if sym.Kind == SymClass {
				return InstanceType(sym)
			}
			return SymbolType(sym)
		}
		return nil
	case ast.NodeSubScript:
		return r.resolveParsedSubscriptAnnotation(expr, subTree)
	case ast.NodeTuple:
		items := make([]*Type, 0, subTree.ChildCount(expr))
		for child := subTree.Node(expr).FirstChild; child != ast.NoNode; child = subTree.Node(child).NextSibling {
			items = append(items, r.resolveParsedAnnotation(child, subTree))
		}
		return TupleType(items...)
	case ast.NodeString:
		// Nested string annotation - resolve recursively using main cache
		nestedText, _ := subTree.StringText(expr)
		if nestedText == "" {
			return nil
		}
		// Check main cache first
		if cachedType, ok := r.stringAnnotCache[nestedText]; ok {
			return cachedType
		}
		return nil
	case ast.NodeNone:
		// None literal in annotation - resolve to NoneType
		if noneSym := BuiltinSymbol("NoneType"); noneSym != nil {
			return BuiltinType(noneSym)
		}
		return nil
	case ast.NodeBinOp:
		// Handle union types (X | Y) - Python 3.10+
		left := subTree.ChildAt(expr, 0)
		right := subTree.ChildAt(expr, 1)
		leftType := r.resolveParsedAnnotation(left, subTree)
		rightType := r.resolveParsedAnnotation(right, subTree)
		if leftType != nil && rightType != nil {
			return UnionType(leftType, rightType)
		}
		if leftType != nil {
			return leftType
		}
		return rightType
	default:
		return nil
	}
}

// resolveParsedSubscriptAnnotation handles subscript types like list[int], dict[str, Any]
// from a parsed sub-AST (string annotation)
func (r *Resolver) resolveParsedSubscriptAnnotation(expr ast.NodeID, subTree *ast.AST) *Type {
	base := subTree.ChildAt(expr, 0)
	index := subTree.ChildAt(expr, 1)
	if base == ast.NoNode || index == ast.NoNode || subTree.Node(base).Kind != ast.NodeName {
		return nil
	}

	baseName, _ := subTree.NameText(base)
	switch baseName {
	case "list":
		return ListType(r.resolveParsedAnnotation(index, subTree))
	case "tuple":
		if subTree.Node(index).Kind == ast.NodeTuple {
			items := make([]*Type, 0, subTree.ChildCount(index))
			for child := subTree.Node(index).FirstChild; child != ast.NoNode; child = subTree.Node(child).NextSibling {
				items = append(items, r.resolveParsedAnnotation(child, subTree))
			}
			return TupleType(items...)
		}
		return TupleType(r.resolveParsedAnnotation(index, subTree))
	case "dict":
		if subTree.Node(index).Kind != ast.NodeTuple || subTree.ChildCount(index) != 2 {
			return nil
		}
		key := subTree.ChildAt(index, 0)
		value := subTree.ChildAt(index, 1)
		return DictType(r.resolveParsedAnnotation(key, subTree), r.resolveParsedAnnotation(value, subTree))
	case "set":
		return SetType(r.resolveParsedAnnotation(index, subTree))
	default:
		return nil
	}
}

func (r *Resolver) resolveSubscriptAnnotation(expr ast.NodeID) *Type {
	base := r.tree.ChildAt(expr, 0)
	index := r.tree.ChildAt(expr, 1)
	if base == ast.NoNode || index == ast.NoNode || r.tree.Node(base).Kind != ast.NodeName {
		return nil
	}

	baseName, _ := r.tree.NameText(base)
	switch baseName {
	case "list":
		return ListType(r.resolveAnnotation(index))
	case "tuple":
		if r.tree.Node(index).Kind == ast.NodeTuple {
			items := make([]*Type, 0, r.tree.ChildCount(index))
			for child := r.tree.Node(index).FirstChild; child != ast.NoNode; child = r.tree.Node(child).NextSibling {
				items = append(items, r.resolveAnnotation(child))
			}
			return TupleType(items...)
		}
		return TupleType(r.resolveAnnotation(index))
	case "dict":
		if r.tree.Node(index).Kind != ast.NodeTuple || r.tree.ChildCount(index) != 2 {
			return nil
		}
		key := r.tree.ChildAt(index, 0)
		value := r.tree.ChildAt(index, 1)
		return DictType(r.resolveAnnotation(key), r.resolveAnnotation(value))
	case "set":
		return SetType(r.resolveAnnotation(index))
	default:
		return nil
	}
}

func (r *Resolver) resolveName(id ast.NodeID, ctx NameContext) {
	name, _ := r.tree.NameText(id)
	span := r.tree.RangeOf(id)

	var sym *Symbol
	if ctx == Write {
		sym = r.current.Symbols[name]

		if sym == nil {
			r.error(span, "internal error: write to undefined local "+name)
			return
		}
	} else {
		var ok bool
		sym, ok = r.current.Lookup(name)
		if !ok || sym == nil {
			r.error(span, "undefined name: "+name)
			return
		}
	}
	r.Resolved[id] = sym
}

func (r *Resolver) visitExpr(expr ast.NodeID, ctx NameContext) {
	if expr == ast.NoNode {
		return
	}

	switch r.tree.Node(expr).Kind {
	case ast.NodeName:
		r.resolveName(expr, ctx)
		if ctx == Read {
			// Check for type narrowing from isinstance() check
			name, _ := r.tree.NameText(expr)
			if narrowedType, ok := r.typeConstraints[name]; ok {
				// Use the narrowed type from isinstance() check
				r.setExprType(expr, narrowedType)
			} else {
				r.setExprType(expr, SymbolType(r.Resolved[expr]))
			}
		}
		return

	case ast.NodeNumber:
		if lit, ok := r.tree.NumberText(expr); ok {
			if strings.ContainsAny(lit, ".eE") {
				r.setExprType(expr, BuiltinType(BuiltinSymbol("float")))
			} else {
				r.setExprType(expr, BuiltinType(BuiltinSymbol("int")))
			}
		}
		return

	case ast.NodeString:
		r.setExprType(expr, BuiltinType(BuiltinSymbol("str")))
		return

	case ast.NodeBytes:
		r.setExprType(expr, BuiltinType(BuiltinSymbol("bytes")))
		return

	case ast.NodeFStringText:
		return

	case ast.NodeFString:
		for child := r.tree.Nodes[expr].FirstChild; child != ast.NoNode; child = r.tree.Nodes[child].NextSibling {
			r.visitExpr(child, Read)
		}
		r.setExprType(expr, BuiltinType(BuiltinSymbol("str")))
		return

	case ast.NodeFStringExpr:
		r.visitExpr(r.tree.ChildAt(expr, 0), Read)
		return

	case ast.NodeBoolean:
		r.setExprType(expr, BuiltinType(BuiltinSymbol("bool")))
		return

	case ast.NodeNone, ast.NodeErrExp:
		return

	case ast.NodeYield:
		if value := r.tree.Nodes[expr].FirstChild; value != ast.NoNode {
			r.visitExpr(value, Read)
		}
		return

	case ast.NodeBinOp:
		left := r.tree.Nodes[expr].FirstChild
		right := ast.NoNode
		if left != ast.NoNode {
			right = r.tree.Nodes[left].NextSibling
		}
		r.visitExpr(left, Read)
		r.visitExpr(right, Read)

	case ast.NodeUnaryOp:
		r.visitExpr(r.tree.Nodes[expr].FirstChild, Read)

	case ast.NodeBooleanOp:
		for child := r.tree.Nodes[expr].FirstChild; child != ast.NoNode; child = r.tree.Nodes[child].NextSibling {
			r.visitExpr(child, Read)
		}

	case ast.NodeCompare:
		left := r.tree.Nodes[expr].FirstChild
		if left == ast.NoNode {
			return
		}
		r.visitExpr(left, Read)
		for cmp := r.tree.Nodes[left].NextSibling; cmp != ast.NoNode; cmp = r.tree.Nodes[cmp].NextSibling {
			r.visitExpr(r.tree.Nodes[cmp].FirstChild, Read)
		}
		r.setExprType(expr, BuiltinType(BuiltinSymbol("bool")))

	case ast.NodeCall:
		funcID := r.tree.Nodes[expr].FirstChild
		for child := funcID; child != ast.NoNode; child = r.tree.Nodes[child].NextSibling {
			r.visitExpr(child, Read)
		}

		if r.tree.Node(funcID).Kind == ast.NodeName {
			sym := r.Resolved[funcID]
			if sym != nil && sym.Kind == SymClass {
				r.setExprType(expr, InstanceType(sym))
			} else if sym != nil && sym.Kind == SymFunction && !IsUnknownType(sym.Returns) {
				r.setExprType(expr, sym.Returns)
			} else if sym != nil && sym.Kind == SymType {
				r.setExprType(expr, BuiltinType(sym))
			}
		} else if r.tree.Node(funcID).Kind == ast.NodeAttribute {
			base := r.tree.ChildAt(funcID, 0)
			attr := r.tree.ChildAt(funcID, 1)
			attrName, _ := r.tree.NameText(attr)
			baseType := r.exprType(base)

			if attrName == "append" {
				arg := r.tree.Node(funcID).NextSibling
				argType := r.exprType(arg)
				if baseType != nil && baseType.Kind == TypeList && !IsUnknownType(argType) {
					baseType.Elem = JoinTypes(baseType.Elem, argType)
					if r.tree.Node(base).Kind == ast.NodeName {
						if baseSym := r.Resolved[base]; baseSym != nil {
							baseSym.Inferred = baseType
						}
					}
				}
			}
			// Infer return types for common str methods
			if baseType != nil && baseType.Kind == TypeBuiltin && baseType.Symbol != nil && baseType.Symbol.Name == "str" {
				switch attrName {
				case "split":
					r.setExprType(expr, ListType(BuiltinType(BuiltinSymbol("str"))))
				case "join":
					r.setExprType(expr, BuiltinType(BuiltinSymbol("str")))
				case "lower", "upper", "strip":
					r.setExprType(expr, BuiltinType(BuiltinSymbol("str")))
				}
			}

			// Infer return types for dict methods
			if baseType != nil && baseType.Kind == TypeDict {
				switch attrName {
				case "items":
					// Returns list[tuple[key_type, value_type]]
					tupleType := TupleType(baseType.Key, baseType.Elem)
					r.setExprType(expr, ListType(tupleType))
				case "keys":
					// Returns list[key_type]
					r.setExprType(expr, ListType(baseType.Key))
				case "values":
					// Returns list[value_type]
					r.setExprType(expr, ListType(baseType.Elem))
				case "get":
					// Returns value_type | None
					noneType := BuiltinType(BuiltinSymbol("NoneType"))
					r.setExprType(expr, UnionType(baseType.Elem, noneType))
				case "pop":
					// Returns value_type
					r.setExprType(expr, baseType.Elem)
				case "update", "clear":
					// Returns None
					r.setExprType(expr, BuiltinType(BuiltinSymbol("NoneType")))
				}
			}
		}

	case ast.NodeTuple, ast.NodeList:
		itemTypes := make([]*Type, 0)
		for child := r.tree.Nodes[expr].FirstChild; child != ast.NoNode; child = r.tree.Nodes[child].NextSibling {
			r.visitExpr(child, ctx)
			itemTypes = append(itemTypes, r.exprType(child))
		}
		if r.tree.Node(expr).Kind == ast.NodeList {
			elemType := UnknownType()
			if len(itemTypes) > 0 {
				elemType = JoinTypes(itemTypes...)
			}
			r.setExprType(expr, ListType(elemType))
		} else {
			r.setExprType(expr, TupleType(itemTypes...))
		}

	case ast.NodeListComp:
		r.visitListComp(expr)

	case ast.NodeDictComp:
		r.visitDictComp(expr)

	case ast.NodeDict:
		var keyType, elemType *Type
		for child := r.tree.Nodes[expr].FirstChild; child != ast.NoNode; child = r.tree.Nodes[child].NextSibling {
			r.visitExpr(child, Read)
			// Each child is a key-value pair (NodeTuple with 2 elements)
			if r.tree.Node(child).Kind == ast.NodeTuple {
				keyNode := r.tree.ChildAt(child, 0)
				valueNode := r.tree.ChildAt(child, 1)
				keyType = JoinTypes(keyType, r.exprType(keyNode))
				elemType = JoinTypes(elemType, r.exprType(valueNode))
			}
		}
		r.setExprType(expr, DictType(keyType, elemType))

	case ast.NodeKeywordArg:
		r.visitExpr(r.tree.ChildAt(expr, 1), Read)

	case ast.NodeStarArg, ast.NodeKwStarArg:
		r.visitExpr(r.tree.ChildAt(expr, 0), Read)

	case ast.NodeSubScript:
		base := r.tree.Nodes[expr].FirstChild
		index := ast.NoNode
		if base != ast.NoNode {
			index = r.tree.Nodes[base].NextSibling
		}
		r.visitExpr(base, Read)
		r.visitExpr(index, Read)
		if resultType := SubscriptResultType(r.exprType(base)); !IsUnknownType(resultType) {
			r.setExprType(expr, resultType)
		}

	case ast.NodeSlice:
		for child := r.tree.Nodes[expr].FirstChild; child != ast.NoNode; child = r.tree.Nodes[child].NextSibling {
			r.visitExpr(child, Read)
		}

	case ast.NodeAttribute:
		base := r.tree.Nodes[expr].FirstChild
		r.visitExpr(base, Read)

		// BACKWARD INFERENCE: If receiver type is unknown, try to infer from attribute name
		// This helps with unannotated function parameters that use methods like .split(), .append(), etc.
		if ctx == Read {
			baseType := r.exprType(base)
			if IsUnknownType(baseType) {
				attrName, _ := r.tree.NameText(r.tree.ChildAt(expr, 1))
				if inferredType := InferReceiverTypeFromMethod(attrName); inferredType != nil {
					// Update the receiver symbol's type and expression type
					if r.tree.Node(base).Kind == ast.NodeName {
						if baseSym := r.Resolved[base]; baseSym != nil {
							baseSym.Inferred = inferredType
							r.setExprType(base, inferredType)
							baseType = inferredType
						}
					}
				}
			}
		}

		if ctx == Read {
			if sym, ok := r.resolveAttributeExpr(expr); ok {
				if typ := SymbolType(sym); !IsUnknownType(typ) {
					r.setExprType(expr, typ)
				}
				return
			}
		}
		r.PendingAttrs = append(r.PendingAttrs, PendingAttr{
			Node:     expr,
			Class:    r.currentClass,
			SelfName: r.selfName,
		})
	}
}

func (r *Resolver) visitListComp(expr ast.NodeID) {
	resultExpr, clauses := r.tree.ListCompParts(expr)
	compScope := NewScope(r.current, ScopeBlock)
	prev := r.current
	r.current = compScope
	for _, clause := range clauses {
		r.visitComprehension(clause)
	}
	r.visitExpr(resultExpr, Read)
	r.current = prev
	r.setExprType(expr, ListType(r.exprType(resultExpr)))
}

func (r *Resolver) visitDictComp(expr ast.NodeID) {
	keyExpr, valueExpr, clauses := r.tree.DictCompParts(expr)
	compScope := NewScope(r.current, ScopeBlock)
	prev := r.current
	r.current = compScope
	for _, clause := range clauses {
		r.visitComprehension(clause)
	}
	r.visitExpr(keyExpr, Read)
	r.visitExpr(valueExpr, Read)
	r.current = prev
	r.setExprType(expr, DictType(r.exprType(keyExpr), r.exprType(valueExpr)))
}

func (r *Resolver) visitComprehension(id ast.NodeID) {
	target, iter, filters := r.tree.ComprehensionParts(id)
	r.visitExpr(iter, Read)
	r.defineComprehensionTarget(target)
	r.visitExpr(target, Write)
	r.assignTargetType(target, SubscriptResultType(r.exprType(iter)))
	for _, filter := range filters {
		r.visitExpr(filter, Read)
	}
}

func (r *Resolver) defineComprehensionTarget(target ast.NodeID) {
	if target == ast.NoNode {
		return
	}
	switch r.tree.Node(target).Kind {
	case ast.NodeName:
		name, _ := r.tree.NameText(target)
		if _, ok := r.current.Symbols[name]; ok {
			return
		}
		sym := &Symbol{Name: name, Kind: SymVariable, Span: r.tree.RangeOf(target), Def: target}
		_ = r.current.Define(sym)
	case ast.NodeTuple, ast.NodeList:
		for child := r.tree.Node(target).FirstChild; child != ast.NoNode; child = r.tree.Node(child).NextSibling {
			r.defineComprehensionTarget(child)
		}
	}
}

func (r *Resolver) assignTargetType(target ast.NodeID, typ *Type) {
	if target == ast.NoNode || IsUnknownType(typ) {
		return
	}
	switch r.tree.Node(target).Kind {
	case ast.NodeName:
		if sym := r.Resolved[target]; sym != nil {
			sym.Inferred = JoinTypes(sym.Inferred, typ)
		}
	case ast.NodeTuple, ast.NodeList:
		for child := r.tree.Node(target).FirstChild; child != ast.NoNode; child = r.tree.Node(child).NextSibling {
			r.assignTargetType(child, typ)
		}
	}
}

func (r *Resolver) error(span ast.Range, msg string) {
	r.errors = append(r.errors, SemanticError{
		Span: span,
		Msg:  msg,
	})
}
