package main

import (
	"github.com/fvbock/trie"
	"log"
	"net"
	//"github.com/refraction-networking/Metis/endpoint"
	"encoding/json"
	"github.com/gorilla/mux"
	"net/http"
)

var masterList trie.Trie

func updateFilter(){

}

func handleMsg(clientConn net.Conn, id int){
	//Add the trie included in the message (with new sites for the blocked list)
	//to the list of tries to add to master.
	//Error checking? With what confidence do we list a site as blocked?
}

type Website struct {
	Domain string `json:"domain,omitempty"`
}

var blockedList []Website

func getBlocked(writer http.ResponseWriter, req *http.Request){
	json.NewEncoder(writer).Encode(blockedList)
}

func addBlocked(writer http.ResponseWriter, req *http.Request){

}

func main(){
	//log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	//endpt := new(endpoint.Endpoint)
	log.Println("Starting Metis server...")
	//endpt.Listen(9090, handleMsg)
	router := mux.NewRouter()
	router.HandleFunc("/blocked", getBlocked).Methods("GET")
	router.HandleFunc("/blocked/add", addBlocked).Methods("POST")
	blockedList = append(blockedList, Website{Domain: "www.facebook.com"})
	blockedList = append(blockedList, Website{Domain: "www.google.com"})
	log.Fatal(http.ListenAndServe(":9090", router))
}
