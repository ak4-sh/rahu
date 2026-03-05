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

// TestConn_GracefulDrain checks if the JSONRPC connection drains gracefully if
// a burst of messages are sent and the comm. channel is closed abruptly
func TestConn_GracefulDrain(t *testing.T) {
	in := bytes.NewBuffer(nil)
	out := &bytes.Buffer{}

	conn := NewConn(
		bufio.NewReader(in),
		bufio.NewWriter(out),
		func() error { return nil },
	)

	conn.Start()

	// Since c.outgoing has a buffer of 100, these enqueue instantly.
	numMessages := 50
	for i := range numMessages {
		err := conn.Notify("test_drain", i)
		if err != nil {
			t.Fatalf("failed to enqueue message %d: %v", i, err)
		}
	}

	// This signals c.closed, forcing the writeLoop to enter its draining phase.
	// conn.Close() will block until the writeLoop finishes draining and closes writeDone.
	err := conn.Close()
	if err != nil {
		t.Fatalf("unexpected error on Close: %v", err)
	}

	// Because Close() waited for the drain to finish, it is now safe to read the buffer
	// without any mutexes or data races.
	output := out.String()
	actualCount := strings.Count(output, `"method":"test_drain"`)

	if actualCount != numMessages {
		t.Fatalf("expected %d messages to be drained and written, but found %d", numMessages, actualCount)
	}
}
