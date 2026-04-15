package ast

const (
	ParamFlagHasAnnotation = 1 << iota
	ParamFlagHasDefault
	ParamFlagIsVarArg
	ParamFlagIsKwArg
)

// ChildCount counts all immediate children of a given nodeID
func (a *AST) ChildCount(id NodeID) int {
	if id == NoNode {
		return 0
	}

	count := 0

	for child := a.Nodes[id].FirstChild; child != NoNode; child = a.Nodes[child].NextSibling {
		count++
	}
	return count
}

// Children returns a list of all NodeIDs that are children of given node
func (a *AST) Children(id NodeID) []NodeID {
	if id == NoNode {
		return nil
	}

	out := make([]NodeID, 0, 4)

	for child := a.Nodes[id].FirstChild; child != NoNode; child = a.Nodes[child].NextSibling {
		out = append(out, child)
	}

	return out
}

// ChildAt returns the child node at the specific index of a given parent node
func (a *AST) ChildAt(id NodeID, index int) NodeID {
	if id == NoNode || index < 0 {
		return NoNode
	}

	i := 0

	for child := a.Nodes[id].FirstChild; child != NoNode; child = a.Nodes[child].NextSibling {
		if i == index {
			return child
		}

		i++
	}
	return NoNode
}

// HasChildren checks whether given node has any children
func (a *AST) HasChildren(id NodeID) bool {
	return id == NoNode || a.Node(id).FirstChild == NoNode
}

// LastChildID returns the NodeID for the last child node of a given parent node
func (a *AST) LastChildID(id NodeID) NodeID {
	if id == NoNode {
		return NoNode
	}

	if a.Nodes[id].LastChild == NoNode {
		return NoNode
	}

	return a.Nodes[id].LastChild
}

// RangeOf returns range of given Node in a Range struct
func (a *AST) RangeOf(id NodeID) Range {
	if id == NoNode {
		return Range{}
	}

	start, end := a.Nodes[id].Range()
	return Range{Start: start, End: end}
}

// IsKind checks whether a given NodeID is a specific given NodeKind
func (a *AST) IsKind(id NodeID, kind NodeKind) bool {
	if id == NoNode {
		return false
	}

	return a.Nodes[id].Kind == kind
}

// NameText fetches the string from a given NodeName
func (a *AST) NameText(id NodeID) (string, bool) {
	if id == NoNode || a.Nodes[id].Kind != NodeName {
		return "", false
	}

	idx := a.Nodes[id].Data
	if int(idx) >= len(a.Names) {
		return "", false
	}

	return a.Names[idx], true
}

// StringText fetches the string from a given NodeString
func (a *AST) StringText(id NodeID) (string, bool) {
	if id == NoNode || (a.Nodes[id].Kind != NodeString && a.Nodes[id].Kind != NodeFStringText) {
		return "", false
	}

	idx := a.Nodes[id].Data
	if int(idx) >= len(a.Strings) {
		return "", false
	}

	return a.Strings[idx], true
}

// BytesText fetches the bytes string from a given NodeBytes
func (a *AST) BytesText(id NodeID) (string, bool) {
	if id == NoNode || a.Nodes[id].Kind != NodeBytes {
		return "", false
	}

	idx := a.Nodes[id].Data
	if int(idx) >= len(a.Bytes) {
		return "", false
	}

	return a.Bytes[idx], true
}

// NumberText fetches the Number for a given NodeNumber
func (a *AST) NumberText(id NodeID) (string, bool) {
	if id == NoNode || a.Nodes[id].Kind != NodeNumber {
		return "", false
	}

	idx := a.Nodes[id].Data
	if int(idx) >= len(a.Numbers) {
		return "", false
	}
	return a.Numbers[idx], true
}

// ParamParts returns the typed children of a parameter node.
func (a *AST) ParamParts(id NodeID) (name, annotation, defaultValue NodeID) {
	if id == NoNode || a.Nodes[id].Kind != NodeParam {
		return NoNode, NoNode, NoNode
	}

	name = a.Nodes[id].FirstChild
	if name == NoNode {
		return NoNode, NoNode, NoNode
	}

	second := a.Nodes[name].NextSibling
	if second == NoNode {
		return name, NoNode, NoNode
	}

	flags := a.Nodes[id].Data
	hasAnnotation := flags&ParamFlagHasAnnotation != 0
	hasDefault := flags&ParamFlagHasDefault != 0
	if hasAnnotation && !hasDefault {
		return name, second, NoNode
	}
	if !hasAnnotation && hasDefault {
		return name, NoNode, second
	}

	third := a.Nodes[second].NextSibling
	if third != NoNode {
		return name, second, third
	}

	if a.Nodes[second].Kind == NodeErrExp {
		return name, second, NoNode
	}

	return name, NoNode, second
}

func (a *AST) ParamIsVarArg(id NodeID) bool {
	return id != NoNode && a.Nodes[id].Kind == NodeParam && a.Nodes[id].Data&ParamFlagIsVarArg != 0
}

func (a *AST) ParamIsKwArg(id NodeID) bool {
	return id != NoNode && a.Nodes[id].Kind == NodeParam && a.Nodes[id].Data&ParamFlagIsKwArg != 0
}

// AnnAssignParts returns the typed children of an annotated assignment node.
func (a *AST) AnnAssignParts(id NodeID) (target, annotation, value NodeID) {
	if id == NoNode || a.Nodes[id].Kind != NodeAnnAssign {
		return NoNode, NoNode, NoNode
	}

	target = a.Nodes[id].FirstChild
	if target == NoNode {
		return NoNode, NoNode, NoNode
	}
	annotation = a.Nodes[target].NextSibling
	if annotation == NoNode {
		return target, NoNode, NoNode
	}
	value = a.Nodes[annotation].NextSibling
	return target, annotation, value
}

func (a *AST) AssertParts(id NodeID) (test, msg NodeID) {
	if id == NoNode || a.Nodes[id].Kind != NodeAssert {
		return NoNode, NoNode
	}
	test = a.Nodes[id].FirstChild
	if test == NoNode {
		return NoNode, NoNode
	}
	msg = a.Nodes[test].NextSibling
	return test, msg
}

func (a *AST) DelTargets(id NodeID) []NodeID {
	if id == NoNode || a.Nodes[id].Kind != NodeDel {
		return nil
	}
	return a.Children(id)
}

func (a *AST) NameList(id NodeID) []NodeID {
	if id == NoNode {
		return nil
	}
	if a.Nodes[id].Kind != NodeGlobal && a.Nodes[id].Kind != NodeNonlocal {
		return nil
	}
	return a.Children(id)
}

// FunctionParts returns the typed children of a function node.
func (a *AST) FunctionParts(id NodeID) (name, args, body NodeID) {
	name, args, _, body = a.FunctionPartsWithReturn(id)
	return name, args, body
}

// Decorators returns decorator children for function or class definitions.
func (a *AST) Decorators(id NodeID) []NodeID {
	if id == NoNode {
		return nil
	}
	if a.Nodes[id].Kind != NodeFunctionDef && a.Nodes[id].Kind != NodeClassDef {
		return nil
	}

	var decorators []NodeID
	for child := a.Nodes[id].FirstChild; child != NoNode; child = a.Nodes[child].NextSibling {
		if a.Nodes[child].Kind != NodeDecorator {
			break
		}
		decorators = append(decorators, child)
	}
	return decorators
}

// DecoratorExpr returns the expression child of a decorator node.
func (a *AST) DecoratorExpr(id NodeID) NodeID {
	if id == NoNode || a.Nodes[id].Kind != NodeDecorator {
		return NoNode
	}
	return a.Nodes[id].FirstChild
}

// FunctionPartsWithReturn returns the typed children of a function node,
// including the optional return annotation expression.
func (a *AST) FunctionPartsWithReturn(id NodeID) (name, args, returnAnnotation, body NodeID) {
	if id == NoNode || a.Nodes[id].Kind != NodeFunctionDef {
		return NoNode, NoNode, NoNode, NoNode
	}

	for child := a.Nodes[id].FirstChild; child != NoNode; child = a.Nodes[child].NextSibling {
		if a.Nodes[child].Kind == NodeDecorator {
			continue
		}
		if name == NoNode {
			name = child
			continue
		}

		switch a.Nodes[child].Kind {
		case NodeArgs:
			args = child
		case NodeBlock:
			body = child
		default:
			returnAnnotation = child
		}
	}

	return name, args, returnAnnotation, body
}

// ClassParts returns the typed children of a class node.
func (a *AST) ClassParts(id NodeID) (name, bases, body NodeID) {
	if id == NoNode || a.Nodes[id].Kind != NodeClassDef {
		return NoNode, NoNode, NoNode
	}

	for child := a.Nodes[id].FirstChild; child != NoNode; child = a.Nodes[child].NextSibling {
		if a.Nodes[child].Kind == NodeDecorator {
			continue
		}
		if name == NoNode {
			name = child
			continue
		}

		switch a.Nodes[child].Kind {
		case NodeBaseList:
			bases = child
		case NodeBlock:
			body = child
		}
	}

	return name, bases, body
}

// AliasParts returns the target and optional alias children of an alias node.
func (a *AST) AliasParts(id NodeID) (target, alias NodeID) {
	if id == NoNode || a.Nodes[id].Kind != NodeAlias {
		return NoNode, NoNode
	}

	target = a.Nodes[id].FirstChild
	if target != NoNode {
		alias = a.Nodes[target].NextSibling
	}

	return target, alias
}

// FromImportParts returns the module path and imported aliases.
func (a *AST) FromImportParts(id NodeID) (module NodeID, aliases []NodeID) {
	if id == NoNode || a.Nodes[id].Kind != NodeFromImport {
		return NoNode, nil
	}

	first := a.Nodes[id].FirstChild
	if first != NoNode && a.Nodes[first].Kind != NodeAlias {
		module = first
		first = a.Nodes[first].NextSibling
	}
	for child := first; child != NoNode; child = a.Nodes[child].NextSibling {
		aliases = append(aliases, child)
	}

	return module, aliases
}

// TryParts returns the try body, except handlers, optional else block, and optional finally block.
func (a *AST) TryParts(id NodeID) (body NodeID, excepts []NodeID, elseBlock NodeID, finallyBlock NodeID) {
	if id == NoNode || a.Nodes[id].Kind != NodeTry {
		return NoNode, nil, NoNode, NoNode
	}
	children := a.Children(id)
	if len(children) == 0 {
		return NoNode, nil, NoNode, NoNode
	}
	body = children[0]
	i := 1
	for i < len(children) && a.Nodes[children[i]].Kind == NodeExcept {
		excepts = append(excepts, children[i])
		i++
	}
	if a.Nodes[id].Data&1 != 0 && i < len(children) {
		elseBlock = children[i]
		i++
	}
	if a.Nodes[id].Data&2 != 0 && i < len(children) {
		finallyBlock = children[i]
	}
	return body, excepts, elseBlock, finallyBlock
}

// RaiseParts returns the optional exception expression and optional cause expression.
func (a *AST) RaiseParts(id NodeID) (exc, cause NodeID) {
	if id == NoNode || a.Nodes[id].Kind != NodeRaise {
		return NoNode, NoNode
	}

	exc = a.Nodes[id].FirstChild
	if exc != NoNode {
		cause = a.Nodes[exc].NextSibling
	}

	return exc, cause
}

// ExceptParts returns the optional exception type, optional bound name, and body block.
func (a *AST) ExceptParts(id NodeID) (excType, asName, body NodeID) {
	if id == NoNode || a.Nodes[id].Kind != NodeExcept {
		return NoNode, NoNode, NoNode
	}
	children := a.Children(id)
	if len(children) == 0 {
		return NoNode, NoNode, NoNode
	}
	body = children[len(children)-1]
	if len(children) >= 2 {
		excType = children[0]
	}
	if len(children) >= 3 {
		asName = children[1]
	}
	return excType, asName, body
}

// ListCompParts returns the result expression and comprehension clauses.
func (a *AST) ListCompParts(id NodeID) (expr NodeID, clauses []NodeID) {
	if id == NoNode || a.Nodes[id].Kind != NodeListComp {
		return NoNode, nil
	}
	expr = a.Nodes[id].FirstChild
	for child := a.Nodes[expr].NextSibling; child != NoNode; child = a.Nodes[child].NextSibling {
		clauses = append(clauses, child)
	}
	return expr, clauses
}

// DictCompParts returns the key expression, value expression, and comprehension clauses.
func (a *AST) DictCompParts(id NodeID) (key, value NodeID, clauses []NodeID) {
	if id == NoNode || a.Nodes[id].Kind != NodeDictComp {
		return NoNode, NoNode, nil
	}

	key = a.Nodes[id].FirstChild
	if key == NoNode {
		return NoNode, NoNode, nil
	}
	value = a.Nodes[key].NextSibling
	if value == NoNode {
		return key, NoNode, nil
	}
	for child := a.Nodes[value].NextSibling; child != NoNode; child = a.Nodes[child].NextSibling {
		clauses = append(clauses, child)
	}
	return key, value, clauses
}

// ComprehensionParts returns the target, iterable, and optional filters.
func (a *AST) ComprehensionParts(id NodeID) (target, iter NodeID, filters []NodeID) {
	if id == NoNode || a.Nodes[id].Kind != NodeComprehension {
		return NoNode, NoNode, nil
	}
	target = a.Nodes[id].FirstChild
	if target == NoNode {
		return NoNode, NoNode, nil
	}
	iter = a.Nodes[target].NextSibling
	if iter == NoNode {
		return target, NoNode, nil
	}
	for child := a.Nodes[iter].NextSibling; child != NoNode; child = a.Nodes[child].NextSibling {
		filters = append(filters, child)
	}
	return target, iter, filters
}

// WithParts returns the with-items and body block for a with statement.
func (a *AST) WithParts(id NodeID) (items []NodeID, body NodeID) {
	if id == NoNode || a.Nodes[id].Kind != NodeWith {
		return nil, NoNode
	}

	for child := a.Nodes[id].FirstChild; child != NoNode; child = a.Nodes[child].NextSibling {
		if a.Nodes[child].Kind == NodeBlock {
			body = child
			continue
		}
		items = append(items, child)
	}

	return items, body
}

// WithItemParts returns the context expression and optional bound target.
func (a *AST) WithItemParts(id NodeID) (contextExpr, asTarget NodeID) {
	if id == NoNode || a.Nodes[id].Kind != NodeWithItem {
		return NoNode, NoNode
	}

	contextExpr = a.Nodes[id].FirstChild
	if contextExpr != NoNode {
		asTarget = a.Nodes[contextExpr].NextSibling
	}

	return contextExpr, asTarget
}

// DocString fetches the docstring stored in a node's Data field.
func (a *AST) DocString(id NodeID) (string, bool) {
	if id == NoNode {
		return "", false
	}

	idx := a.Nodes[id].Data
	if idx == 0 || int(idx) >= len(a.Strings) {
		return "", false
	}

	return a.Strings[idx], true
}
