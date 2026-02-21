package source

type LineIndex struct {
	lineStarts []int
}

func NewLineIndex(text string) *LineIndex {
	starts := []int{0}
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

	line = lo - 1
	if line < 0 {
		line = 0
	}

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
