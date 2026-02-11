package jsonrpc

import (
	j "encoding/json"
	"log"
)

func Dispatch(conn *Conn) {
	log.Println("DISPATCH: started")

	for msg := range conn.Incoming() {
		log.Printf("DISPATCH: received message type %T", msg)

		switch m := msg.(type) {
		case *Request:
			log.Printf("DISPATCH: request %s", m.Method)
			go dispatchRequest(conn, m)

		case *Notification:
			log.Printf("DISPATCH: notification %s", m.Method)
			go dispatchNotification(conn, m)

		default:
			log.Printf("DISPATCH: unknown message %+v", msg)
		}
	}

	log.Println("DISPATCH: exiting")
}

func dispatchNotification(conn *Conn, notif *Notification) {
	handler, ok := notificationHandlers[notif.Method]
	if !ok {
		return
	}

	defer func() {
		if r := recover(); r != nil {
			conn.Close()
		}
	}()
	handler(notif.Params)
}

func dispatchRequest(conn *Conn, req *Request) {
	resp := &Response{
		JSONRPC: "2.0",
		ID:      req.ID,
	}

	handler, ok := requestHandlers[req.Method]

	if !ok {
		log.Printf("DISPATCH: method not found: %s", req.Method)
		resp.Error = MethodNotFoundError()
		if err := conn.SendResponse(resp); err != nil {
			conn.Close()
		}
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
