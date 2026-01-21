package jsonrpc

import j "encoding/json"

type (
	RequestHandler      func(j.RawMessage) (j.RawMessage, *Error)
	NotificationHandler func(j.RawMessage)
)

var requestHandlers = map[string]RequestHandler{}

var notificationHandlers = map[string]NotificationHandler{}

func RegisterRequest(method string, h RequestHandler) {
	requestHandlers[method] = h
}

func RegisterNofication(method string, h NotificationHandler) {
	notificationHandlers[method] = h
}
