package lexer

import (
	"strings"
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

func TestHexBinaryOctalNumbers(t *testing.T) {
	tests := []struct {
		input    string
		wantLit  string
		wantType TokenType
	}{
		// Hex literals
		{"0x023301", "<144129>", NUMBER},
		{"0X1A", "<26>", NUMBER},
		{"0xff", "<255>", NUMBER},
		{"0x0", "<0>", NUMBER},
		// Binary literals
		{"0b1010", "<10>", NUMBER},
		{"0B1111", "<15>", NUMBER},
		{"0b0", "<0>", NUMBER},
		// Octal literals
		{"0o777", "<511>", NUMBER},
		{"0O755", "<493>", NUMBER},
		{"0o0", "<0>", NUMBER},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			l := New(tt.input)
			tok := l.NextToken()

			if tok.Type != tt.wantType {
				t.Errorf("want %v, got %v", tt.wantType, tok.Type)
			}
			if tok.Literal != tt.wantLit {
				t.Errorf("want literal %q, got %q", tt.wantLit, tok.Literal)
			}
		})
	}
}

func TestFStringToken(t *testing.T) {
	input := `f"hello {name}"`
	l := New(input)
	tok := l.NextToken()
	if tok.Type != FSTRING {
		t.Fatalf("expected FSTRING, got %v", tok.Type)
	}
	if tok.Literal != input {
		t.Fatalf("unexpected f-string literal: got %q", tok.Literal)
	}
}

func TestTripleQuotedFStringToken(t *testing.T) {
	input := "f'''hello {name}'''"
	l := New(input)
	tok := l.NextToken()
	if tok.Type != FSTRING {
		t.Fatalf("expected FSTRING, got %v", tok.Type)
	}
}

func TestRawStringToken(t *testing.T) {
	input := `r"hello\nworld"`
	l := New(input)
	tok := l.NextToken()
	if tok.Type != STRING {
		t.Fatalf("expected STRING, got %v", tok.Type)
	}
	if tok.Literal != `hello\nworld` {
		t.Fatalf("unexpected raw string literal: got %q", tok.Literal)
	}
}

func TestRawFStringToken(t *testing.T) {
	input := `rf"hello {name}\n"`
	l := New(input)
	tok := l.NextToken()
	if tok.Type != FSTRING {
		t.Fatalf("expected FSTRING, got %v", tok.Type)
	}
	if tok.Literal != input {
		t.Fatalf("unexpected raw f-string literal: got %q", tok.Literal)
	}
}

func TestTripleQuotedRawStringToken(t *testing.T) {
	input := "r'''hello\\nworld'''"
	l := New(input)
	tok := l.NextToken()
	if tok.Type != STRING {
		t.Fatalf("expected STRING, got %v", tok.Type)
	}
	if tok.Literal != `hello\nworld` {
		t.Fatalf("unexpected triple raw string literal: got %q", tok.Literal)
	}
}

func TestBasicIndent(t *testing.T) {
	input := "def foo():\n    pass"
	want := []TokenType{
		DEF, NAME, LPAR, RPAR, COLON, NEWLINE, INDENT, PASS, DEDENT, EOF,
	}
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

func TestEOFFlushesDedent(t *testing.T) {
	input := "if x:\n    y = 1"
	l := New(input)

	expected := []TokenType{
		IF,
		NAME,
		COLON,
		NEWLINE,
		INDENT,
		NAME,
		EQUAL,
		NUMBER,
		DEDENT,
		EOF,
	}

	for i, want := range expected {
		tok := l.NextToken()

		if tok.Type != want {
			t.Fatalf(
				"token %d mismatch: expected %v but got %v",
				i,
				want,
				tok.Type,
			)
		}
	}
}

func benchmarkLexAll(b *testing.B, input string) {
	b.Helper()
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		l := New(input)
		for {
			if tok := l.NextToken(); tok.Type == EOF {
				break
			}
		}
	}
}

func benchmarkPythonLines(n int) string {
	var sb strings.Builder
	for i := range n {
		sb.WriteString("x")
		sb.WriteString(" = ")
		sb.WriteString("123\n")
		if i%25 == 0 {
			sb.WriteString("if x:\n    y = x\n")
		}
	}
	return sb.String()
}

func BenchmarkLexerSmall(b *testing.B) {
	benchmarkLexAll(b, benchmarkPythonLines(50))
}

func BenchmarkLexerMedium(b *testing.B) {
	benchmarkLexAll(b, benchmarkPythonLines(250))
}

func BenchmarkLexerLarge(b *testing.B) {
	benchmarkLexAll(b, benchmarkPythonLines(1500))
}

func BenchmarkLexerOperatorHeavy(b *testing.B) {
	benchmarkLexAll(b, strings.Repeat("a <<= 1\nb >>= 2\nc **= 3\nd //= 4\ne := f == g != h <= i >= j\n", 300))
}

func BenchmarkLexerIndentHeavy(b *testing.B) {
	var sb strings.Builder
	for range 300 {
		sb.WriteString("if cond:\n")
		sb.WriteString("    if nested:\n")
		sb.WriteString("        value = item\n")
		sb.WriteString("    else:\n")
		sb.WriteString("        value = other\n")
	}
	benchmarkLexAll(b, sb.String())
}

func BenchmarkLexerStringHeavy(b *testing.B) {
	benchmarkLexAll(b, strings.Repeat("msg = \"hello world\"\nlong = '''abc\ndef\nghi'''\n", 300))
}

func TestBytesStringLiterals(t *testing.T) {
	tests := []struct {
		input    string
		wantType TokenType
		wantLit  string
	}{
		// Basic bytes strings
		{`b"hello"`, BSTRING, "hello"},
		{`b'world'`, BSTRING, "world"},
		// Raw bytes strings (rb and br prefixes)
		{`rb"raw bytes"`, BSTRING, "raw bytes"},
		{`br"raw bytes"`, BSTRING, "raw bytes"},
		{`RB"uppercase"`, BSTRING, "uppercase"},
		{`Br"mixed"`, BSTRING, "mixed"},
		{`bR"mixed2"`, BSTRING, "mixed2"},
		// Triple-quoted bytes strings
		{`b"""triple quoted"""`, BSTRING, "triple quoted"},
		{`rb'''raw triple'''`, BSTRING, "raw triple"},
		// Empty bytes strings
		{`b""`, BSTRING, ""},
		{`rb''`, BSTRING, ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			l := New(tt.input)
			tok := l.NextToken()

			if tok.Type != tt.wantType {
				t.Errorf("want %v, got %v", tt.wantType, tok.Type)
			}
			if tok.Literal != tt.wantLit {
				t.Errorf("want literal %q, got %q", tt.wantLit, tok.Literal)
			}
		})
	}
}

func TestBytesVsStringTokenTypes(t *testing.T) {
	// Ensure bytes and strings are tokenized differently
	src := `s = "hello"
b = b"world"`
	l := New(src)

	// Skip NAME, EQUAL
	l.NextToken() // s
	l.NextToken() // =

	// Get string token
	strTok := l.NextToken()
	if strTok.Type != STRING {
		t.Errorf("expected STRING, got %v", strTok.Type)
	}
	if strTok.Literal != "hello" {
		t.Errorf("expected 'hello', got %q", strTok.Literal)
	}

	// Skip NEWLINE, NAME, EQUAL
	l.NextToken() // \n
	l.NextToken() // b
	l.NextToken() // =

	// Get bytes token
	bytesTok := l.NextToken()
	if bytesTok.Type != BSTRING {
		t.Errorf("expected BSTRING, got %v", bytesTok.Type)
	}
	if bytesTok.Literal != "world" {
		t.Errorf("expected 'world', got %q", bytesTok.Literal)
	}
}
