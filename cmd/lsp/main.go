package main

import (
	"bufio"
	"os"

	"rahu/jsonrpc"
	"rahu/server"
)

func main() {
	in := bufio.NewReader(os.Stdin)
	out := bufio.NewWriter(os.Stdout)

	closeFn := func() error { return os.Stdin.Close() }

	conn := jsonrpc.NewConn(in, out, closeFn)

	srv := server.New(conn)
	server.Register(srv)

	conn.Start()

	jsonrpc.Dispatch(conn)

	conn.Wait()
}
