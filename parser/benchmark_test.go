package parser

import (
	"os"
	"testing"
)

func BenchmarkParseMega(b *testing.B) {
	src, err := os.ReadFile("../mega.py")
	if err != nil {
		b.Fatal(err)
	}

	input := string(src)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		p := New(input)
		_ = p.Parse()
	}
}

func BenchmarkParseSmall(b *testing.B) {
	src, err := os.ReadFile("../longerScript.py")
	if err != nil {
		b.Fatal(err)
	}

	input := string(src)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		p := New(input)
		_ = p.Parse()
	}
}
