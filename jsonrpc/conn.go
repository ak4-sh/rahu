package jsonrpc

import (
	"bufio"
	"context"
	j "encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
)

type outbound struct {
	payload []byte
}

type Conn struct {
	r             *bufio.Reader
	w             *bufio.Writer
	closeFn       func() error
	incoming      chan Message
	outgoing      chan outbound
	errors        chan error
	once          sync.Once
	closeOnce     sync.Once
	ctx           context.Context
	cancel        context.CancelFunc
	writeDone     chan struct{}
	closing       sync.Once
	closed        chan struct{}
	nextRequestID atomic.Uint64
	pendingMu     sync.Mutex
	pending       map[string]chan *Response
}

func NewConn(reader *bufio.Reader, writer *bufio.Writer, closeFn func() error) *Conn {
	if closeFn == nil {
		panic("closeFn must be provided to unblock reader")
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &Conn{
		r:         reader,
		w:         writer,
		closeFn:   closeFn,
		incoming:  make(chan Message, 100),
		outgoing:  make(chan outbound, 100),
		errors:    make(chan error, 2),
		ctx:       ctx,
		cancel:    cancel,
		writeDone: make(chan struct{}),
		closed:    make(chan struct{}),
		pending:   make(map[string]chan *Response),
	}
}

func (c *Conn) markClosed() {
	c.closing.Do(func() {
		close(c.closed)
	})
}

func (c *Conn) enqueueBytes(b []byte) error {
	select {
	case <-c.closed:
		return fmt.Errorf("connection closed")
	default:
	}

	select {
	case <-c.closed:
		return fmt.Errorf("connection closed")
	case c.outgoing <- outbound{payload: b}:
		return nil
	}
}

func (c *Conn) readLoop() {
	defer func() {
		if r := recover(); r != nil {
			c.fail(fmt.Errorf("panic in readLoop: %v", r))
		}
		close(c.incoming)
	}()

	for {
		msg, err := readBody(c.r)
		if err != nil {
			return
		}

		select {
		case c.incoming <- msg:
		case <-c.ctx.Done():
			return
		}
	}
}

func (c *Conn) writeLoop() {
	defer func() {
		if r := recover(); r != nil {
			c.fail(fmt.Errorf("panic in writeLoop: %v", r))
		}
		close(c.writeDone)
	}()

	for {
		select {
		case <-c.closed:
			// Shutdown initiated. Drain the remaining messages, then exit.
			for {
				select {
				case msg := <-c.outgoing:
					c.writeMessage(msg)
				default:
					return // Buffer is fully drained
				}
			}

		case msg := <-c.outgoing:
			c.writeMessage(msg)
		}
	}
}

// Extracted for readability
func (c *Conn) writeMessage(msg outbound) {
	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(msg.payload))
	if _, err := c.w.WriteString(header); err != nil {
		c.fail(err)
	}
	if _, err := c.w.Write(msg.payload); err != nil {
		c.fail(err)
	}
	if err := c.w.Flush(); err != nil {
		c.fail(err)
	}
}

func (c *Conn) Errors() <-chan error {
	return c.errors
}

func (c *Conn) Notify(method string, params any) error {
	msg := map[string]any{
		"jsonrpc": "2.0",
		"method":  method,
		"params":  params,
	}
	b, err := j.Marshal(msg)
	if err != nil {
		return err
	}

	return c.enqueueBytes(b)
}

func pendingKey(id j.RawMessage) string {
	return string(id)
}

func (c *Conn) Request(ctx context.Context, method string, params any) (*Response, error) {
	idNum := c.nextRequestID.Add(1)
	id := j.RawMessage([]byte(fmt.Sprintf("%d", idNum)))
	msg := map[string]any{
		"jsonrpc": "2.0",
		"id":      idNum,
		"method":  method,
		"params":  params,
	}
	b, err := j.Marshal(msg)
	if err != nil {
		return nil, err
	}

	respCh := make(chan *Response, 1)
	key := pendingKey(id)
	c.pendingMu.Lock()
	c.pending[key] = respCh
	c.pendingMu.Unlock()

	cleanup := func() {
		c.pendingMu.Lock()
		delete(c.pending, key)
		c.pendingMu.Unlock()
	}

	if err := c.enqueueBytes(b); err != nil {
		cleanup()
		return nil, err
	}

	select {
	case resp := <-respCh:
		cleanup()
		return resp, nil
	case <-ctx.Done():
		cleanup()
		return nil, ctx.Err()
	case <-c.closed:
		cleanup()
		return nil, fmt.Errorf("connection closed")
	}
}

func (c *Conn) deliverResponse(resp *Response) {
	if resp == nil {
		return
	}
	key := pendingKey(resp.ID)
	c.pendingMu.Lock()
	respCh, ok := c.pending[key]
	c.pendingMu.Unlock()
	if !ok {
		return
	}
	select {
	case respCh <- resp:
	default:
	}
}

func (c *Conn) SendResponse(resp *Response) error {
	b, err := j.Marshal(resp)
	if err != nil {
		return err
	}
	return c.enqueueBytes(b)
}

func (c *Conn) Incoming() <-chan Message {
	return c.incoming
}

func (c *Conn) Close() error {
	c.markClosed()
	<-c.writeDone
	c.cancel()
	c.closeOnce.Do(func() {
		_ = c.closeFn()
	})
	return nil
}

func (c *Conn) Start() {
	go c.readLoop()
	go c.writeLoop()
}

func (c *Conn) reportError(err error) {
	if err == nil {
		return
	}
	select {
	case c.errors <- err:
	default:
	}
}

func (c *Conn) fail(err error) {
	c.once.Do(func() {
		c.reportError(err)
		c.markClosed()
		c.cancel()
		c.closeOnce.Do(
			func() {
				_ = c.closeFn()
			},
		)
	})
}

// Wait blocks until shutdown is initiated (fail called). It does not guarantee readLoop exits
// unless closeFn unblocks the reader.
func (c *Conn) Wait() {
	<-c.ctx.Done()
	<-c.writeDone
}
