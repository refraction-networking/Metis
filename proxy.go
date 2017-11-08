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
	"strconv"
	"sync"
	"encoding/json"
)

type Endpoint struct {
	listener net.Listener
	mutex sync.RWMutex
}

type Website struct {
	Domain string `json:"domain,omitempty"`
}

var client = &http.Client{}
var blockedDomains []string

func contains(slice []string, s string) bool {
	for _, e := range slice {
		if e == s { return true}
	}
	return false
}

func isBlocked(url *url.URL) (bool) {
	return contains(blockedDomains, url.Hostname())
}

func orPanic(err error) {
	if err != nil{
		log.Println(err)
		panic(err)
	}
}

func updateBlockedList() {
	req, err := http.NewRequest("GET", "HTTP://localhost:9090/blocked", nil)
	orPanic(err)
	resp, err := client.Do(req)
	orPanic(err)
	dec := json.NewDecoder(resp.Body)

	// read open bracket
	t, err := dec.Token()
	orPanic(err)
	fmt.Printf("%T: %v\n", t, t)

	// while the array contains values
	for dec.More() {
		var site Website
		// decode an array value (Message)
		err := dec.Decode(&site)
		orPanic(err)
		fmt.Printf("Domain: %v\n", site.Domain)
		if !contains(blockedDomains,site.Domain) {
			blockedDomains = append(blockedDomains, site.Domain)
		}
	}

	// read closing bracket
	t, err = dec.Token()
	orPanic(err)
	fmt.Printf("%T: %v\n", t, t)

}

func parseRequest(conn net.Conn)(*http.Request, error){
	connReader := bufio.NewReader(conn)
	req, err := http.ReadRequest(connReader)
	if err == io.EOF {return nil, err}
	orPanic(err)
	return req, nil
}

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
	routeToTransport := isBlocked(reqUrl)
	if !routeToTransport && method != "CONNECT" {
		doHttpRequest(clientConn, req, id)
	} else {
		connectToResource(clientConn, req, id, routeToTransport)
	}
}

func (e *Endpoint) handleConnection(clientConn net.Conn, id int, handler func(net.Conn, int)) {
	handler(clientConn, id)
}

func (e *Endpoint) Listen(port int, handler func(net.Conn, int)) error {
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
		go e.handleConnection(conn, id, handler)
		id++
	}
}


func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	endpt := new(Endpoint)
	log.Println("Starting Metis proxy....")
	updateBlockedList()
	endpt.Listen(8080, handleConnection)
}

