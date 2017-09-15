package main

import (
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

func (e *Endpoint) handleConnection(conn net.Conn) {
	reader := bufio.NewReader(conn)
	defer conn.Close()

	req, err := http.ReadRequest(reader)
	if err != nil {
		log.Println("Error parsing HTTP request: ", err)
		return
	}
	switch req.Method {
	case "GET":
		resp, err := client.Do(req)
		if err != nil {
			log.Println("Couldn't perform GET request!")
			//TODO: Handle this error correctly instead of returning an empty response. How do we do that?
			resp = new(http.Response)
		}
		resp.Write(conn)
	case "CONNECT":

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

