package Metis_server

import (
	"github.com/fvbock/trie"
	"log"
	"net"
	"github.com/refraction-networking/Metis/endpoint"
)

var masterList trie.Trie

func updateFilter(){

}

func handleMsg(clientConn net.Conn, id int){
	//Listen for updates sent by Metis clients
	//Upon reception of msg, add the trie included in it (with new sites for the blocked list)
	//to the list of tries to add to master.
	//Error checking? With what confidence do we list a site as blocked?
}

func main(){
	//log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	endpt := new(endpoint.Endpoint)
	log.Println("Starting Metis server...")
	endpt.Listen(9090, handleMsg)
}
