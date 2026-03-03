package analyser

func PromoteClassMembers(scope *Scope) {
	for _, sym := range scope.Symbols {
		if sym.Kind == SymClass {
			promoteOneClass(sym)
			if sym.Inner != nil {
				PromoteClassMembers(sym.Inner)
			}
		}
		if sym.Inner != nil {
			PromoteClassMembers(sym.Inner)
		}
	}
}

func promoteOneClass(cls *Symbol) {
	if cls.Members == nil {
		cls.Members = NewScope(nil, ScopeMember)
	}

	// 1. Methods
	if cls.Inner != nil {
		for _, s := range cls.Inner.Symbols {
			cls.Members.Define(s)
		}
	}

	// 2. Instance attributes
	if cls.Attrs != nil {
		for _, a := range cls.Attrs.Symbols {
			cls.Members.Define(a)
		}
	}

	// 3. (Next step) Base classes
	// for _, base := range cls.Bases { ... }
}
