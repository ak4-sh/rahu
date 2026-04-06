package analyser

import "rahu/parser/ast"

type ScopeBuilder struct {
	tree         *ast.AST
	source       string // Source text for extracting default values
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

const maxDefaultValueLen = 50

// extractValue extracts the source text for a node, truncating if too long.
func (b *ScopeBuilder) extractValue(nodeID ast.NodeID) string {
	if nodeID == ast.NoNode || b.source == "" {
		return ""
	}
	r := b.tree.RangeOf(nodeID)
	if int(r.End) > len(b.source) {
		return ""
	}
	val := b.source[r.Start:r.End]
	if len(val) > maxDefaultValueLen {
		return val[:maxDefaultValueLen] + "..."
	}
	return val
}

func (b *ScopeBuilder) define(scope *Scope, nameID ast.NodeID, kind SymbolKind, span ast.Range) *Symbol {
	name, ok := b.tree.NameText(nameID)
	if !ok {
		return nil
	}

	sym := &Symbol{
		ID:   b.newSymID(),
		Name: name,
		Kind: kind,
		Span: span,
		Def:  nameID,
	}
	_ = scope.Define(sym)
	b.Defs[nameID] = sym
	return sym
}

func BuildScopes(tree *ast.AST, source string) (*Scope, map[ast.NodeID]*Symbol) {
	global := NewScope(builtinScope, ScopeGlobal)
	defsCap := len(tree.Nodes) / 8
	if defsCap < 8 {
		defsCap = 8
	}
	b := &ScopeBuilder{
		tree:    tree,
		source:  source,
		current: global,
		Defs:    make(map[ast.NodeID]*Symbol, defsCap),
	}
	b.visitModule()
	return global, b.Defs
}

func (b *ScopeBuilder) visitModule() {
	if b.tree == nil {
		return
	}

	for stmt := b.tree.Nodes[b.tree.Root].FirstChild; stmt != ast.NoNode; stmt = b.tree.Nodes[stmt].NextSibling {
		if stmt != ast.NoNode {
			b.visitStmt(stmt)
		}
	}
}

func (b *ScopeBuilder) visitStmt(stmt ast.NodeID) {
	switch b.tree.Node(stmt).Kind {
	case ast.NodeAssign:
		b.visitAssign(stmt)
	case ast.NodeAnnAssign:
		b.visitAnnAssign(stmt)
	case ast.NodeFunctionDef:
		b.visitFunctionDef(stmt)
	case ast.NodeIf:
		b.visitIf(stmt)
	case ast.NodeFor:
		b.visitFor(stmt)
	case ast.NodeWhile:
		b.visitWhile(stmt)
	case ast.NodeClassDef:
		b.visitClassDef(stmt)
	case ast.NodeTry:
		b.visitTry(stmt)
	case ast.NodeExcept:
		b.visitExcept(stmt)
	case ast.NodeWith:
		b.visitWith(stmt)
	case ast.NodeExprStmt:
		b.visitExpr(b.tree.Nodes[stmt].FirstChild)
	case ast.NodeReturn:
		if value := b.tree.Nodes[stmt].FirstChild; value != ast.NoNode {
			b.visitExpr(value)
		}
	case ast.NodeRaise:
		exc, cause := b.tree.RaiseParts(stmt)
		b.visitExpr(exc)
		b.visitExpr(cause)
	case ast.NodeAugAssign:
		b.visitAugAssign(stmt)
	case ast.NodeImport:
		b.visitImport(stmt)
	case ast.NodeFromImport:
		b.visitFromImport(stmt)
	case ast.NodePass, ast.NodeBreak, ast.NodeContinue, ast.NodeErrStmt:
	default:
		return
	}
}

func importBoundName(tree *ast.AST, target ast.NodeID) ast.NodeID {
	if target == ast.NoNode {
		return ast.NoNode
	}

	if tree.Node(target).Kind == ast.NodeName {
		return target
	}

	if tree.Node(target).Kind != ast.NodeAttribute {
		return ast.NoNode
	}

	base := tree.ChildAt(target, 0)
	if base == ast.NoNode {
		return ast.NoNode
	}

	return importBoundName(tree, base)
}

func (b *ScopeBuilder) visitImport(id ast.NodeID) {
	for alias := b.tree.Node(id).FirstChild; alias != ast.NoNode; alias = b.tree.Node(alias).NextSibling {
		target, asName := b.tree.AliasParts(alias)
		bound := asName
		if bound == ast.NoNode {
			bound = importBoundName(b.tree, target)
		}
		if bound != ast.NoNode {
			b.define(b.current, bound, SymImport, b.tree.RangeOf(bound))
		}
	}
}

func (b *ScopeBuilder) visitFromImport(id ast.NodeID) {
	_, aliases := b.tree.FromImportParts(id)
	for _, alias := range aliases {
		target, asName := b.tree.AliasParts(alias)
		bound := asName
		if bound == ast.NoNode {
			bound = target
		}
		if bound != ast.NoNode {
			b.define(b.current, bound, SymImport, b.tree.RangeOf(bound))
		}
	}
}

func (b *ScopeBuilder) visitAugAssign(id ast.NodeID) {
	target := b.tree.Nodes[id].FirstChild
	value := ast.NoNode
	if target != ast.NoNode {
		value = b.tree.Nodes[target].NextSibling
	}

	switch b.tree.Node(target).Kind {
	case ast.NodeName:
		b.define(b.current, target, SymVariable, b.tree.RangeOf(target))

	case ast.NodeAttribute:
		if b.currentClass != nil && b.inFunction {
			base := b.tree.Nodes[target].FirstChild
			attr := ast.NoNode
			if base != ast.NoNode {
				attr = b.tree.Nodes[base].NextSibling
			}
			baseName, _ := b.tree.NameText(base)
			attrName, _ := b.tree.NameText(attr)
			if b.tree.Node(base).Kind == ast.NodeName && baseName == b.selfName {
				if b.currentClass.Attrs == nil {
					b.currentClass.Attrs = NewScope(nil, ScopeAttr)
				}
				sym := &Symbol{
					Name: attrName,
					Kind: SymAttr,
					Span: b.tree.RangeOf(attr),
					Def:  attr,
					ID:   b.newSymID(),
				}
				_ = b.currentClass.Attrs.Define(sym)
				b.Defs[attr] = sym
			}
		}

	case ast.NodeSubScript:
		b.visitExpr(target)
	}

	b.visitExpr(value)
}

func (b *ScopeBuilder) visitExpr(id ast.NodeID) {
	if id == ast.NoNode {
		return
	}

	switch b.tree.Node(id).Kind {
	case ast.NodeName, ast.NodeNumber, ast.NodeString, ast.NodeFStringText, ast.NodeBoolean, ast.NodeNone, ast.NodeErrExp:
		return

	case ast.NodeFString:
		for child := b.tree.Nodes[id].FirstChild; child != ast.NoNode; child = b.tree.Nodes[child].NextSibling {
			b.visitExpr(child)
		}
		return

	case ast.NodeFStringExpr:
		b.visitExpr(b.tree.ChildAt(id, 0))
		return

	case ast.NodeListComp:
		b.visitListComp(id)
		return

	case ast.NodeDictComp:
		b.visitDictComp(id)
		return

	case ast.NodeComprehension:
		b.visitComprehension(id)
		return

	case ast.NodeCall:
		for child := b.tree.Nodes[id].FirstChild; child != ast.NoNode; child = b.tree.Nodes[child].NextSibling {
			b.visitExpr(child)
		}

	case ast.NodeKeywordArg:
		b.visitExpr(b.tree.ChildAt(id, 1))

	case ast.NodeStarArg, ast.NodeKwStarArg:
		b.visitExpr(b.tree.ChildAt(id, 0))

	case ast.NodeSubScript:
		base := b.tree.Nodes[id].FirstChild
		index := ast.NoNode
		if base != ast.NoNode {
			index = b.tree.Nodes[base].NextSibling
		}
		b.visitExpr(base)
		b.visitExpr(index)

	case ast.NodeSlice:
		for child := b.tree.Nodes[id].FirstChild; child != ast.NoNode; child = b.tree.Nodes[child].NextSibling {
			b.visitExpr(child)
		}

	case ast.NodeAttribute:
		b.visitExpr(b.tree.Nodes[id].FirstChild)

	case ast.NodeBinOp:
		left := b.tree.Nodes[id].FirstChild
		right := ast.NoNode
		if left != ast.NoNode {
			right = b.tree.Nodes[left].NextSibling
		}
		b.visitExpr(left)
		b.visitExpr(right)

	case ast.NodeUnaryOp:
		b.visitExpr(b.tree.Nodes[id].FirstChild)

	case ast.NodeCompare:
		left := b.tree.Nodes[id].FirstChild
		if left == ast.NoNode {
			return
		}
		b.visitExpr(left)
		for cmp := b.tree.Nodes[left].NextSibling; cmp != ast.NoNode; cmp = b.tree.Nodes[cmp].NextSibling {
			b.visitExpr(b.tree.Nodes[cmp].FirstChild)
		}

	case ast.NodeTuple, ast.NodeList, ast.NodeDict, ast.NodeBooleanOp:
		for child := b.tree.Nodes[id].FirstChild; child != ast.NoNode; child = b.tree.Nodes[child].NextSibling {
			b.visitExpr(child)
		}
	}
}

func (b *ScopeBuilder) defineTargetPattern(id ast.NodeID) {
	if id == ast.NoNode {
		return
	}
	switch b.tree.Node(id).Kind {
	case ast.NodeName:
		b.define(b.current, id, SymVariable, b.tree.RangeOf(id))
	case ast.NodeTuple, ast.NodeList:
		for child := b.tree.Node(id).FirstChild; child != ast.NoNode; child = b.tree.Node(child).NextSibling {
			b.defineTargetPattern(child)
		}
	}
}

func (b *ScopeBuilder) visitListComp(id ast.NodeID) {
	expr, clauses := b.tree.ListCompParts(id)
	compScope := NewScope(b.current, ScopeBlock)
	prev := b.current
	prevClass := b.currentClass
	b.current = compScope
	b.currentClass = nil
	for _, clause := range clauses {
		b.visitComprehension(clause)
	}
	b.visitExpr(expr)
	b.current = prev
	b.currentClass = prevClass
}

func (b *ScopeBuilder) visitDictComp(id ast.NodeID) {
	key, value, clauses := b.tree.DictCompParts(id)
	compScope := NewScope(b.current, ScopeBlock)
	prev := b.current
	prevClass := b.currentClass
	b.current = compScope
	b.currentClass = nil
	for _, clause := range clauses {
		b.visitComprehension(clause)
	}
	b.visitExpr(key)
	b.visitExpr(value)
	b.current = prev
	b.currentClass = prevClass
}

func (b *ScopeBuilder) visitComprehension(id ast.NodeID) {
	target, iter, filters := b.tree.ComprehensionParts(id)
	b.visitExpr(iter)
	b.defineTargetPattern(target)
	for _, filter := range filters {
		b.visitExpr(filter)
	}
}

func (b *ScopeBuilder) visitWhile(id ast.NodeID) {
	test := b.tree.Nodes[id].FirstChild
	body := ast.NoNode
	if test != ast.NoNode {
		body = b.tree.Nodes[test].NextSibling
	}
	for stmt := b.tree.Nodes[body].FirstChild; stmt != ast.NoNode; stmt = b.tree.Nodes[stmt].NextSibling {
		b.visitStmt(stmt)
	}
}

func (b *ScopeBuilder) visitFor(id ast.NodeID) {
	target := b.tree.Nodes[id].FirstChild
	b.defineTargetPattern(target)

	iter := ast.NoNode
	body := ast.NoNode
	if target != ast.NoNode {
		iter = b.tree.Nodes[target].NextSibling
	}
	if iter != ast.NoNode {
		body = b.tree.Nodes[iter].NextSibling
	}
	for stmt := b.tree.Nodes[body].FirstChild; stmt != ast.NoNode; stmt = b.tree.Nodes[stmt].NextSibling {
		b.visitStmt(stmt)
	}

	orelse := ast.NoNode
	if body != ast.NoNode {
		orelse = b.tree.Nodes[body].NextSibling
	}
	if orelse != ast.NoNode {
		for stmt := b.tree.Nodes[orelse].FirstChild; stmt != ast.NoNode; stmt = b.tree.Nodes[stmt].NextSibling {
			b.visitStmt(stmt)
		}
	}
}

func (b *ScopeBuilder) visitIf(id ast.NodeID) {
	test := b.tree.Nodes[id].FirstChild
	body := ast.NoNode
	if test != ast.NoNode {
		body = b.tree.Nodes[test].NextSibling
	}
	for stmt := b.tree.Nodes[body].FirstChild; stmt != ast.NoNode; stmt = b.tree.Nodes[stmt].NextSibling {
		b.visitStmt(stmt)
	}

	orelse := ast.NoNode
	if body != ast.NoNode {
		orelse = b.tree.Nodes[body].NextSibling
	}
	if orelse != ast.NoNode {
		for stmt := b.tree.Nodes[orelse].FirstChild; stmt != ast.NoNode; stmt = b.tree.Nodes[stmt].NextSibling {
			b.visitStmt(stmt)
		}
	}
}

func (b *ScopeBuilder) visitTry(id ast.NodeID) {
	body, excepts, elseBlock, finallyBlock := b.tree.TryParts(id)
	if body != ast.NoNode {
		for stmt := b.tree.Node(body).FirstChild; stmt != ast.NoNode; stmt = b.tree.Node(stmt).NextSibling {
			b.visitStmt(stmt)
		}
	}
	for _, exceptClause := range excepts {
		b.visitExcept(exceptClause)
	}
	if elseBlock != ast.NoNode {
		for stmt := b.tree.Node(elseBlock).FirstChild; stmt != ast.NoNode; stmt = b.tree.Node(stmt).NextSibling {
			b.visitStmt(stmt)
		}
	}
	if finallyBlock != ast.NoNode {
		for stmt := b.tree.Node(finallyBlock).FirstChild; stmt != ast.NoNode; stmt = b.tree.Node(stmt).NextSibling {
			b.visitStmt(stmt)
		}
	}
}

func (b *ScopeBuilder) visitExcept(id ast.NodeID) {
	excType, asName, body := b.tree.ExceptParts(id)
	b.visitExpr(excType)
	if asName != ast.NoNode {
		b.define(b.current, asName, SymVariable, b.tree.RangeOf(asName))
	}
	if body != ast.NoNode {
		for stmt := b.tree.Node(body).FirstChild; stmt != ast.NoNode; stmt = b.tree.Node(stmt).NextSibling {
			b.visitStmt(stmt)
		}
	}
}

func (b *ScopeBuilder) visitWith(id ast.NodeID) {
	items, body := b.tree.WithParts(id)
	for _, item := range items {
		contextExpr, asTarget := b.tree.WithItemParts(item)
		b.visitExpr(contextExpr)
		b.defineTargetPattern(asTarget)
	}
	if body != ast.NoNode {
		for stmt := b.tree.Node(body).FirstChild; stmt != ast.NoNode; stmt = b.tree.Node(stmt).NextSibling {
			b.visitStmt(stmt)
		}
	}
}

func (b *ScopeBuilder) visitAssign(id ast.NodeID) {
	firstValue := b.tree.Nodes[id].FirstChild
	value := firstValue
	for target := ast.NoNode; value != ast.NoNode; {
		target = b.tree.Nodes[value].NextSibling
		if target == ast.NoNode {
			break
		}

		switch b.tree.Node(target).Kind {
		case ast.NodeName:
			sym := b.define(b.current, target, SymVariable, b.tree.RangeOf(target))
			if sym != nil {
				sym.DefaultValue = b.extractValue(firstValue)
			}

		case ast.NodeAttribute:
			if b.currentClass == nil || !b.inFunction {
				break
			}

			base := b.tree.Nodes[target].FirstChild
			attr := ast.NoNode
			if base != ast.NoNode {
				attr = b.tree.Nodes[base].NextSibling
			}
			baseName, _ := b.tree.NameText(base)
			attrName, _ := b.tree.NameText(attr)
			if b.tree.Node(base).Kind != ast.NodeName || baseName != b.selfName {
				break
			}

			if b.currentClass.Attrs == nil {
				b.currentClass.Attrs = NewScope(nil, ScopeAttr)
			}

			sym := &Symbol{
				Name: attrName,
				Kind: SymAttr,
				Span: b.tree.RangeOf(attr),
				Def:  attr,
				ID:   b.newSymID(),
			}
			_ = b.currentClass.Attrs.Define(sym)
			b.Defs[attr] = sym

		case ast.NodeSubScript:
			b.visitExpr(target)
		}

		value = target
	}

	if value != ast.NoNode {
		b.visitExpr(value)
	}
}

func (b *ScopeBuilder) visitAnnAssign(id ast.NodeID) {
	target, annotation, value := b.tree.AnnAssignParts(id)
	if target == ast.NoNode {
		return
	}

	if b.tree.Node(target).Kind == ast.NodeName {
		sym := b.define(b.current, target, SymVariable, b.tree.RangeOf(target))
		if sym != nil && value != ast.NoNode {
			sym.DefaultValue = b.extractValue(value)
		}
	}
	b.visitExpr(annotation)
	b.visitExpr(value)
}

func (b *ScopeBuilder) visitClassDef(id ast.NodeID) {
	for _, decorator := range b.tree.Decorators(id) {
		b.visitExpr(b.tree.DecoratorExpr(decorator))
	}

	name, _, body := b.tree.ClassParts(id)
	nameText, _ := b.tree.NameText(name)
	if nameText == "<incomplete>" {
		return
	}

	classScope := NewScope(b.current, ScopeClass)

	classSym := &Symbol{
		Name: nameText,
		Kind: SymClass,
		Span: b.tree.RangeOf(name),
		ID:   b.newSymID(),
		Def:  name,
	}
	classScope.Owner = classSym
	_ = b.current.Define(classSym)
	b.Defs[name] = classSym

	classSym.Inner = classScope
	prev := b.current
	prevClass := b.currentClass
	prevSelf := b.selfName
	b.current = classScope
	b.currentClass = classSym
	b.selfName = ""

	for stmt := b.tree.Nodes[body].FirstChild; stmt != ast.NoNode; stmt = b.tree.Nodes[stmt].NextSibling {
		b.visitStmt(stmt)
	}

	b.current = prev
	b.currentClass = prevClass
	b.selfName = prevSelf
}

func (b *ScopeBuilder) visitFunctionDef(id ast.NodeID) {
	for _, decorator := range b.tree.Decorators(id) {
		b.visitExpr(b.tree.DecoratorExpr(decorator))
	}

	name, args, body := b.tree.FunctionParts(id)
	nameText, _ := b.tree.NameText(name)
	if nameText == "<incomplete>" {
		return
	}

	fnScope := NewScope(b.current, ScopeFunction)

	fnSym := &Symbol{
		Name: nameText,
		Kind: SymFunction,
		Span: b.tree.RangeOf(name),
		ID:   b.newSymID(),
		Def:  name,
	}

	fnScope.Owner = fnSym

	_ = b.current.Define(fnSym)
	b.Defs[name] = fnSym

	fnSym.Inner = fnScope
	prevSelf := b.selfName
	if b.current.Kind == ScopeClass && args != ast.NoNode {
		firstParam := b.tree.Nodes[args].FirstChild
		paramName, _, _ := b.tree.ParamParts(firstParam)
		b.selfName, _ = b.tree.NameText(paramName)
	} else {
		b.selfName = ""
	}
	prev := b.current
	b.current = fnScope
	prevInFunc := b.inFunction
	b.inFunction = true

	if args != ast.NoNode {
		for arg := b.tree.Nodes[args].FirstChild; arg != ast.NoNode; arg = b.tree.Nodes[arg].NextSibling {
			paramName, annotation, def := b.tree.ParamParts(arg)
			sym := b.define(b.current, paramName, SymParameter, b.tree.RangeOf(paramName))
			if sym != nil && def != ast.NoNode {
				sym.DefaultValue = b.extractValue(def)
				sym.IsVarArg = b.tree.ParamIsVarArg(arg)
				sym.IsKwArg = b.tree.ParamIsKwArg(arg)
			} else if sym != nil {
				sym.IsVarArg = b.tree.ParamIsVarArg(arg)
				sym.IsKwArg = b.tree.ParamIsKwArg(arg)
			}
			if annotation != ast.NoNode {
				b.visitExpr(annotation)
			}
			if def != ast.NoNode {
				b.visitExpr(def)
			}
		}
	}

	for stmt := b.tree.Nodes[body].FirstChild; stmt != ast.NoNode; stmt = b.tree.Nodes[stmt].NextSibling {
		b.visitStmt(stmt)
	}

	b.current = prev
	b.selfName = prevSelf
	b.inFunction = prevInFunc
}
