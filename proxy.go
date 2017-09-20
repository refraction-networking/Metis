package main

import (
	"github.com/elazarl/goproxy"
	"log"
	"net"
	"sync"
	"strconv"
	"bufio"
	"net/http"
)

var client = &http.Client{}

type Endpoint struct {
	listener net.Listener
	mutex sync.RWMutex
}

func (e *Endpoint) handleConnection(clientConn net.Conn) {
	reader := bufio.NewReader(clientConn)
	defer clientConn.Close()

	req, err := http.ReadRequest(reader)
	if err != nil {
		log.Println("Error parsing HTTP request: ", err)
		return
	}
	switch req.Method {
	case "GET":
		log.Println("Received GET request")
		resp, err := client.Do(req)
		if err != nil {
			log.Println("Couldn't perform GET request!")
			//TODO: Handle this error correctly instead of returning an empty response. How do we do that?
			resp = new(http.Response)
		}
		resp.Write(clientConn)
	case "CONNECT":
		log.Println("Received CONNECT request")
		servConn, err := net.Dial("tcp", req.URL.Host)
		if err != nil {
			log.Println("Can't connect using net.Dial")
			//TODO: Handle this more gracefully
			return
		}
		//send 200 ok? Why?
		status, err := bufio.NewReader(servConn).ReadString('\n')
		//log.Println(status)
		//TODO: Send requests from clientConn to servConn and send responses in the opposite direction
	}

}


func (e *Endpoint) Listen(port int) error {
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
		go e.handleConnection(conn)
	}
}

//listen on incoming port
//accept incoming connection
// parses the request
// print the info.
func main() {
	endpt := new(Endpoint)
	endpt.Listen(8080)
}

