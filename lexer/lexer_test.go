package lexer

import (
	"testing"
)

func TestSingleCharOperators(t *testing.T) {
	for input, tokType := range SingleCharOps {
		l := New(input)
		msg := l.NextToken()
		if msg.Type != tokType {
			t.Errorf("%v not parsed correctly, wanted %v but received %v", input, tokType, msg.Type)
		}
	}
}

func TestMultiCharOperators(t *testing.T) {
	for input, tokType := range MultiCharOps {
		l := New(input)
		msg := l.NextToken()
		if msg.Type != tokType {
			t.Errorf("%v not parsed correctly, wanted %v but received %v", input, tokType, msg.Type)
		}
	}
}

func TestSingleIdentifier(t *testing.T) {
	input := "my_var1"
	want := NAME
	l := New(input)
	tokType := l.NextToken().Type
	if tokType != want {
		t.Errorf("identifier %v not parsed correctly, want NAME but received %v instead", input, tokType)
	}
}

func TestSingleNumber(t *testing.T) {
	input := "9"
	want := NUMBER
	l := New(input)
	msg := l.NextToken()

	if msg.Type != want {
		t.Errorf("number not parsed correctly, want NUMBER but received %v instead", msg.Type)
	}

	if msg.Literal != "9" {
		t.Errorf("want literal %v, got %v", input, msg.Literal)
	}
}

func TestBasicIndent(t *testing.T) {
	input := "def foo():\n    pass"
	want := []TokenType{NAME, NAME, LPAR, RPAR, COLON, NEWLINE, INDENT, NAME, EOF}
	l := New(input)
	count := 0
	for {
		tok := l.NextToken()
		if count >= len(want) {
			t.Fatalf("Got more tokens than expedted. Token %v", tok.Type)
		}
		t.Logf("Debug: token emmited: %v\n", tok.Type)
		if tok.Type != want[count] {
			t.Errorf("Token type mismatch, expected %v but got %v\n", want[count], tok.Type)
		}
		count++
		if tok.Type == EOF {
			break
		}
	}
}

// TODO: add tests for single indent, empty input, multiple dedents and string literals
