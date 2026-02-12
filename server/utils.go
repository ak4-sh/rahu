package server

import (
	"rahu/lsp"
	"rahu/parser"
)

func ToRange(r parser.Range) lsp.Range {
	return lsp.Range{
		Start: lsp.Position{
			Line:      r.Start.Line,
			Character: r.Start.Col,
		},
		End: lsp.Position{
			Line:      r.End.Line,
			Character: r.End.Col,
		},
	}
}

func FromRange(r lsp.Range) parser.Range {
	return parser.Range{
		Start: parser.Position{
			Line: r.Start.Line,
			Col:  r.Start.Character,
		},
		End: parser.Position{
			Line: r.End.Line,
			Col:  r.End.Character,
		},
	}
}
