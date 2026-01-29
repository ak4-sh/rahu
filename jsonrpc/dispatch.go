package jsonrpc

import (
	j "encoding/json"
)

func Dispatch(conn *Conn) {
	for msg := range conn.Incoming() {
		switch m := msg.(type) {
		case *Request:
			go dispatchRequest(conn, m)
		case *Notification:
			go dispatchNotification(conn, m)

		default:
			// unreachable if framing is correct
		}
	}
}

func dispatchNotification(conn *Conn, notif *Notification) {
	handler, ok := notificationHandlers[notif.Method]
	if !ok {
		return
	}
	handler(notif.Params)
}

func dispatchRequest(conn *Conn, req *Request) {
	resp := &Response{
		JSONRPC: "2.0",
		ID:      req.ID,
	}

	handler, ok := requestHandlers[req.Method]

	if !ok {
		resp.Error = MethodNotFoundError()
		_ = conn.SendResponse(resp)
		return
	}

	var (
		result j.RawMessage
		err    *Error
	)

	func() {
		defer func() {
			if r := recover(); r != nil {
				err = InternalError()
			}
		}()
		result, err = handler(req.Params)
	}()

	if err != nil {
		resp.Error = err
	} else {
		resp.Result = result
	}

	_ = conn.SendResponse(resp)
}
