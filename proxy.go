package main

import (
	"log"
	"net"
	"net/http"
	"io"
	"bufio"
	"net/url"
	"github.com/sergeyfrolov/gotapdance/tapdance"
	"fmt"
	"github.com/refraction-networking/Metis/endpoint"
)

func isWhitelisted(url *url.URL) (bool) {
	//Hash url and check the Bloom filter here
	return true
}

func orPanic(err error) {
	if err != nil{
		log.Println(err)
		panic(err)
	}
}

func parseRequest(conn net.Conn)(*http.Request, error){
	connReader := bufio.NewReader(conn)
	req, err := http.ReadRequest(connReader)
	if err == io.EOF {return nil, err}
	orPanic(err)
	return req, nil
}

var client = &http.Client{}

func doHttpRequest(clientConn net.Conn, req *http.Request, id int) {
	log.Println(id, ": Performing non-CONNECT HTTP request")
	defer clientConn.Close()
	//http.Request has a field RequestURI that should be replaced by URL, RequestURI cannot be set for client.Do.
	req.RequestURI = ""
	resp, err := client.Do(req)
	orPanic(err)
	resp.Write(clientConn)
}


func connectToTapdance(clientConn net.Conn, req *http.Request, id int) (net.Conn, error){
	fmt.Println("req.URL.Hostname():req.URL.Port()", req.URL.Hostname()+":"+req.URL.Port())
	remoteConn, err := tapdance.Dial("tcp", req.URL.Hostname()+":"+req.URL.Port())
	orPanic(err)
	return remoteConn, err
}

func connectToResource(clientConn net.Conn, req *http.Request, id int, routeToTd bool) {
	log.Println(id, ": CONNECTing to resource")
	var remoteConn net.Conn
	var err error
	if(!routeToTd) {
		remoteConn, err = net.Dial("tcp", req.URL.Hostname()+":"+req.URL.Port())
	} else {
		remoteConn, err = connectToTapdance(clientConn, req, id)
	}
	orPanic(err)


	clientConn.Write([]byte("HTTP/1.1 200 Connection established\r\n\r\n"))

	errChan := make(chan error)
	defer func() {
		remoteConn.Close()
		clientConn.Close()
	}()

	forwardFromClientToRemote := func() {
		cBuf := make([]byte, 65536)
		n, err := io.CopyBuffer(remoteConn, clientConn, cBuf)
		log.Println(id, ": Client request length: - ", n)
		errChan <- err
	}

	forwardFromRemoteToClient := func() {
		rBuf := make([]byte, 65536)
		n, err := io.CopyBuffer(clientConn, remoteConn, rBuf)
		log.Println(id, ": Remote response length: - ", n)
		errChan <- err
	}

	go forwardFromClientToRemote()
	go forwardFromRemoteToClient()
	<- errChan
}

func handleConnection(clientConn net.Conn, id int) {
	defer func() {
		log.Println("Goroutine", id, "is closed.")
	}()
	//Parse the request as HTTP
	req, err := parseRequest(clientConn)
	if err == io.EOF { return }
	orPanic(err)
	method := req.Method
	reqUrl := req.URL

	//Check the bloom filter to see where request should be routed
	routeToTD := isWhitelisted(reqUrl)
	if !routeToTD && method== "CONNECT" {
		doHttpRequest(clientConn, req, id)
	} else {
		connectToResource(clientConn, req, id, routeToTD)
	}
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	endpt := new(endpoint.Endpoint)
	log.Println("Starting Metis proxy....")
	endpt.Listen(8080, handleConnection)
}

