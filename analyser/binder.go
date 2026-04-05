package analyser

import "rahu/parser/ast"

func (r *Resolver) BindMembers() {
	for _, p := range r.PendingAttrs {
		attrNode := p.Node
		base := r.tree.Nodes[attrNode].FirstChild
		attrNameNode := ast.NoNode
		if base != ast.NoNode {
			attrNameNode = r.tree.Nodes[base].NextSibling
		}
		if base == ast.NoNode || attrNameNode == ast.NoNode {
			continue
		}

		baseName, _ := r.tree.NameText(base)
		baseSym := r.Resolved[base]
		if baseSym == nil {
			continue
		}

		attrName, _ := r.tree.NameText(attrNameNode)

		if p.Class != nil && p.SelfName != "" &&
			baseName == p.SelfName &&
			baseSym.Kind == SymParameter {

			sym, ok := p.Class.Members.Lookup(attrName)
			if !ok {
				r.error(r.tree.RangeOf(attrNameNode), "undefined attribute: "+attrName)
				continue
			}

			r.ResolvedAttr[attrNode] = sym
			continue
		}

		if baseType := r.exprType(base); baseType != nil {
			sym, ok := LookupMemberOnType(baseType, attrName)
			if !ok {
				r.error(r.tree.RangeOf(attrNameNode), "undefined attribute: "+attrName)
				continue
			}

			r.ResolvedAttr[attrNode] = sym
			continue
		}

		if baseSym.InstanceOf != nil {
			sym, ok := baseSym.InstanceOf.Members.Lookup(attrName)
			if !ok {
				r.error(r.tree.RangeOf(attrNameNode), "undefined attribute: "+attrName)
				continue
			}

			r.ResolvedAttr[attrNode] = sym
			continue
		}
	}
}

func ResolveWithAttrs(
	tree *ast.AST,
	global *Scope,
) (
	[]SemanticError,
	map[ast.NodeID]*Symbol,
	map[ast.NodeID]*Symbol,
	[]PendingAttr,
) {
	r := newResolver(tree, global)
	r.visitModule()
	PromoteClassMembers(global)
	r.BindMembers()
	return r.errors, r.Resolved, r.ResolvedAttr, r.PendingAttrs
}
