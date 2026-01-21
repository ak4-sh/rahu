package jsonrpc

import (
	"bufio"
	s "strings"
	"testing"
)

func TestHeaderParsing(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantLength int
		wantErr    bool
	}{
		{
			name:       "valid header",
			input:      "Content-Length: 5\r\n\r\n",
			wantLength: 5,
			wantErr:    false,
		},
		{
			name:       "valid header with larger number",
			input:      "Content-Length: 12345\r\n\r\n",
			wantLength: 12345,
			wantErr:    false,
		},
		{
			name:       "valid header with extra spaces",
			input:      "Content-Length:    42   \r\n\r\n",
			wantLength: 42,
			wantErr:    false,
		},
		{
			name:       "header with Content-Type (should skip)",
			input:      "Content-Type: application/json\r\nContent-Length: 100\r\n\r\n",
			wantLength: 100,
			wantErr:    false,
		},
		{
			name:       "missing Content-Length",
			input:      "Content-Type: application/json\r\n\r\n",
			wantLength: 0,
			wantErr:    true,
		},
		{
			name:       "invalid number",
			input:      "Content-Length: abc\r\n\r\n",
			wantLength: 0,
			wantErr:    true,
		},
		{
			name:       "missing blank line",
			input:      "Content-Length: 5\r\n",
			wantLength: 0,
			wantErr:    true,
		},
		{
			name:       "lowercase content-length(unsupported)",
			input:      "content-length: 5\r\n\r\n",
			wantLength: 0,
			wantErr:    true,
		},
		{
			name:       "multiple content-length headers",
			input:      "Content-Length: 5\r\nContent-Length: 10\r\n\r\n",
			wantLength: 10,
			wantErr:    false,
		},
		{
			name:       "unknown headers after content-length",
			input:      "Content-Length: 7\r\nFoo: bar\r\n\r\n",
			wantLength: 7,
			wantErr:    false,
		},
		{
			name:       "zero content-length",
			input:      "Content-Length: 0\r\n\r\n",
			wantLength: 0,
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			sr := s.NewReader(tt.input)
			r := bufio.NewReader(sr)
			contentLength, err := readHeader(r)

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if contentLength != tt.wantLength {
				t.Errorf("got content length %d, want %d", contentLength, tt.wantLength)
			}
		})
	}
}

func TestReadBody_InvalidJSON(t *testing.T) {
	input := "Content-Length: 5\r\n\r\n{bad}"
	r := bufio.NewReader(s.NewReader(input))

	_, err := readBody(r)
	if err == nil {
		t.Fatalf("expected parse error")
	}
}
