// Package jsonrpc implements the JSON-RPC protocol to be used in the LSP protocol implementation
package jsonrpc

import j "encoding/json"

type Message interface {
	isMessage()
}

type Request struct {
	JSONRPC string       `json:"jsonrpc"`
	Method  string       `json:"method"`
	ID      j.RawMessage `json:"id"`
	Params  j.RawMessage `json:"params,omitempty"`
}

type Notification struct {
	JSONRPC string       `json:"jsonrpc"`
	Method  string       `json:"method"`
	Params  j.RawMessage `json:"params,omitempty"`
}

type Response struct {
	JSONRPC string       `json:"jsonrpc"`
	Result  j.RawMessage `json:"result,omitempty"`
	Error   *Error       `json:"error,omitempty"`
	ID      j.RawMessage `json:"id"`
}

func (*Request) isMessage()      {}
func (*Response) isMessage()     {}
func (*Notification) isMessage() {}
