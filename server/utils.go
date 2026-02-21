package server

import (
	"rahu/lsp"
	"rahu/parser"
	"rahu/source"
)

func ToRange(li *source.LineIndex, r parser.Range) lsp.Range {
	sl, sc := li.OffsetToPosition(r.Start)
	el, ec := li.OffsetToPosition(r.End)

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

func FromRange(li *source.LineIndex, r lsp.Range) parser.Range {
	start := li.PositionToOffset(r.Start.Line, r.Start.Character)
	end := li.PositionToOffset(r.End.Line, r.End.Character)

	return parser.Range{
		Start: start,
		End:   end,
	}
}
