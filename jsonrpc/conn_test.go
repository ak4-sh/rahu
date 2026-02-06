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
