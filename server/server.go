package main

import (
	"github.com/fvbock/trie"
	"log"
	"net"
	"strconv"
)

var masterList trie.Trie

func updateFilter(){

}

func (e *Endpoint) listen(port int) error{
	//Listen for updates sent by Metis clients
	//Upon reception of msg, add the trie included in it (with new sites for the blocked list)
	//to the list of tries to add to master.
	//Error checking? With what confidence do we list a site as blocked?
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

func main(){
	//log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	endpt := new(Endpoint)
	log.Println("Starting Metis server...")
	endpt.Listen(9090)
}
