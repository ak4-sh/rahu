package analyser

import "maps"

func PromoteClassMembers(scope *Scope) {
	for _, sym := range scope.Symbols {
		if sym.Kind == SymClass {
			wasPromoted := sym.Members != nil
			promoteOneClass(sym)
			// Only recurse into Inner for freshly promoted (local) classes.
			// Already-promoted classes have shared Inner scopes — recursing
			// would call promoteOneClass on the same nested class pointers
			// from multiple goroutines, causing concurrent map writes.
			if !wasPromoted && sym.Inner != nil {
				PromoteClassMembers(sym.Inner)
			}
		} else if sym.Inner != nil {
			PromoteClassMembers(sym.Inner)
		}
	}
}

func promoteOneClass(cls *Symbol) {
	// Always create a fresh Members scope before writing. Imported class
	// symbols share their Members pointer across goroutines (shallow clone);
	// writing into a shared map causes concurrent map write panics. Reading
	// from the existing Members to seed the fresh scope is safe because no
	// other goroutine is writing to it at that point.
	//
	// isRepromotion is true when Members was already set (e.g. this class was
	// imported from a surface that already ran PromoteClassMembers, or this is
	// a second promotion pass after import binding). On re-promotion we skip
	// the s.Scope reassignments: the symbols in Inner/Attrs are shared across
	// goroutines for imported classes, so writing to their Scope field
	// concurrently would be a data race.
	isRepromotion := cls.Members != nil
	fresh := NewScope(nil, ScopeMember)
	if cls.Members != nil {
		maps.Copy(fresh.Symbols, cls.Members.Symbols)
	}
	cls.Members = fresh

	// 1. Methods
	if cls.Inner != nil {
		for _, s := range cls.Inner.Symbols {
			if !isRepromotion {
				// Point the method's scope at Members so that classOwner()
				// walking Parent chains won't accidentally find the ScopeClass
				// inner scope and double-prefix the label (e.g. "Foo.method").
				s.Scope = cls.Members
			}
			cls.Members.Symbols[s.Name] = s
		}
	}

	// 2. Instance attributes
	if cls.Attrs != nil {
		for _, a := range cls.Attrs.Symbols {
			if !isRepromotion {
				a.Scope = cls.Members
			}
			cls.Members.Symbols[a.Name] = a
		}
	}

	// 3. Base classes — do NOT reassign sym.Scope here. The inherited symbol
	// belongs to the base; overwriting its Scope with the child's Members
	// scope is a race when the symbol is shared, and classOwner() on a
	// Members scope (nil Parent) returns nil anyway, so the assignment was
	// a no-op for all callers.
	for _, base := range cls.Bases {
		if base == nil || base.Members == nil {
			continue
		}

		for name, sym := range base.Members.Symbols {
			// Do not override child definitions
			if _, exists := cls.Members.Symbols[name]; !exists {
				cls.Members.Symbols[name] = sym
			}
		}
	}
}
