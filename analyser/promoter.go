package analyser

func PromoteClassMembers(scope *Scope) {
	for _, sym := range scope.Symbols {
		if sym.Kind == SymClass {
			promoteOneClass(sym)
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
			s.Scope = cls.Members
			cls.Members.Symbols[s.Name] = s
		}
	}

	// 2. Instance attributes
	if cls.Attrs != nil {
		for _, a := range cls.Attrs.Symbols {
			a.Scope = cls.Members
			cls.Members.Symbols[a.Name] = a
		}
	}

	// 3. (Next step) Base classes
	// for _, base := range cls.Bases { ... }
	for _, base := range cls.Bases {
		if base == nil || base.Members == nil {
			continue
		}

		for name, sym := range base.Members.Symbols {
			// Do not override child definitions
			if _, exists := cls.Members.Symbols[name]; !exists {
				sym.Scope = cls.Members
				cls.Members.Symbols[name] = sym
			}
		}
	}
}
