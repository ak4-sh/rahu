package source

import (
	"testing"
)

func TestApplyEdit(t *testing.T) {
	tests := []struct {
		name        string
		initial     string
		startLine   int
		startChar   int
		endLine     int
		endChar     int
		newText     string
		wantFinal   string
	}{
		{
			name:      "single char insert mid-line",
			initial:   "hello world\n",
			startLine: 0, startChar: 5,
			endLine: 0, endChar: 5,
			newText:   ",",
			wantFinal: "hello, world\n",
		},
		{
			name:      "single char delete",
			initial:   "hello world\n",
			startLine: 0, startChar: 5,
			endLine: 0, endChar: 6,
			newText:   "",
			wantFinal: "helloworld\n",
		},
		{
			name:      "replace word same line",
			initial:   "foo bar baz\n",
			startLine: 0, startChar: 4,
			endLine: 0, endChar: 7,
			newText:   "qux",
			wantFinal: "foo qux baz\n",
		},
		{
			name:      "insert newline splits line",
			initial:   "abc def\n",
			startLine: 0, startChar: 3,
			endLine: 0, endChar: 4,
			newText:   "\n",
			wantFinal: "abc\ndef\n",
		},
		{
			name:      "multi-line insert",
			initial:   "line1\nline2\nline3\n",
			startLine: 1, startChar: 0,
			endLine: 1, endChar: 5,
			newText:   "new\nlines\nhere",
			wantFinal: "line1\nnew\nlines\nhere\nline3\n",
		},
		{
			name:      "multi-line delete collapses to one line",
			initial:   "aaa\nbbb\nccc\n",
			startLine: 0, startChar: 3,
			endLine: 1, endChar: 3,
			newText:   "",
			wantFinal: "aaa\nccc\n",
		},
		{
			name:      "pure deletion spanning lines",
			initial:   "aaa\nbbb\nccc\n",
			startLine: 0, startChar: 0,
			endLine: 1, endChar: 3,
			newText:   "",
			wantFinal: "\nccc\n",
		},
		{
			name:      "edit at end of file",
			initial:   "hello",
			startLine: 0, startChar: 5,
			endLine: 0, endChar: 5,
			newText:   " world",
			wantFinal: "hello world",
		},
		{
			name:      "empty newText is pure deletion",
			initial:   "abcdef",
			startLine: 0, startChar: 2,
			endLine: 0, endChar: 4,
			newText:   "",
			wantFinal: "abef",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			li := NewLineIndex(tt.initial)

			startOff := li.PositionToOffset(tt.startLine, tt.startChar)
			endOff := li.PositionToOffset(tt.endLine, tt.endChar)

			newLI := li.ApplyEdit(startOff, endOff, tt.newText)
			got := tt.initial[:startOff] + tt.newText + tt.initial[endOff:]

			if got != tt.wantFinal {
				t.Fatalf("text mismatch: got %q, want %q", got, tt.wantFinal)
			}

			// Verify newLI matches NewLineIndex on the final text.
			wantLI := NewLineIndex(tt.wantFinal)
			if len(newLI.lineStarts) != len(wantLI.lineStarts) {
				t.Fatalf("lineStarts len: got %d, want %d\ngot:  %v\nwant: %v",
					len(newLI.lineStarts), len(wantLI.lineStarts),
					newLI.lineStarts, wantLI.lineStarts)
			}
			for i := range wantLI.lineStarts {
				if newLI.lineStarts[i] != wantLI.lineStarts[i] {
					t.Errorf("lineStarts[%d]: got %d, want %d", i, newLI.lineStarts[i], wantLI.lineStarts[i])
				}
			}
		})
	}
}
