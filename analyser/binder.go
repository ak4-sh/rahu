package analyser

import "rahu/parser/ast"

func (r *Resolver) BindMembers() {
	for _, p := range r.PendingAttrs {
		a := p.Node
		if a == nil || p.Class == nil || p.SelfName == "" {
			continue
		}

		base, ok := a.Value.(*ast.Name)
		if !ok || base.ID != p.SelfName {
			continue
		}

		baseSym := r.Resolved[base]
		if baseSym == nil || baseSym.Kind != SymParameter {
			continue
		}

		if p.Class.Members == nil {
			r.error(a.Attr.Pos, "internal error: missing class members")
			continue
		}

		sym, ok := p.Class.Members.Lookup(a.Attr.ID)
		if !ok {
			r.error(a.Attr.Pos, "undefined attribute: "+a.Attr.ID)
			continue
		}

		r.ResolvedAttr[a] = sym
	}
}

func ResolveWithAttrs(
	m *ast.Module,
	global *Scope,
) (
	[]SemanticError,
	map[*ast.Name]*Symbol,
	map[*ast.Attribute]*Symbol,
	[]PendingAttr,
) {
	r := newResolver(global)
	r.visitModule(m)
	PromoteClassMembers(global)
	r.BindMembers()
	return r.errors, r.Resolved, r.ResolvedAttr, r.PendingAttrs
}
