package parser

import (
	l "rahu/lexer"
	a "rahu/parser/ast"
)

func (p *Parser) parseDottedName() a.NodeID {
	if p.current.Type != l.NAME {
		p.errorCurrent("expected name")
		return a.NoNode
	}

	node := p.tree.NewNameNode(p.current.Start, p.current.End, p.current.Literal)
	p.advance()

	for p.current.Type == l.DOT {
		dotStart := p.current.Start
		p.advance()
		if p.current.Type != l.NAME {
			p.error(a.Range{Start: dotStart, End: p.current.End}, "expected name after '.'")
			return node
		}

		right := p.tree.NewNameNode(p.current.Start, p.current.End, p.current.Literal)
		attr := p.tree.NewNode(a.NodeAttribute, p.tree.Nodes[node].Start, p.current.End)
		p.tree.AddChild(attr, node)
		p.tree.AddChild(attr, right)
		node = attr
		p.advance()
	}

	return node
}

func (p *Parser) parseImportAlias() a.NodeID {
	start := p.current.Start
	target := p.parseDottedName()
	if target == a.NoNode {
		return a.NoNode
	}

	aliasNode := p.tree.NewNode(a.NodeAlias, start, p.tree.Nodes[target].End)
	p.tree.AddChild(aliasNode, target)

	if p.current.Type == l.AS {
		p.advance()
		if p.current.Type != l.NAME {
			p.errorCurrent("expected alias name after 'as'")
			return aliasNode
		}

		alias := p.tree.NewNameNode(p.current.Start, p.current.End, p.current.Literal)
		p.tree.AddChild(aliasNode, alias)
		p.tree.Nodes[aliasNode].End = p.current.End
		p.advance()
	}

	return aliasNode
}

func (p *Parser) finishSimpleStatement(start uint32, end uint32, msg string) uint32 {
	if p.current.Type == l.NEWLINE {
		end = p.current.Start
		p.advance()
		return end
	}
	if p.current.Type != l.EOF {
		p.error(a.Range{Start: p.current.Start, End: p.current.End}, msg)
		p.syncTo(l.NEWLINE, l.EOF)
		end = p.current.Start
		if p.current.Type == l.NEWLINE {
			p.advance()
		}
	}
	return end
}

func (p *Parser) parseImport() a.NodeID {
	start := p.current.Start
	p.advance()

	ret := p.tree.NewNode(a.NodeImport, start, start)
	alias := p.parseImportAlias()
	if alias == a.NoNode {
		p.errorCurrent("expected import path")
		p.syncTo(l.NEWLINE, l.EOF)
		p.tree.Nodes[ret].End = p.current.Start
		if p.current.Type == l.NEWLINE {
			p.advance()
		}
		return ret
	}
	p.tree.AddChild(ret, alias)
	p.tree.Nodes[ret].End = p.tree.Nodes[alias].End

	for p.current.Type == l.COMMA {
		p.advance()
		alias = p.parseImportAlias()
		if alias == a.NoNode {
			p.errorCurrent("expected import path after ','")
			break
		}
		p.tree.AddChild(ret, alias)
		p.tree.Nodes[ret].End = p.tree.Nodes[alias].End
	}

	p.tree.Nodes[ret].End = p.finishSimpleStatement(start, p.tree.Nodes[ret].End, "expected newline after import statement")
	return ret
}

func (p *Parser) parseRelativeModulePath() (uint32, a.NodeID) {
	depth := uint32(0)
	for p.current.Type == l.DOT {
		depth++
		p.advance()
	}
	if p.current.Type != l.NAME {
		return depth, a.NoNode
	}
	return depth, p.parseDottedName()
}

func (p *Parser) parseFromImport() a.NodeID {
	start := p.current.Start
	p.advance()

	ret := p.tree.NewNode(a.NodeFromImport, start, start)
	depth, module := p.parseRelativeModulePath()
	p.tree.Nodes[ret].Data = depth
	if depth == 0 && module == a.NoNode {
		p.errorCurrent("expected module path after 'from'")
		p.syncTo(l.NEWLINE, l.EOF)
		p.tree.Nodes[ret].End = p.current.Start
		if p.current.Type == l.NEWLINE {
			p.advance()
		}
		return ret
	}
	if depth > 0 || module != a.NoNode {
		p.tree.AddChild(ret, module)
	}

	if p.current.Type != l.IMPORT {
		p.errorCurrent("expected 'import' after module path")
		p.syncTo(l.IMPORT, l.NEWLINE, l.EOF)
		if p.current.Type != l.IMPORT {
			p.tree.Nodes[ret].End = p.current.Start
			if p.current.Type == l.NEWLINE {
				p.advance()
			}
			return ret
		}
	}
	p.advance()

	alias := p.parseImportAlias()
	if alias == a.NoNode {
		p.errorCurrent("expected imported name")
		p.syncTo(l.NEWLINE, l.EOF)
		p.tree.Nodes[ret].End = p.current.Start
		if p.current.Type == l.NEWLINE {
			p.advance()
		}
		return ret
	}
	p.tree.AddChild(ret, alias)
	p.tree.Nodes[ret].End = p.tree.Nodes[alias].End

	for p.current.Type == l.COMMA {
		p.advance()
		alias = p.parseImportAlias()
		if alias == a.NoNode {
			p.errorCurrent("expected imported name after ','")
			break
		}
		p.tree.AddChild(ret, alias)
		p.tree.Nodes[ret].End = p.tree.Nodes[alias].End
	}

	p.tree.Nodes[ret].End = p.finishSimpleStatement(start, p.tree.Nodes[ret].End, "expected newline after from-import statement")
	return ret
}
