package endpoint

import (
	"net"
	"sync"
	"log"
	"strconv"
)

type Endpoint struct {
	listener net.Listener
	mutex sync.RWMutex
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
