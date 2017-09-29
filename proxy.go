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

func needsTapdance(req *http.Request) (bool) {
	return false
}

func orPanic(err error) {
	if err != nil {
		panic(err)
	}
}

/*if needsTapdance(req) {
		log.Println("Request needs tapdance, connecting to Tapdance client")
		//write request out to Tapdance port
	} else {
		log.Println("Request doesn't need Tapdance, passing to goproxy") */
//write request to connection goproxy is listening on
/*conn, err := net.Dial("tcp", "localhost:" + strconv.Itoa(goproxyPort))
if err != nil {
	log.Println("ERROR: Couldn't connect to goproxy.")
}
req.WriteProxy(conn)*/


func (e *Endpoint) handleConnection(clientConn net.Conn, id int) {
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
	//Until an error gets thrown?
	//log.Println("Entering for/while loop...")
	//for {
		//req, err := http.ReadRequest(clientBuf.Reader)



		//orPanic(req.Write(remoteBuf))
		//orPanic(remoteBuf.Flush())
		//Get the response
		//resp, err := http.ReadResponse(remoteBuf.Reader, req)
		//log.Println(id, ": Response from goproxy is:", resp)

		//orPanic(resp.Write(clientBuf.Writer))
		//orPanic(clientBuf.Flush())
	//}

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

//listen on incoming port
//accept incoming connection
// parses the request
// print the info.
func main() {
	log.Println("Starting goproxy...")
	go startGoProxy()
	log.Println("Done.")
	endpt := new(Endpoint)
	log.Println("Starting my proxy....")
	endpt.Listen(8080)
}

