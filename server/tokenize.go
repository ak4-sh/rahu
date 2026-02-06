package server

import "rahu/lexer"

func tokenize(text string) []lexer.Token {
	l := lexer.New(text)

	var toks []lexer.Token

	for {
		t := l.NextToken()
		toks = append(toks, t)
		if t.Type == lexer.EOF {
			break
		}
	}

	return toks
}
