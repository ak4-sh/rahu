package ast

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
	if id == NoNode || a.Nodes[id].Kind != NodeString {
		return "", false
	}

	idx := a.Nodes[id].Data
	if int(idx) >= len(a.Strings) {
		return "", false
	}

	return a.Strings[idx], true
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

// FunctionParts returns the typed children of a function node.
func (a *AST) FunctionParts(id NodeID) (name, args, body NodeID) {
	if id == NoNode || a.Nodes[id].Kind != NodeFunctionDef {
		return NoNode, NoNode, NoNode
	}

	for child := a.Nodes[id].FirstChild; child != NoNode; child = a.Nodes[child].NextSibling {
		if name == NoNode {
			name = child
			continue
		}

		switch a.Nodes[child].Kind {
		case NodeArgs:
			args = child
		case NodeBlock:
			body = child
		}
	}

	return name, args, body
}

// ClassParts returns the typed children of a class node.
func (a *AST) ClassParts(id NodeID) (name, bases, body NodeID) {
	if id == NoNode || a.Nodes[id].Kind != NodeClassDef {
		return NoNode, NoNode, NoNode
	}

	for child := a.Nodes[id].FirstChild; child != NoNode; child = a.Nodes[child].NextSibling {
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
