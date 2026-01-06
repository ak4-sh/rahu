package lexer

import (
	"errors"
	"strings"
)

type Token struct {
	Type    TokenType
	Literal string
	Line    int
	Col     int
	EndCol  int
	File    string
}

type Lexer struct {
	input         string
	position      int
	readPosition  int
	ch            byte
	line          int
	col           int
	indentStack   []int
	atLineStart   bool
	pendingTokens []Token
	indentChar    byte
}

func New(input string) *Lexer {
	l := &Lexer{
		input:         input,
		line:          1,
		col:           0,
		atLineStart:   true,
		indentStack:   []int{0},
		pendingTokens: []Token{},
		indentChar:    0,
	}
	l.readChar()
	return l
}

// Read next character. If we exceed input len, return ASCII nil.
// If valid char exists, advance position and readPosition
func (l *Lexer) readChar() {
	if l.readPosition >= len(l.input) {
		l.ch = 0
	} else {
		l.ch = l.input[l.readPosition]
	}

	l.position = l.readPosition
	l.readPosition++
	if l.ch == '\n' {
		l.col = 0
		l.line++
	} else {
		l.col++
	}
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

func (l *Lexer) skipWhitespace() {
	for l.ch == ' ' || l.ch == '\t' {
		l.readChar()
	}
}

// Checks if there exists a multi char token from current position.
// If no multichar token exists, return 0.
// If multi char token exists, return 2 or 3
func (l *Lexer) isMultiCharToken() (TokenType, int, bool) {
	// Try 3-char token first (longest match)
	if l.position+3 <= len(l.input) {
		token := l.input[l.position : l.position+3]
		if tokType, ok := multiCharOps[token]; ok {
			return tokType, 3, true
		}
	}

	// Try 2-char token
	if l.position+2 <= len(l.input) {
		token := l.input[l.position : l.position+2]
		if tokType, ok := multiCharOps[token]; ok {
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

func (l *Lexer) isChar() bool {
	if (l.ch >= 'a' && l.ch <= 'z') || (l.ch >= 'A' && l.ch <= 'Z') {
		return true
	}
	return false
}

func (l *Lexer) readNumber() string {
	var sb strings.Builder
	sb.WriteByte(l.ch)
	for {
		l.readChar()
		if !l.isDigit() {
			break
		}
		sb.WriteByte(l.ch)
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

	for l.ch != quoteType && l.ch != 0 {
		// Stop at closing quote or EOF
		sb.WriteByte(l.ch)
		l.readChar()
	}

	if l.ch != quoteType {
		return sb.String(), ILLEGAL
	}

	l.readChar()
	return sb.String(), STRING
}

func (l *Lexer) countLeadingSpaces() (int, error) {
	count := 0
	seenSpace := false
	seenTab := false

	for {
		curr := l.peekAhead(count)

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

func (l *Lexer) readMultilineString(quoteType byte) (string, TokenType) {
	var sb strings.Builder
	// skipping all quotes
	for range 3 {
		l.readChar()
	}

	for {
		if l.ch == 0 {
			return sb.String(), ILLEGAL
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

func (l *Lexer) NextToken() Token {
	var tok Token

	if len(l.pendingTokens) > 0 {
		tok := l.pendingTokens[0]
		l.pendingTokens = l.pendingTokens[1:]
		return tok
	}

	if l.atLineStart {
		spaces, err := l.countLeadingSpaces()
		if err != nil {
			return Token{
				Literal: "",
				Type:    ILLEGAL,
			}
		}

		// 2. Check if it's a blank line or comment
		//    (Don't process indentation for these)
		if l.ch == '\n' || l.ch == '#' {
			// Skip blank lines and comments - don't change indentation
			// Just let normal tokenization continue
			// Don't set atLineStart = false yet
		} else {
			// 3. This is a real line with content - process indentation

			// Get current indentation level (top of stack)
			currentIndent := l.indentStack[len(l.indentStack)-1]

			// Compare spaces to current indent
			if spaces > currentIndent {
				l.indentStack = append(l.indentStack, spaces)
				tok.Type = INDENT
				tok.Line = l.line
				tok.Col = l.col
				tok.Literal = ""
				l.atLineStart = false

				for range spaces {
					l.readChar()
				}
				return tok
			} else if spaces < currentIndent {
				dedentCount := 0
				for len(l.indentStack) > 0 && l.indentStack[len(l.indentStack)-1] > spaces {
					l.indentStack = l.indentStack[:len(l.indentStack)-1]
					dedentCount++
				}

				if l.indentStack[len(l.indentStack)-1] != spaces {
					return Token{Type: ILLEGAL, Literal: "Indentation error"}
				}

				tok.Type = DEDENT
				tok.Literal = ""
				tok.Line = l.line
				tok.Col = l.col

				for i := 1; i < dedentCount; i++ {
					l.pendingTokens = append(l.pendingTokens, Token{
						Type:    DEDENT,
						Literal: "",
						Line:    l.line,
						Col:     l.col,
					})
				}

				for range spaces {
					l.readChar()
				}

				l.atLineStart = false
				return tok
			} else {
				for range spaces {
					l.readChar()
				}
				l.atLineStart = false
				// spaces == currentIndent
				// Same level - no indent/dedent token needed
				// Just consume the whitespace and continue
			}
		}
	}

	l.skipWhitespace()

	tok.Line = l.line
	tok.Col = l.col

	tokType, tokLen, ok := l.isMultiCharToken()
	if ok {
		tok.EndCol = l.col + tokLen - 1
		tok.Type = tokType
		tok.Literal = l.input[l.position : l.position+tokLen]
		for range tokLen {
			l.readChar()
		}
		return tok
	} else {
		// not a multichar token, need to check other cases
		// 1. Is it a number?
		// 2. Is it a word?
		// 3. Is it a
		if l.isDigit() {
			tok.Literal = l.readNumber()
			tok.Type = NUMBER
			tok.EndCol = l.position
			return tok
		}

		if l.isChar() || l.ch == '_' {
			literal := l.readIdentifier()
			tok.Literal = literal
			tok.Type = NAME
			tok.EndCol = l.position
			return tok
		}

		if l.ch == '\n' {
			tok.Literal = "\n"
			tok.Type = NEWLINE
			l.readChar()
			l.atLineStart = true
			return tok
		}

		if l.ch == 0 {
			tok.Literal = ""
			tok.Type = EOF
			return tok
		}

		// single char operator check
		if val, ok := singleCharOps[string(l.ch)]; ok {
			tok.Type = val
			tok.Literal = string(l.ch)
			tok.EndCol = l.position
			l.readChar()
			return tok
		}

		if l.ch == '"' || l.ch == '\'' {
			if l.ch == '\'' && l.peek() == '\'' && l.peekAhead(1) == '\'' {
				tokStr, tokType := l.readMultilineString('\'')
				tok.Type = tokType
				tok.Literal = tokStr
				tok.Line = l.line
				return tok
			} else if l.ch == '"' && l.peek() == '"' && l.peekAhead(1) == '"' {
				tokStr, tokType := l.readMultilineString('"')
				tok.Type = tokType
				tok.Literal = tokStr
				tok.Line = l.line
				return tok
			} else {
				// normal string
				openingQuote := l.ch
				tokStr, tokType := l.readString(openingQuote)
				tok.Type = tokType
				tok.Literal = tokStr
				return tok
			}
		}

	}
	tok.Type = ILLEGAL
	tok.Literal = string(l.ch)
	l.readChar()
	return tok
}
