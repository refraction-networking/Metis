package main

import (
	"github.com/elazarl/goproxy"
	"log"
	"net"
	"sync"
	"strconv"
	"net/http"
	"flag"
	"io"
	"bufio"
	"errors"
	"net/url"
)

var goproxyPort = 8181

type Endpoint struct {
	listener net.Listener
	mutex sync.RWMutex
}

func startGoProxy() {
	verbose := flag.Bool("v", true, "should every proxy request be logged to stdout")
	addr := flag.String("addr", ":" + strconv.Itoa(goproxyPort), "proxy listen address")
	flag.Parse()
	proxy := goproxy.NewProxyHttpServer()
	proxy.Verbose = *verbose
	log.Fatal(http.ListenAndServe(*addr, proxy))
}

func needsTapdance(url *url.URL) (bool) {
	//Hash url and check the bloom filter here
	return false
}

func orPanic(err error) {
	if err != nil {
		panic(err)
	}
}

func parseRequest(conn net.Conn)(string, *url.URL, error){
	connReader := bufio.Reader(conn)
	req, err := http.ReadRequest(&connReader)
	orPanic(err)
	if req.Method == "GET" || req.Method == "CONNECT" {
		return req.Method, req.URL, nil
	} else {
		err = errors.New("Chrome gave me "+req.Method+" instead of GET or CONNECT")
		return "", nil, err
	}
}

func getResource(clientConn net.Conn, id int) {
	reader := bufio.NewReader(clientConn)
	client := &http.Client{}
	defer clientConn.Close()
	req, err := http.ReadRequest(reader)
	orPanic(err)
	resp, err := client.Do(req)
	orPanic(err)
	resp.Write(clientConn)
}

func connectToResource(clientConn net.Conn, id int) {
	remoteConn, err := net.Dial("tcp", "localhost:" + strconv.Itoa(goproxyPort))
	orPanic(err)
	errChan := make(chan error)
	/*defer func() {
		clientConn.Close()
		remoteConn.Close()
		_ = <-errChan // wait for second goroutine to close
	}()*/

	forwardFromClientToGoproxy := func() {
		cBuf := make([]byte, 65536)
		n, err := io.CopyBuffer(remoteConn, clientConn, cBuf)
		orPanic(err)
		log.Println(id, ": Client request length: - ", n)
		errChan <- err
	}

	forwardFromGoproxyToClient := func() {
		rBuf := make([]byte, 65536)
		n, err := io.CopyBuffer(clientConn, remoteConn, rBuf)
		orPanic(err)
		log.Println(id, ": Remote response length: - ", n)
		errChan <- err
	}

	go forwardFromClientToGoproxy()
	go forwardFromGoproxyToClient()
	<- errChan
}

func (e *Endpoint) handleConnection(clientConn net.Conn, id int) {
	//Make a copy of clientConn
	clientConnCopy := new(net.Conn)
	io.Copy(clientConnCopy, clientConn)

	//Parse the copy (to see if it worked) as HTTP
	method, url, err := parseRequest(clientConnCopy)
	orPanic(err)
	//Check the bloom filter to see where request should be routed
	routeToTD := needsTapdance(url)
	if !routeToTD {
		if method == "GET" {
			getResource(clientConn, id)
		} else {
			connectToResource(clientConn, id)
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
		log.Println("Waiting for a connection request to accept.")
		//Spins until a request comes in
		conn, err := e.listener.Accept()
		if err != nil {
			log.Println("Failed accepting a connection request:", err)
			continue
		}
		log.Println("Accepted request, handling messages.")
		go e.handleConnection(conn, id)
		id++
	}
}

func main() {
	log.Println("Starting goproxy...")
	go startGoProxy()
	log.Println("Done.")
	endpt := new(Endpoint)
	log.Println("Starting my proxy....")
	endpt.Listen(8080)
}

