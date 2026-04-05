package main

import (
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"

	"rahu/analyser"
	"rahu/parser"
	"rahu/parser/ast"
)

var (
	bold    string
	red     string
	reset   string
	blue    string
	cyan    string
	green   string
	yellow  string
	magenta string
)

func initColors(noColor bool) {
	if noColor {
		bold, red, reset, blue, cyan, green, yellow, magenta = "", "", "", "", "", "", "", ""
	} else {
		bold = "\033[1m"
		red = "\033[31m"
		reset = "\033[0m"
		blue = "\033[34m"
		cyan = "\033[36m"
		green = "\033[32m"
		yellow = "\033[33m"
		magenta = "\033[35m"
	}
}

func header(s string)   { fmt.Println(bold + cyan + s + reset) }
func errLine(s string)  { fmt.Println(red + s + reset) }
func okLine(s string)   { fmt.Println(green + s + reset) }
func warnLine(s string) { fmt.Println(yellow + s + reset) }

func main() {
	noColor := flag.Bool("no-color", false, "Disable colored output")
	flag.Parse()

	initColors(*noColor)

	src, err := os.ReadFile("temp.py")
	if err != nil {
		panic(err)
	}

	header("=== SOURCE ===")
	fmt.Println(string(src))
	fmt.Println()

	p := parser.New(string(src))
	tree := p.Parse()

	if errs := p.Errors(); len(errs) > 0 {
		header("=== PARSER ERRORS ===")
		for _, e := range errs {
			errLine(fmt.Sprintf("%v: %s", e.Span, e.Msg))
		}
		return
	}

	global, _ := analyser.BuildScopes(tree)

	header("=== SCOPES ===")
	dumpScope(global, 0)
	fmt.Println()

	analyser.PromoteClassMembers(global)

	semErrors, resolved, resolvedAttrs, pendingAttrs := analyser.ResolveWithAttrs(tree, global)

	header("=== RESOLVER STATS ===")
	fmt.Printf("names=%d attrs=%d pending=%d semErrs=%d\n\n",
		len(resolved),
		len(resolvedAttrs),
		len(pendingAttrs),
		len(semErrors),
	)

	if len(semErrors) > 0 {
		header("=== SEMANTIC ERRORS ===")
		for _, e := range semErrors {
			errLine(fmt.Sprintf("%v: %s", e.Span, e.Msg))
		}
		fmt.Println()
	}

	header("=== RESOLVED NAMES ===")
	nameByID := collectNameNodes(tree)

	type nameEntry struct {
		id   ast.NodeID
		name nameInfo
		sym  *analyser.Symbol
	}

	var names []nameEntry
	for id, s := range resolved {
		n, ok := nameByID[id]
		if !ok {
			continue
		}
		names = append(names, nameEntry{id: id, name: n, sym: s})
	}

	sort.Slice(names, func(i, j int) bool {
		return names[i].name.Span.Start < names[j].name.Span.Start
	})

	for _, e := range names {
		okLine(fmt.Sprintf(
			"%s @ [%d,%d] -> %s (%s)",
			e.name.Text,
			e.name.Span.Start,
			e.name.Span.End,
			e.sym.Name,
			e.sym.Kind,
		))
	}

	fmt.Println()
	header("=== ATTRIBUTE BINDINGS ===")
	dumpAttrBindings(tree, resolvedAttrs)

	fmt.Println()
	header("=== INSTANCE ATTRIBUTES PER CLASS ===")
	dumpClassAttrs(global)

	fmt.Println()
	header("=== PROMOTED CLASS MEMBERS ===")
	dumpClassMembers(global)
}

func dumpScope(s *analyser.Scope, indent int) {
	prefix := strings.Repeat("  ", indent)

	fmt.Printf("%sScope(%s)\n", prefix, s.Kind)

	var names []string
	for n := range s.Symbols {
		names = append(names, n)
	}
	sort.Strings(names)

	for _, n := range names {
		sym := s.Symbols[n]

		fmt.Printf("%s  %s%s%s : %s\n",
			prefix,
			yellow,
			sym.Name,
			reset,
			sym.Kind,
		)

		if sym.Inner != nil {
			dumpScope(sym.Inner, indent+2)
		}
	}
}

func dumpClassAttrs(s *analyser.Scope) {
	for _, sym := range s.Symbols {
		if sym.Kind == analyser.SymClass {
			fmt.Printf("%sClass %s%s\n", magenta, sym.Name, reset)

			if sym.Attrs != nil {
				var names []string
				for n := range sym.Attrs.Symbols {
					names = append(names, n)
				}
				sort.Strings(names)

				for _, n := range names {
					fmt.Printf("  %sattr%s %s\n", yellow, reset, n)
				}
			}

			if sym.Inner != nil {
				dumpClassAttrs(sym.Inner)
			}
		}
	}
}

func dumpAttrBindings(tree *ast.AST, attrs map[ast.NodeID]*analyser.Symbol) {
	walk(tree, tree.Root, func(id ast.NodeID) {
		if tree.Node(id).Kind != ast.NodeAttribute {
			return
		}

		baseID := tree.ChildAt(id, 0)
		attrID := tree.ChildAt(id, 1)
		if attrID == ast.NoNode {
			return
		}

		base := "<expr>"
		if baseID != ast.NoNode && tree.Node(baseID).Kind == ast.NodeName {
			base, _ = tree.NameText(baseID)
		}

		attrName, _ := tree.NameText(attrID)
		attrSpan := tree.RangeOf(attrID)
		sym := attrs[id]

		if sym == nil {
			errLine(fmt.Sprintf(
				"UNBOUND %s.%s at [%d,%d]",
				base,
				attrName,
				attrSpan.Start,
				attrSpan.End,
			))
			return
		}

		okLine(fmt.Sprintf(
			"BOUND   %s.%s -> %s (%s) at [%d,%d]",
			base,
			attrName,
			sym.Name,
			sym.Kind,
			attrSpan.Start,
			attrSpan.End,
		))
	})
}

type nameInfo struct {
	Text string
	Span ast.Range
}

func collectNameNodes(tree *ast.AST) map[ast.NodeID]nameInfo {
	result := make(map[ast.NodeID]nameInfo)

	walk(tree, tree.Root, func(id ast.NodeID) {
		if tree.Node(id).Kind != ast.NodeName {
			return
		}

		text, _ := tree.NameText(id)
		result[id] = nameInfo{
			Text: text,
			Span: tree.RangeOf(id),
		}
	})

	return result
}

func walk(tree *ast.AST, id ast.NodeID, visit func(ast.NodeID)) {
	if tree == nil || id == ast.NoNode {
		return
	}

	visit(id)
	for _, child := range tree.Children(id) {
		walk(tree, child, visit)
	}
}

func dumpClassMembers(s *analyser.Scope) {
	for _, sym := range s.Symbols {
		if sym.Kind == analyser.SymClass {
			fmt.Printf("%sClass %s%s\n", magenta, sym.Name, reset)

			if sym.Members != nil {
				var names []string
				for n := range sym.Members.Symbols {
					names = append(names, n)
				}
				sort.Strings(names)

				for _, n := range names {
					m := sym.Members.Symbols[n]

					fmt.Printf("  %smember%s %s : %s\n",
						yellow,
						reset,
						m.Name,
						m.Kind,
					)
				}
			} else {
				warnLine("  (no members)")
			}

			if sym.Inner != nil {
				dumpClassMembers(sym.Inner)
			}
		}
	}
}
