package main

import (
	"bufio"
	"os"

	"rahu/jsonrpc"
	"rahu/server"
)

func main() {
	srv := server.New()
	server.Register(srv)

	in := bufio.NewReader(os.Stdin)
	out := bufio.NewWriter(os.Stdout)

	flush := func() error { return out.Flush() }

	conn := jsonrpc.NewConn(in, out, flush)
	conn.Start()
	jsonrpc.Dispatch(conn)

	conn.Wait()
}
