package server

import (
	"rahu/lsp"
	ast "rahu/parser/ast"
	"rahu/source"
)

func ToRange(li *source.LineIndex, r ast.Range) lsp.Range {
	sl, sc := li.OffsetToPosition(int(r.Start))
	el, ec := li.OffsetToPosition(int(r.End))

	return lsp.Range{
		Start: lsp.Position{
			Line:      sl,
			Character: sc,
		},
		End: lsp.Position{
			Line:      el,
			Character: ec,
		},
	}
}

func FromRange(li *source.LineIndex, r lsp.Range) ast.Range {
	start := li.PositionToOffset(r.Start.Line, r.Start.Character)
	end := li.PositionToOffset(r.End.Line, r.End.Character)

	return ast.Range{
		Start: uint32(start),
		End:   uint32(end),
	}
}
