// testproxy is a throwaway HTTP CONNECT proxy for checking that a chat
// really goes through the proxy it is given: it logs every host asked for,
// and passes the connection on.
//
//	go run ./tools/testproxy 127.0.0.1:18888
//
// Then set a chat's proxy to http://127.0.0.1:18888 and open it: the
// lines it prints are the hosts that chat reached.
package main

import (
	"bufio"
	"io"
	"log"
	"net"
	"net/http"
	"os"
)

func main() {
	addr := "127.0.0.1:18888"
	if len(os.Args) > 1 {
		addr = os.Args[1]
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("listening on %s", addr)
	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Fatal(err)
		}
		go serve(conn)
	}
}

// serve reads one request: CONNECT tunnels through, anything else is
// answered with what the proxy is for.
func serve(conn net.Conn) {
	defer conn.Close()
	reader := bufio.NewReader(conn)
	req, err := http.ReadRequest(reader)
	if err != nil {
		return
	}
	if req.Method != http.MethodConnect {
		log.Printf("%s %s", req.Method, req.URL)
		io.WriteString(conn, "HTTP/1.1 501 Not Implemented\r\nContent-Length: 0\r\n\r\n")
		return
	}
	log.Printf("CONNECT %s", req.Host)
	upstream, err := net.Dial("tcp", req.Host)
	if err != nil {
		log.Printf("CONNECT %s failed: %v", req.Host, err)
		io.WriteString(conn, "HTTP/1.1 502 Bad Gateway\r\nContent-Length: 0\r\n\r\n")
		return
	}
	defer upstream.Close()
	io.WriteString(conn, "HTTP/1.1 200 Connection Established\r\n\r\n")
	done := make(chan struct{}, 2)
	go func() { io.Copy(upstream, reader); done <- struct{}{} }()
	go func() { io.Copy(conn, upstream); done <- struct{}{} }()
	<-done
}
