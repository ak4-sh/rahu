# Phase 1 — Make It Real (Single-File Depth)

Goal: solid Python semantics inside one file.

1. Inheritance (finish it properly)
	•	Store base class symbols
	•	Promote base members into child (respect override)
	•	Resolve method lookup via class → bases
	•	Make hover show inherited origin

Result: Child.base_method resolves correctly.

2. Instance Tracking (Major Upgrade)

Handle:

c = Child()
c.x
c.sum()

Add:
	•	Track simple constructor calls
	•	Attach inferred class to variable symbol
	•	Attribute lookup via class members

This unlocks:
	•	Real-world resolution
	•	Cross-method usage

3. Improve Attribute Binding Model

Right now attributes are discovered only via self.

Add:
	•	Track writes anywhere inside class
	•	Track instance creation patterns
	•	Improve error precision

4. Hover Upgrade

Include:
	•	Full signature
	•	Owning class
	•	Inherited-from info
	•	File + line
	•	Possibly inferred type

Make hover feel professional.

# Phase 2 — Cross-File Support (Workspace-Aware)

This is the biggest step forward.

5. Import Resolution

Support:
	•	import module
	•	from module import name
	•	Relative imports

You need:
	•	Workspace file map
	•	Module symbol table cache
	•	Module-level scope graph

Now Rahu stops being single-file.

6. Workspace Index

Maintain:
	•	File checksum
	•	Dirty tracking
	•	Re-analysis propagation
	•	Global symbol registry

Only re-analyze affected files.

7. Cross-File Definition

Jump to definition across modules.

Now it feels real.

# Phase 3 — Developer Features

Now add LSP features.

8. References

Find:
	•	All name usages
	•	Cross-file
	•	Attribute usages

Requires good symbol graph.

9. Rename

Safe rename:
	•	Local scope
	•	Class scope
	•	Cross-file

This requires stable symbol identity.

10. Completion

Context-aware:
	•	Inside class → members
	•	After . → class members
	•	Global scope → globals + imports

Completion is easy once symbol graph is strong.

# Phase 4 — Smarter Semantics

Optional but powerful.

11. Basic Type Propagation

Shallow inference:
	•	Constructor return → instance type
	•	Simple arithmetic types
	•	Function return tracking

Enough to improve hover + errors.

12. Error Improvements

Add:
	•	Unused variable warning
	•	Duplicate symbol errors
	•	Shadowing detection
	•	Dead code detection

# Phase 5 — Performance

Only after correctness.
	•	Incremental parsing
	•	Incremental resolution
	•	File-level dependency graph
	•	Parallel analysis

What NOT to Do Yet
	•	Multi-language support
	•	Plugin system
	•	Full type inference
	•	Metaclass/decorator complexity
	•	LSP exotic features

Order of Execution (Concrete)
	1.	Inheritance
	2.	Instance tracking
	3.	Attribute resolution outside self
	4.	Imports
	5.	Workspace indexing
	6.	References
	7.	Rename
	8.	Completion
	9.	Type propagation

When Is It “Usable”?

After:
	•	Imports work
	•	Instance resolution works
	•	Cross-file definition works
	•	References work

At that point Rahu becomes a serious tool.

