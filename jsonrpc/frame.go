package jsonrpc

import (
	"bufio"
	j "encoding/json"
	"fmt"
	"io"
	s "strings"
)

func readHeader(r *bufio.Reader) (int, error) {
	found := false
	var contentLength int
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return 0, ParseError()
		}

		line = s.TrimSpace(line)

		if line == "" {
			break
		}

		valueStr, ok := s.CutPrefix(line, "Content-Length:")
		if !ok {
			continue
		}
		valueStr = s.TrimSpace(valueStr)

		_, err = fmt.Sscanf(valueStr, "%d", &contentLength)
		if err != nil {
			return 0, ParseError()
		}
		found = true
	}
	if !found {
		return 0, ParseError()
	}
	return contentLength, nil
}

func readBody(r *bufio.Reader) (Message, error) {
	length, err := readHeader(r)
	if err != nil {
		return nil, err
	}

	buf := make([]byte, length)
	if _, err := io.ReadFull(r, buf); err != nil {
		return nil, ParseError()
	}

	var peek struct {
		ID     j.RawMessage `json:"id"`
		Method string       `json:"method"`
		Result j.RawMessage `json:"result"`
		Error  j.RawMessage `json:"error"`
	}

	if err := j.Unmarshal(buf, &peek); err != nil {
		return nil, ParseError()
	}

	if peek.Method != "" {
		if len(peek.ID) > 0 && string(peek.ID) != "null" {
			var req Request
			if err := j.Unmarshal(buf, &req); err != nil {
				return nil, ParseError()
			}
			return &req, nil
		}
		var notif Notification
		if err := j.Unmarshal(buf, &notif); err != nil {
			return nil, ParseError()
		}
		return &notif, nil
	}

	if len(peek.ID) > 0 && string(peek.ID) != "null" && (len(peek.Result) > 0 || len(peek.Error) > 0) {
		var resp Response
		if err := j.Unmarshal(buf, &resp); err != nil {
			return nil, ParseError()
		}
		return &resp, nil
	}

	return nil, InvalidRequestError()
}
