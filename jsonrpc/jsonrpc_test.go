package jsonrpc

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestConn_Read(t *testing.T) {
	req := `{"jsonrpc":"2.0","id":1,"method":"ping"}`
	input := fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(req), req)

	in := bytes.NewBufferString(input)
	out := &bytes.Buffer{}

	conn := NewConn(
		bufio.NewReader(in),
		bufio.NewWriter(out),
		func() error { return nil },
	)

	conn.Start()

	msg, ok := <-conn.Incoming()
	if !ok {
		t.Fatalf("incoming channel closed")
	}
	if _, ok := msg.(*Request); !ok {
		t.Fatalf("expected Request, got %T", msg)
	}

	conn.Close()
	conn.Wait()
}

func TestConn_SendResponse(t *testing.T) {
	in := bytes.NewBuffer(nil)
	out := &bytes.Buffer{}

	conn := NewConn(
		bufio.NewReader(in),
		bufio.NewWriter(out),
		func() error { return nil },
	)

	conn.Start()

	resp := &Response{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Result:  json.RawMessage(`"ok"`),
	}

	if err := conn.SendResponse(resp); err != nil {
		t.Fatal(err)
	}

	conn.Close()
	conn.Wait()

	output := out.String()
	if !strings.Contains(output, "Content-Length:") {
		t.Fatalf("missing Content-Length header")
	}
	if !strings.Contains(output, `"ok"`) {
		t.Fatalf("missing response body")
	}
}

func TestConn_GracefulDrain(t *testing.T) {
	in := bytes.NewBuffer(nil)
	out := &bytes.Buffer{}

	conn := NewConn(
		bufio.NewReader(in),
		bufio.NewWriter(out),
		func() error { return nil },
	)

	conn.Start()

	numMessages := 50
	for i := range numMessages {
		err := conn.Notify("test_drain", i)
		if err != nil {
			t.Fatalf("failed to enqueue message %d: %v", i, err)
		}
	}

	err := conn.Close()
	if err != nil {
		t.Fatalf("unexpected error on Close: %v", err)
	}

	output := out.String()
	actualCount := strings.Count(output, `"method":"test_drain"`)

	if actualCount != numMessages {
		t.Fatalf("expected %d messages to be drained and written, but found %d", numMessages, actualCount)
	}
}

func TestHeaderParsing(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantLength int
		wantErr    bool
	}{
		{name: "valid header", input: "Content-Length: 5\r\n\r\n", wantLength: 5, wantErr: false},
		{name: "valid header with larger number", input: "Content-Length: 12345\r\n\r\n", wantLength: 12345, wantErr: false},
		{name: "valid header with extra spaces", input: "Content-Length:    42   \r\n\r\n", wantLength: 42, wantErr: false},
		{name: "header with Content-Type (should skip)", input: "Content-Type: application/json\r\nContent-Length: 100\r\n\r\n", wantLength: 100, wantErr: false},
		{name: "missing Content-Length", input: "Content-Type: application/json\r\n\r\n", wantLength: 0, wantErr: true},
		{name: "invalid number", input: "Content-Length: abc\r\n\r\n", wantLength: 0, wantErr: true},
		{name: "missing blank line", input: "Content-Length: 5\r\n", wantLength: 0, wantErr: true},
		{name: "lowercase content-length(unsupported)", input: "content-length: 5\r\n\r\n", wantLength: 0, wantErr: true},
		{name: "multiple content-length headers", input: "Content-Length: 5\r\nContent-Length: 10\r\n\r\n", wantLength: 10, wantErr: false},
		{name: "unknown headers after content-length", input: "Content-Length: 7\r\nFoo: bar\r\n\r\n", wantLength: 7, wantErr: false},
		{name: "zero content-length", input: "Content-Length: 0\r\n\r\n", wantLength: 0, wantErr: false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			sr := strings.NewReader(tt.input)
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
	r := bufio.NewReader(strings.NewReader(input))

	_, err := readBody(r)
	if err == nil {
		t.Fatalf("expected parse error")
	}
}
