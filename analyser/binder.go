package analyser

import "rahu/parser/ast"

func (r *Resolver) BindMembers() {
	for _, p := range r.PendingAttrs {
		a := p.Node
		if a == nil {
			continue
		}

		base, ok := a.Value.(*ast.Name)
		if !ok {
			continue
		}

		baseSym := r.Resolved[base.ID]
		if baseSym == nil {
			continue
		}

		if p.Class != nil && p.SelfName != "" &&
			base.Text == p.SelfName &&
			baseSym.Kind == SymParameter {

			sym, ok := p.Class.Members.Lookup(a.Attr.Text)
			if !ok {
				r.error(a.Attr.Pos, "undefined attribute: "+a.Attr.Text)
				continue
			}

			r.ResolvedAttr[a.ID] = sym
			continue
		}

		// --- case 2: instance.attr outside class ---
		if baseSym.InstanceOf != nil {
			class := baseSym.InstanceOf

			sym, ok := class.Members.Lookup(a.Attr.Text)
			if !ok {
				r.error(a.Attr.Pos, "undefined attribute: "+a.Attr.Text)
				continue
			}

			r.ResolvedAttr[a.ID] = sym
			continue
		}
	}
}

func ResolveWithAttrs(
	m *ast.Module,
	global *Scope,
) (
	[]SemanticError,
	map[ast.NodeID]*Symbol,
	map[ast.NodeID]*Symbol,
	[]PendingAttr,
) {
	r := newResolver(global)
	r.visitModule(m)
	PromoteClassMembers(global)
	r.BindMembers()
	return r.errors, r.Resolved, r.ResolvedAttr, r.PendingAttrs
}
