package jsonrpc

import j "encoding/json"

func AdaptRequest[T any, R any](
	h func(*T) (R, *Error),
) RequestHandler {
	return func(raw j.RawMessage) (j.RawMessage, *Error) {
		var params T
		if raw != nil {
			if err := j.Unmarshal(raw, &params); err != nil {
				return nil, InvalidParamsError(raw)
			}
		}

		result, rpcErr := h(&params)
		if rpcErr != nil {
			return nil, rpcErr
		}

		if any(result) == nil {
			return []byte("null"), nil
		}

		out, err := j.Marshal(result)
		if err != nil {
			return nil, InternalError()
		}
		return out, nil
	}
}

func AdaptNotification[T any](
	h func(*T),
) NotificationHandler {
	return func(raw j.RawMessage) {
		var params T

		if raw != nil {
			if err := j.Unmarshal(raw, &params); err != nil {
				return
			}
		}
		h(&params)
	}
}
