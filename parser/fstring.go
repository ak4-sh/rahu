package parser

import (
	"strings"

	l "rahu/lexer"
	a "rahu/parser/ast"
)

func (p *Parser) parseFString() a.NodeID {
	start := p.current.Start
	end := p.current.End
	lexeme := p.current.Literal
	p.advance()

	content, contentOffset, ok := parseFStringLexeme(lexeme)
	ret := p.tree.NewNode(a.NodeFString, start, end)
	if !ok {
		p.error(a.Range{Start: start, End: end}, "invalid f-string literal")
		return ret
	}

	textStart := 0
	for i := 0; i < len(content); {
		switch content[i] {
		case '{':
			if i+1 < len(content) && content[i+1] == '{' {
				i += 2
				continue
			}
			p.addFStringText(ret, content[textStart:i], start+contentOffset+uint32(textStart), uint32(i-textStart))
			field, ok := scanFStringField(content, i+1)
			if !ok {
				p.error(a.Range{Start: start + contentOffset + uint32(i), End: end}, "unterminated f-string expression")
				return ret
			}
			exprSrc := content[field.exprStart:field.exprEnd]
			exprNode := p.parseFStringExpr(exprSrc, start+contentOffset+uint32(field.exprStart), start+contentOffset+uint32(field.close+1))
			if field.hasConversion {
				conv := content[field.conversion]
				if conv != 'r' && conv != 'R' && conv != 's' && conv != 'S' && conv != 'a' && conv != 'A' {
					p.error(a.Range{Start: start + contentOffset + uint32(field.conversion-1), End: start + contentOffset + uint32(field.conversion+1)}, "invalid f-string conversion")
				}
			}
			p.tree.AddChild(ret, exprNode)
			i = field.close + 1
			textStart = i

		case '}':
			if i+1 < len(content) && content[i+1] == '}' {
				i += 2
				continue
			}
			p.error(a.Range{Start: start + contentOffset + uint32(i), End: start + contentOffset + uint32(i+1)}, "single '}' is not allowed in f-string")
			i++
			textStart = i

		default:
			i++
		}
	}

	p.addFStringText(ret, content[textStart:], start+contentOffset+uint32(textStart), uint32(len(content)-textStart))
	return ret
}

type fStringField struct {
	exprStart     int
	exprEnd       int
	conversion    int
	formatSpec    int
	close         int
	hasConversion bool
	hasFormatSpec bool
}

func (p *Parser) stringNodeAsFStringText(id a.NodeID) a.NodeID {
	if id == a.NoNode || p.tree.Node(id).Kind != a.NodeString {
		return a.NoNode
	}
	text, _ := p.tree.StringText(id)
	idx := uint32(len(p.tree.Strings))
	p.tree.Strings = append(p.tree.Strings, text)
	textNode := p.tree.NewNode(a.NodeFStringText, p.tree.Node(id).Start, p.tree.Node(id).End)
	p.tree.Nodes[textNode].Data = idx
	return textNode
}

func (p *Parser) mergeStringLiterals(left, right a.NodeID) a.NodeID {
	if left == a.NoNode || right == a.NoNode {
		return left
	}
	leftKind := p.tree.Node(left).Kind
	rightKind := p.tree.Node(right).Kind
	if leftKind == a.NodeString && rightKind == a.NodeString {
		leftText, _ := p.tree.StringText(left)
		rightText, _ := p.tree.StringText(right)
		idx := uint32(len(p.tree.Strings))
		p.tree.Strings = append(p.tree.Strings, leftText+rightText)
		p.tree.Nodes[left].Data = idx
		p.tree.Nodes[left].End = p.tree.Node(right).End
		return left
	}

	var merged a.NodeID
	if leftKind == a.NodeFString {
		merged = left
	} else {
		merged = p.tree.NewNode(a.NodeFString, p.tree.Node(left).Start, p.tree.Node(right).End)
		if textNode := p.stringNodeAsFStringText(left); textNode != a.NoNode {
			p.tree.AddChild(merged, textNode)
		}
	}
	if rightKind == a.NodeString {
		if textNode := p.stringNodeAsFStringText(right); textNode != a.NoNode {
			p.tree.AddChild(merged, textNode)
		}
	} else {
		for child := p.tree.Node(right).FirstChild; child != a.NoNode; {
			next := p.tree.Node(child).NextSibling
			p.tree.Nodes[child].NextSibling = a.NoNode
			p.tree.AddChild(merged, child)
			child = next
		}
	}
	p.tree.Nodes[merged].End = p.tree.Node(right).End
	return merged
}

func (p *Parser) addFStringText(parent a.NodeID, text string, start, rawLen uint32) {
	if text == "" {
		return
	}
	text = strings.ReplaceAll(text, "{{", "{")
	text = strings.ReplaceAll(text, "}}", "}")
	idx := uint32(len(p.tree.Strings))
	p.tree.Strings = append(p.tree.Strings, text)
	id := p.tree.NewNode(a.NodeFStringText, start, start+rawLen)
	p.tree.Nodes[id].Data = idx
	p.tree.AddChild(parent, id)
}

func (p *Parser) parseFStringExpr(src string, absStart, absEnd uint32) a.NodeID {
	wrapper := p.tree.NewNode(a.NodeFStringExpr, absStart-1, absEnd)
	if strings.TrimSpace(src) == "" {
		p.error(a.Range{Start: absStart - 1, End: absEnd}, "empty f-string expression")
		errNode := p.tree.NewNode(a.NodeErrExp, absStart, absEnd)
		p.tree.AddChild(wrapper, errNode)
		return wrapper
	}

	sub := New(src)
	sub.tree = a.New(len(src))
	expr := sub.parseExpression(LOWEST)
	if expr == a.NoNode {
		errNode := p.tree.NewNode(a.NodeErrExp, absStart, absEnd)
		p.tree.AddChild(wrapper, errNode)
		return wrapper
	}
	if sub.current.Type != l.EOF {
		sub.error(a.Range{Start: sub.current.Start, End: sub.current.End}, "unexpected token in f-string expression")
	}
	for _, err := range sub.errors {
		p.error(a.Range{Start: absStart + err.Span.Start, End: absStart + err.Span.End}, err.Msg)
	}
	cloned := cloneSubtree(p.tree, sub.tree, expr, absStart)
	p.tree.AddChild(wrapper, cloned)
	return wrapper
}

func cloneSubtree(dst, src *a.AST, id a.NodeID, delta uint32) a.NodeID {
	if src == nil || id == a.NoNode {
		return a.NoNode
	}
	node := src.Node(id)
	var cloned a.NodeID
	switch node.Kind {
	case a.NodeName:
		name, _ := src.NameText(id)
		cloned = dst.NewNameNode(node.Start+delta, node.End+delta, name)
	case a.NodeString, a.NodeFStringText:
		text, _ := src.StringText(id)
		idx := uint32(len(dst.Strings))
		dst.Strings = append(dst.Strings, text)
		cloned = dst.NewNode(node.Kind, node.Start+delta, node.End+delta)
		dst.Nodes[cloned].Data = idx
	case a.NodeNumber:
		lit, _ := src.NumberText(id)
		idx := uint32(len(dst.Numbers))
		dst.Numbers = append(dst.Numbers, lit)
		cloned = dst.NewNode(node.Kind, node.Start+delta, node.End+delta)
		dst.Nodes[cloned].Data = idx
	default:
		cloned = dst.NewNode(node.Kind, node.Start+delta, node.End+delta)
		dst.Nodes[cloned].Data = node.Data
	}
	for child := node.FirstChild; child != a.NoNode; child = src.Node(child).NextSibling {
		dst.AddChild(cloned, cloneSubtree(dst, src, child, delta))
	}
	return cloned
}

func parseFStringLexeme(lexeme string) (string, uint32, bool) {
	prefixLen := 0
	for prefixLen < len(lexeme) {
		ch := lexeme[prefixLen]
		if ch == 'f' || ch == 'F' || ch == 'r' || ch == 'R' {
			prefixLen++
			continue
		}
		break
	}
	if prefixLen == 0 || prefixLen > 2 || len(lexeme) < prefixLen+2 {
		return "", 0, false
	}
	quote := lexeme[prefixLen]
	if quote != '\'' && quote != '"' {
		return "", 0, false
	}
	if len(lexeme) >= prefixLen+6 && lexeme[prefixLen+1] == quote && lexeme[prefixLen+2] == quote {
		return lexeme[prefixLen+3 : len(lexeme)-3], uint32(prefixLen + 3), true
	}
	if len(lexeme) < 3 || lexeme[len(lexeme)-1] != quote {
		return "", 0, false
	}
	return lexeme[prefixLen+1 : len(lexeme)-1], uint32(prefixLen + 1), true
}

func scanFStringField(content string, start int) (fStringField, bool) {
	field := fStringField{exprStart: start, exprEnd: -1, conversion: -1, formatSpec: -1, close: -1}
	parenDepth := 0
	bracketDepth := 0
	braceDepth := 0
	inString := byte(0)
	triple := false
	escaped := false

	for i := start; i < len(content); i++ {
		ch := content[i]
		if inString != 0 {
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if triple {
				if i+2 < len(content) && ch == inString && content[i+1] == inString && content[i+2] == inString {
					inString = 0
					triple = false
					i += 2
				}
				continue
			}
			if ch == inString {
				inString = 0
			}
			continue
		}

		switch ch {
		case '\'', '"':
			inString = ch
			triple = i+2 < len(content) && content[i+1] == ch && content[i+2] == ch
			if triple {
				i += 2
			}
		case '!':
			if parenDepth == 0 && bracketDepth == 0 && braceDepth == 0 && !field.hasConversion && !field.hasFormatSpec {
				field.exprEnd = i
				if i+1 >= len(content) {
					return field, false
				}
				field.hasConversion = true
				field.conversion = i + 1
				i++
			}
		case ':':
			if parenDepth == 0 && bracketDepth == 0 && braceDepth == 0 && !field.hasFormatSpec {
				if field.exprEnd < 0 {
					field.exprEnd = i
				}
				field.hasFormatSpec = true
				field.formatSpec = i + 1
			}
		case '(':
			parenDepth++
		case ')':
			if parenDepth > 0 {
				parenDepth--
			}
		case '[':
			bracketDepth++
		case ']':
			if bracketDepth > 0 {
				bracketDepth--
			}
		case '{':
			braceDepth++
		case '}':
			if parenDepth == 0 && bracketDepth == 0 && braceDepth == 0 {
				if field.exprEnd < 0 {
					field.exprEnd = i
				}
				field.close = i
				return field, true
			}
			if braceDepth > 0 {
				braceDepth--
			}
		}
	}

	return field, false
}
