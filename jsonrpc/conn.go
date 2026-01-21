package jsonrpc

import (
	"bufio"
	j "encoding/json"
	"fmt"
	"sync"
)

type Conn struct {
	r        *bufio.Reader
	w        *bufio.Writer
	closeFn  func() error
	incoming chan Message
	outgoing chan *Response
	errors   chan error
	once     sync.Once
	done     chan struct{}
}

func NewConn(reader *bufio.Reader, writer *bufio.Writer, closeFn func() error) *Conn {
	return &Conn{
		r:        reader,
		w:        writer,
		closeFn:  closeFn,
		incoming: make(chan Message, 10),
		outgoing: make(chan *Response, 10),
		errors:   make(chan error, 2),
		done:     make(chan struct{}),
	}
}

func (c *Conn) Write(resp *Response) error {
	content, err := j.Marshal(resp)
	if err != nil {
		return err
	}

	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(content))

	if _, err := c.w.WriteString(header); err != nil {
		return err
	}

	if _, err := c.w.Write(content); err != nil {
		return err
	}

	return c.w.Flush()
}

func (c *Conn) readLoop() {
	defer func() {
		close(c.incoming)
	}()
	for {
		msg, err := readBody(c.r)
		if err != nil {
			c.fail(fmt.Errorf("read error: %w", err))
			return
		}
		c.incoming <- msg
	}
}

func (c *Conn) writeLoop() {
	for resp := range c.outgoing {
		if err := c.Write(resp); err != nil {
			c.fail(fmt.Errorf("write error: %w", err))
			return
		}
	}
}

func (c *Conn) SendResponse(resp *Response) error {
	select {
	case c.outgoing <- resp:
		return nil
	case <-c.done:
		return fmt.Errorf("connection closed")
	}
}

func (c *Conn) Incoming() <-chan Message {
	return c.incoming
}

func (c *Conn) Errors() <-chan error {
	return c.errors
}

func (c *Conn) Close() error {
	c.fail(nil)
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

		close(c.outgoing)

		if c.closeFn != nil {
			_ = c.closeFn()
		}

		close(c.done)
	})
}

// Wait blocks until shutdown is initiated (fail called). It does not guarantee readLoop exits
// unless closeFn unblocks the reader.
func (c *Conn) Wait() {
	<-c.done
}
