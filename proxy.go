package main

import (
	"log"
	"net"
	"sync"
	"strconv"
	"net/http"
	"io"
	"bufio"
	"net/url"
)

type Endpoint struct {
	listener net.Listener
	mutex sync.RWMutex
}

func needsTapdance(url *url.URL) (bool) {
	//Hash url and check the bloom filter here
	return false
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

func connectToResource(clientConn net.Conn, req *http.Request, id int) {
	log.Println(id, ": CONNECTing to resource")
	remoteConn, err := net.Dial("tcp", req.RequestURI)
	orPanic(err)

	clientConn.Write([]byte("HTTP/1.1 200 Connection established\r\n\r\n"))

	errChan := make(chan error)
	defer func() {
		remoteConn.Close()
		clientConn.Close()
	}()

	forwardFromClientToGoproxy := func() {
		cBuf := make([]byte, 65536)
		n, err := io.CopyBuffer(remoteConn, clientConn, cBuf)
		log.Println(id, ": Client request length: - ", n)
		errChan <- err
	}

	forwardFromGoproxyToClient := func() {
		rBuf := make([]byte, 65536)
		n, err := io.CopyBuffer(clientConn, remoteConn, rBuf)
		log.Println(id, ": Remote response length: - ", n)
		errChan <- err
	}

	go forwardFromClientToGoproxy()
	go forwardFromGoproxyToClient()
	<- errChan
}

func (e *Endpoint) handleConnection(clientConn net.Conn, id int) {
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
	routeToTD := needsTapdance(reqUrl)
	if !routeToTD {
		if method == "CONNECT" {
			connectToResource(clientConn, req, id)
		} else {
			doHttpRequest(clientConn, req, id)
		}
	}
}


func (e *Endpoint) Listen(port int) error {
	id := 0
	var err error
	portStr := strconv.Itoa(port)
	e.listener, err = net.Listen("tcp", "127.0.0.1:"+portStr)
	if err != nil {
		log.Println("Unable to listen on port", portStr, err)
		return err
	}
	log.Println("Listening on", e.listener.Addr().String())
	for {
		log.Println(id, ": Waiting for a connection request to accept.")
		//Spins until a request comes in
		conn, err := e.listener.Accept()
		if err != nil {
			log.Println("Failed accepting a connection request:", err)
			continue
		}
		log.Println(id, ": Accepted request, handling messages.")
		go e.handleConnection(conn, id)
		id++
	}
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	endpt := new(Endpoint)
	log.Println("Starting my proxy....")
	endpt.Listen(8080)
}

