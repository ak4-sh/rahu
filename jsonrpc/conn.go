package jsonrpc

import (
	"bufio"
	"context"
	j "encoding/json"
	"fmt"
	"sync"
)

type outbound struct {
	payload []byte
}

type Conn struct {
	r         *bufio.Reader
	w         *bufio.Writer
	closeFn   func() error
	incoming  chan Message
	outgoing  chan outbound
	errors    chan error
	once      sync.Once
	ctx       context.Context
	cancel    context.CancelFunc
	writeDone chan struct{}
	closing   sync.Once
	closed    chan struct{}
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
	}
}

func (c *Conn) markClosed() {
	c.closing.Do(func() {
		close(c.closed)
		close(c.outgoing)
	})
}

func (c *Conn) enqueueBytes(b []byte) error {
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

	for msg := range c.outgoing {

		header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(msg.payload))

		if _, err := c.w.WriteString(header); err != nil {
			c.fail(err)
			return
		}

		if _, err := c.w.Write(msg.payload); err != nil {
			c.fail(err)
			return
		}
		if err := c.w.Flush(); err != nil {
			c.fail(err)
			return
		}

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
	_ = c.closeFn()
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
		_ = c.closeFn()
	})
}

// Wait blocks until shutdown is initiated (fail called). It does not guarantee readLoop exits
// unless closeFn unblocks the reader.
func (c *Conn) Wait() {
	<-c.ctx.Done()
	<-c.writeDone
}
