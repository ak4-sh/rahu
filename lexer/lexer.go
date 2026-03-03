// Package lexer implements a lexical analyzer for Python source code.
//
// The lexer converts raw source text into a stream of tokens suitable for
// consumption by the parser. It is indentation-aware and emits INDENT and
// DEDENT tokens to model Python’s block structure, following rules similar
// to CPython’s tokenizer.
//
// All token positions are represented as byte offsets into the original
// source text. Each token records a half-open range [Start, End), where
// Start is the offset of the first byte belonging to the token and End is
// the offset immediately after the last byte. Line and column information
// is deliberately not tracked in the lexer and is derived later at API
// boundaries using a line index.
//
// Features:
//   - Tokenization of Python keywords, identifiers, literals, and operators
//   - Support for single-, double-, and triple-quoted strings
//   - Handling of multi-character operators with longest-match semantics
//   - Indentation tracking with explicit INDENT / DEDENT tokens
//   - Detection of inconsistent or mixed indentation (tabs vs spaces)
//
// The lexer is stateful and processes input incrementally via NextToken().
// It performs no syntactic or semantic validation beyond what is required
// to produce a correct token stream.
//
// The output of this package is intended to be consumed by the parser
// package to construct an abstract syntax tree (AST), which also operates
// exclusively on byte-offset source ranges.
package lexer

import (
	"errors"
	"fmt"
	"strings"
)

type Token struct {
	Start   int
	Literal string
	Type    TokenType
	File    string
	End     int
}

func (t *Token) String() string {
	return fmt.Sprintf("Tok {Type: %v, Literal: %s}", t.Type, t.Literal)
}

type Lexer struct {
	input         string
	position      int
	readPosition  int
	ch            byte
	indentStack   []int
	atLineStart   bool
	pendingTokens []Token
	indentChar    byte
	parenDepth    int
}

func New(input string) *Lexer {
	l := &Lexer{
		input:         input,
		position:      0,
		readPosition:  0,
		indentStack:   []int{0},
		pendingTokens: []Token{},
		atLineStart:   true,
	}
	l.readChar()
	return l
}

func (l *Lexer) readChar() {
	if l.readPosition >= len(l.input) {
		l.ch = 0
	} else {
		l.ch = l.input[l.readPosition]
	}

	l.position = l.readPosition
	l.readPosition++
}

func (l *Lexer) peek() byte {
	if l.readPosition >= len(l.input) {
		return 0
	}
	return l.input[l.readPosition]
}

func (l *Lexer) peekAhead(delta int) byte {
	nextPos := l.readPosition + delta
	if nextPos >= len(l.input) {
		return 0
	}
	return l.input[nextPos]
}

func (l *Lexer) skipComment() {
	for l.ch != '\n' && l.ch != 0 {
		l.readChar()
	}
}

func (l *Lexer) skipWhitespaceAndComments() {
	for {
		switch l.ch {
		case ' ':
			l.readChar()
		case '#':
			l.skipComment()
		default:
			return
		}
	}
}

func (l *Lexer) isMultiCharToken() (TokenType, int, bool) {
	// Try 3-char token first (longest match)
	if l.position+3 <= len(l.input) {
		token := l.input[l.position : l.position+3]
		if tokType, ok := MultiCharOps[token]; ok {
			return tokType, 3, true
		}
	}

	// Try 2-char token
	if l.position+2 <= len(l.input) {
		token := l.input[l.position : l.position+2]
		if tokType, ok := MultiCharOps[token]; ok {
			return tokType, 2, true
		}
	}

	// No multi-char token found
	return ILLEGAL, 0, false
}

func (l *Lexer) isDigit() bool {
	if l.ch >= '0' && l.ch <= '9' {
		return true
	}

	return false
}

func isDigit(val byte) bool {
	if val >= '0' && val <= '9' {
		return true
	}
	return false
}

func (l *Lexer) isChar() bool {
	if (l.ch >= 'a' && l.ch <= 'z') || (l.ch >= 'A' && l.ch <= 'Z') {
		return true
	}
	return false
}

func (l *Lexer) readNumber() string {
	var sb strings.Builder
	sb.WriteByte(l.ch)
	hasDot := false
	for {
		l.readChar()
		if l.ch == '.' && !hasDot && isDigit(l.peek()) {
			hasDot = true
			sb.WriteByte(l.ch)
		} else if !l.isDigit() {
			break
		} else {
			sb.WriteByte(l.ch)
		}
	}
	return sb.String()
}

func (l *Lexer) isIdentifierChar() bool {
	return l.isChar() || l.isDigit() || l.ch == '_'
}

func (l *Lexer) readIdentifier() string {
	var sb strings.Builder
	sb.WriteByte(l.ch)
	for {
		l.readChar()
		if !l.isIdentifierChar() {
			break
		}
		sb.WriteByte(l.ch)
	}
	return sb.String()
}

func (l *Lexer) readString(quoteType byte) (string, TokenType) {
	var sb strings.Builder
	l.readChar() // Skip opening quote

	for l.ch != 0 {
		// same quote type as string start
		if l.ch == quoteType {
			break
		}

		if l.ch == '\\' {
			sb.WriteByte(l.ch)
			l.readChar()

			// incomplete escape sequence, invalid syntax
			if l.ch == 0 {
				return sb.String(), UNTERMINATED_STRING
			}

			sb.WriteByte(l.ch)
			l.readChar()
			continue
		}
		// Stop at closing quote or EOF
		sb.WriteByte(l.ch)
		l.readChar()
	}

	if l.ch != quoteType {
		return sb.String(), UNTERMINATED_STRING
	}

	l.readChar()
	return sb.String(), STRING
}

func (l *Lexer) readMultilineString(quoteType byte) (string, TokenType) {
	var sb strings.Builder
	// skipping all quotes
	for range 3 {
		l.readChar()
	}

	for {
		if l.ch == 0 {
			return sb.String(), UNTERMINATED_STRING
		}

		if l.ch == quoteType && l.peek() == quoteType && l.peekAhead(1) == quoteType {
			// found endstring
			for range 3 {
				l.readChar()
			}
			return sb.String(), STRING
		}

		sb.WriteByte(l.ch)
		l.readChar()
	}
}

func (l *Lexer) countLeadingSpaces() (int, error) {
	count := 0
	seenSpace := false
	seenTab := false

	for {
		peekPos := l.position + count
		if peekPos >= len(l.input) {
			break
		}
		curr := l.input[peekPos]

		if curr == ' ' {
			seenSpace = true
			count++
		} else if curr == '\t' {
			seenTab = true
			count++
		} else {
			break
		}
	}

	// Check for mixing
	if seenSpace && seenTab {
		return 0, errors.New("mixed tabs and spaces in indentation")
	}

	// Set or verify indentChar for the file
	if count > 0 {
		if seenSpace && l.indentChar == '\t' {
			return 0, errors.New("inconsistent use of tabs and spaces")
		}
		if seenTab && l.indentChar == ' ' {
			return 0, errors.New("inconsistent use of tabs and spaces")
		}

		// Set indent char if not yet determined
		if l.indentChar == 0 {
			if seenSpace {
				l.indentChar = ' '
			} else {
				l.indentChar = '\t'
			}
		}
	}

	return count, nil
}

func (l *Lexer) NextToken() Token {
	if len(l.pendingTokens) > 0 {
		tok := l.pendingTokens[0]
		l.pendingTokens = l.pendingTokens[1:]
		return tok
	}

	if l.atLineStart && l.parenDepth == 0 {
		spaces, err := l.countLeadingSpaces()
		if err != nil {
			pos := l.position
			return Token{
				Type:  ILLEGAL,
				Start: pos,
				End:   pos,
			}
		}

		if l.ch != '\n' && l.ch != '#' {
			current := l.indentStack[len(l.indentStack)-1]
			pos := l.position

			if spaces > current {
				l.indentStack = append(l.indentStack, spaces)

				tok := Token{
					Type:  INDENT,
					Start: pos,
					End:   pos,
				}

				for range spaces {
					l.readChar()
				}

				l.atLineStart = false
				return tok
			}

			if spaces < current {
				dedentCount := 0
				for len(l.indentStack) > 1 &&
					l.indentStack[len(l.indentStack)-1] > spaces {
					l.indentStack = l.indentStack[:len(l.indentStack)-1]
					dedentCount++
				}

				if l.indentStack[len(l.indentStack)-1] != spaces {
					return Token{
						Type:  ILLEGAL,
						Start: pos,
						End:   pos,
					}
				}

				tok := Token{
					Type:  DEDENT,
					Start: pos,
					End:   pos,
				}

				for i := 1; i < dedentCount; i++ {
					l.pendingTokens = append(l.pendingTokens, Token{
						Type:  DEDENT,
						Start: pos,
						End:   pos,
					})
				}

				for range spaces {
					l.readChar()
				}

				l.atLineStart = false
				return tok
			}

			for range spaces {
				l.readChar()
			}
			l.atLineStart = false
		}
	}

	if l.atLineStart && l.parenDepth > 0 {
		l.atLineStart = false
	}

	l.skipWhitespaceAndComments()

	if l.ch == 0 {
		if len(l.indentStack) > 1 {
			l.indentStack = l.indentStack[:len(l.indentStack)-1]
			return Token{
				Type:  DEDENT,
				Start: l.position,
				End:   l.position,
			}
		}
		pos := l.position
		return Token{
			Type:  EOF,
			Start: pos,
			End:   pos,
		}
	}

	if l.ch == '\n' {
		start := l.position
		l.readChar()
		if l.parenDepth > 0 {
			l.atLineStart = false
			return l.NextToken()
		}
		l.atLineStart = true
		return Token{
			Type:  NEWLINE,
			Start: start,
			End:   l.position,
		}
	}

	if tokType, tokLen, ok := l.isMultiCharToken(); ok {
		start := l.position
		literal := l.input[start : start+tokLen]
		for range tokLen {
			l.readChar()
		}
		return Token{
			Type:    tokType,
			Literal: literal,
			Start:   start,
			End:     l.position,
		}
	}

	if l.isDigit() {
		start := l.position
		lit := l.readNumber()
		return Token{
			Type:    NUMBER,
			Literal: lit,
			Start:   start,
			End:     l.position,
		}
	}

	if l.isChar() || l.ch == '_' {
		start := l.position
		lit := l.readIdentifier()
		typ := NAME
		if kw, ok := Keywords[lit]; ok {
			typ = kw
		}
		return Token{
			Type:    typ,
			Literal: lit,
			Start:   start,
			End:     l.position,
		}
	}

	if typ, ok := SingleCharOps[string(l.ch)]; ok {
		start := l.position
		lit := string(l.ch)
		switch l.ch {
		case '(', '[', '{':
			l.parenDepth++
		case ')', ']', '}':
			if l.parenDepth > 0 {
				l.parenDepth--
			}
		}

		l.readChar()
		return Token{
			Type:    typ,
			Literal: lit,
			Start:   start,
			End:     l.position,
		}
	}

	if l.ch == '"' || l.ch == '\'' {
		start := l.position

		var lit string
		var typ TokenType

		if l.ch == '\'' && l.peek() == '\'' && l.peekAhead(1) == '\'' {
			lit, typ = l.readMultilineString('\'')
		} else if l.ch == '"' && l.peek() == '"' && l.peekAhead(1) == '"' {
			lit, typ = l.readMultilineString('"')
		} else {
			lit, typ = l.readString(l.ch)
		}

		return Token{
			Type:    typ,
			Literal: lit,
			Start:   start,
			End:     l.position,
		}
	}

	start := l.position
	ch := l.ch
	l.readChar()
	return Token{
		Type:    ILLEGAL,
		Literal: string(ch),
		Start:   start,
		End:     l.position,
	}
}
