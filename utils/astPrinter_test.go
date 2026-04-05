package utils

import (
	"bytes"
	"strings"
	"testing"

	"rahu/parser"
)

func TestPrintAST_PartialFunctionDoesNotMislabelArgsAsBody(t *testing.T) {
	p := parser.New("def foo(x=bar)")
	tree := p.Parse()

	var out bytes.Buffer
	PrintAST(&out, tree)

	printed := out.String()
	if !strings.Contains(printed, "Args:") {
		t.Fatalf("expected args section in output: %s", printed)
	}
	if strings.Contains(printed, "Body:") {
		t.Fatalf("did not expect body section for partial function: %s", printed)
	}
	if !strings.Contains(printed, "Name(bar)") {
		t.Fatalf("expected default expression to be printed: %s", printed)
	}
}

func TestPrintAST_PartialClassDoesNotMislabelBasesAsBody(t *testing.T) {
	p := parser.New("class Foo(Bar)")
	tree := p.Parse()

	var out bytes.Buffer
	PrintAST(&out, tree)

	printed := out.String()
	if !strings.Contains(printed, "Bases:") {
		t.Fatalf("expected bases section in output: %s", printed)
	}
	if strings.Contains(printed, "Body:") {
		t.Fatalf("did not expect body section for partial class: %s", printed)
	}
	if !strings.Contains(printed, "Name(Bar)") {
		t.Fatalf("expected base expression to be printed: %s", printed)
	}
}
