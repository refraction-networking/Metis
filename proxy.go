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
	"bytes"
	"net/http/httputil"
	"os"
)

type Endpoint struct {
	listener net.Listener
	mutex sync.RWMutex
}

type Website struct {
	Domain string `json:"domain,omitempty"`
}

var client = &http.Client{}

/*
Domains Metis is reasonably certain are censored are stored here.
 */
var blockedDomains []string

/*
Domains that Metis has trouble accessing for reasons that might not be censorship are stored here.
 */
var tempBlockedDomains []string

var detouredDomains []string
var directDomains []string
var failedDomains []string

func contains(slice []string, s string) bool {
	for _, e := range slice {
		if e == s { return true}
	}
	return false
}

func isBlocked(url *url.URL) (bool) {
	return contains(blockedDomains, url.Hostname()) || contains(tempBlockedDomains, url.Hostname())
}

func remove(s []string, e string) []string {
	for i, ele := range s {
		if ele==e {
			s = append(s[:i], s[i+1:]...)
		}
	}
	return s
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

func detectedTampering(id int, resp *http.Response, err error) bool {
	//TODO: do resets get caught correctly here?
	//TODO: how to catch TLS certificate errors?
	netErr, ok := err.(net.Error)
	if ok {
		//Timeout, RST?
		log.Println(id, "Website timed out with network error ", netErr)
		blockedDomains = append(blockedDomains, resp.Request.URL.Hostname())
		return true
	}
	_, ok = err.(*net.OpError)
	if ok {
		//Finds ECONNRESET and EPIPE?
		log.Println(id, "Website threw net.OpError ", err)
		blockedDomains = append(blockedDomains, resp.Request.URL.Hostname())
		return true
	}
	if err != nil {
		log.Println(id, "Website threw unknown error ", err)
		//Don't add to blockedDomains because error waqsn't due to censorship?
		orPanic(err)
	} else {
		//HTTP poisoning: Iran only, code taken from https://github.com/getlantern/detour/blob/master/detect.go
		byteResp, dmpErr := httputil.DumpResponse(resp, true)
		orPanic(dmpErr)
		http403 := []byte("HTTP/1.1 403 Forbidden")
		iranIFrame := []byte(`<iframe src="http://10.10.34.34`)
		if bytes.HasPrefix(byteResp, http403) && bytes.Contains(byteResp, iranIFrame) {
			blockedDomains = append(blockedDomains, resp.Request.URL.Hostname())
			return true
		}
		//TODO: Other tampering detection should go here
	}
	return false
}

func detectedFailedConn(err error) bool {
	//TODO: What errors will get thrown if a connection is censored? Distinguish them from non-censorship errs.
	//TODO: Should putting things in blocked lists go here?
	if err == nil {
		return false
	}
	return true
}

func doHttpRequest(clientConn net.Conn, req *http.Request, id int) {
	log.Println(id, ": Performing non-CONNECT HTTP request")
	defer clientConn.Close()
	//http.Request has a field RequestURI that should be replaced by URL, RequestURI cannot be set for client.Do.
	req.RequestURI = ""
	resp, err := client.Do(req)
	if detectedTampering(id, resp, err) {
		connectToResource(clientConn, req, id, true)
		return
	}
	//orPanic(err)
	logDomains("direct", req.URL.Hostname())
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
	if !routeToTd {
		remoteConn, err = net.Dial("tcp", req.URL.Hostname()+":"+req.URL.Port())
		if detectedFailedConn(err) {
			tempBlockedDomains = append(tempBlockedDomains, req.URL.Hostname())
			remoteConn, err = connectToTapdance(clientConn, req, id)
			if err != nil {
				tempBlockedDomains = remove(tempBlockedDomains, req.URL.Hostname())
				log.Println(id, ": Cannot connect to ", req.URL.Hostname(), ": ", err)
				logDomains("failed", req.URL.Hostname())
				/*TODO: Figure out what errors get thrown here by arbitrarily panicking,
				  and translate them into HTTP responses to write back to the client.
				*/
				orPanic(err)
				//clientConn.Write([]byte(""))
				//return
			} else {
				logDomains("detour", req.URL.Hostname())
			}
		} else {
			logDomains("direct", req.URL.Hostname())
		}
	} else {
		logDomains("detour", req.URL.Hostname())
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

func logDomains(logFile string, d string) {
	domainLog, err := os.OpenFile("log/"+logFile+".txt",os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	orPanic(err)
	defer domainLog.Close()
	n, err := domainLog.WriteString(d+"\r\n")
	log.Println(n, "bytes written to log/"+logFile+".txt. Error: ", err)
	domainLog.Sync()
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	os.Create("log/detour.txt")
	os.Create("log/direct.txt")
	os.Create("log/failed.txt")
	endpt := new(Endpoint)
	log.Println("Starting Metis proxy....")
	updateBlockedList()
	endpt.Listen(8080, handleConnection)
}

