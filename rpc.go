package main

import (
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/rpc"
	"net/rpc/jsonrpc"
)

type HttpConnection struct {
	in  io.Reader
	out io.Writer
}

func (c *HttpConnection) Read(p []byte) (n int, err error)  { return c.in.Read(p) }
func (c *HttpConnection) Write(d []byte) (n int, err error) { return c.out.Write(d) }
func (c *HttpConnection) Close() error                      { return nil }

func httpHandler(w http.ResponseWriter, r *http.Request) {

	var connection HttpConnection = HttpConnection{r.Body, w}

	/*buf := new(strings.Builder)
	io.Copy(buf, r.Body)
	log.Println(buf.String())*/

	serverCodec := jsonrpc.NewServerCodec(&connection)

	rpc.ServeRequest(serverCodec)
}

func rpc_serve() (err error) {
	var listener net.Listener
	var control *Control = new(Control)
	var bind string = fmt.Sprintf("%s:%d", *comb_host, *comb_port)

	rpc.Register(control)

	if listener, err = net.Listen("tcp", bind); err != nil {
		return err
	}
	log.Printf("(rpc) started. listening on %s\n", bind)
	go http.Serve(listener, http.HandlerFunc(httpHandler))
	return nil
}

func rpc_start() {
	var err error
	if err = rpc_serve(); err != nil {
		log.Printf("(rpc) failed to start (%v)\n", err)
	}
}
