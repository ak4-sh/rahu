package source

type LineIndex struct {
	lineStarts []int
}

func NewLineIndex(text string) *LineIndex {
	// Pre-allocate based on estimated line count (~40 chars per line for typical code)
	estimatedLines := len(text)/40 + 1
	starts := make([]int, 1, estimatedLines)
	starts[0] = 0
	for i, b := range []byte(text) {
		if b == '\n' {
			starts = append(starts, i+1)
		}
	}
	return &LineIndex{lineStarts: starts}
}

func (li *LineIndex) OffsetToPosition(off int) (line int, col int) {
	if off < 0 {
		off = 0
	}

	lo, hi := 0, len(li.lineStarts)
	for lo < hi {
		mid := (lo + hi) / 2
		if li.lineStarts[mid] <= off {
			lo = mid + 1
		} else {
			hi = mid
		}
	}

	line = max(lo-1, 0)

	col = off - li.lineStarts[line]
	return
}

func (li *LineIndex) PositionToOffset(line int, col int) int {
	if line < 0 {
		return 0
	}
	if line >= len(li.lineStarts) {
		return li.lineStarts[len(li.lineStarts)-1]
	}

	start := li.lineStarts[line]
	off := start + col

	if off < start {
		return start
	}
	return off
}

// lineForOffset returns the line number containing the given byte offset.
func (li *LineIndex) lineForOffset(off int) int {
	lo, hi := 0, len(li.lineStarts)
	for lo < hi {
		mid := (lo + hi) / 2
		if li.lineStarts[mid] <= off {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	return max(lo-1, 0)
}

// ApplyEdit returns a new LineIndex reflecting a text replacement from startOffset to
// endOffset with newText, without rescanning the entire file.
func (li *LineIndex) ApplyEdit(startOffset, endOffset int, newText string) *LineIndex {
	startLine := li.lineForOffset(startOffset)
	endLine := li.lineForOffset(endOffset)
	delta := len(newText) - (endOffset - startOffset)

	// Count newlines in newText to know how many new line-start entries to add.
	newLineCount := 0
	for i := 0; i < len(newText); i++ {
		if newText[i] == '\n' {
			newLineCount++
		}
	}

	// Layout of new lineStarts:
	//   [0 .. startLine]           unchanged prefix (startLine+1 entries)
	//   [startLine+1 .. startLine+newLineCount]  new lines inside newText
	//   [startLine+newLineCount+1 ..]            shifted tail (lines after endLine)
	oldStarts := li.lineStarts
	tailStart := endLine + 1
	tailCount := len(oldStarts) - tailStart
	newStarts := make([]int, startLine+1+newLineCount+tailCount)

	// Unchanged prefix.
	copy(newStarts[:startLine+1], oldStarts[:startLine+1])

	// New line starts inside newText.
	pos := startOffset
	idx := startLine + 1
	for i := 0; i < len(newText); i++ {
		pos++
		if newText[i] == '\n' {
			newStarts[idx] = pos
			idx++
		}
	}

	// Shifted tail.
	for i, s := range oldStarts[tailStart:] {
		newStarts[idx+i] = s + delta
	}

	return &LineIndex{lineStarts: newStarts}
}
