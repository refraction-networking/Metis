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
	//"math/rand"
	"golang.org/x/net/proxy"
	"errors"
	"runtime"
	"strings"
	"time"
)

type Endpoint struct {
	listener net.Listener
	mutex sync.RWMutex
}

type Website struct {
	Domain string `json:"domain,omitempty"`
}

var client = &http.Client{
	Transport: &http.Transport{
		Dial: (&net.Dialer{
			//Limits the time spent establishing a TCP connection (if a new one is needed)
			//TODO: tweak this value. How? Time how long usual connections take. Valid to take avg over all domains?
			Timeout:   5 * time.Second,
			KeepAlive: 30 * time.Second,
		}).Dial,
		//limits the time spent performing the TLS handshake.
		TLSHandshakeTimeout:   5 * time.Second,
		//Limits time spent reading response headers. TODO: Possibly unnecessary?
		ResponseHeaderTimeout: 10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	},
}

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
		if strings.Contains(s, e) { return true}
	}
	return false
}

func isBlocked(url *url.URL) (bool) {
		return contains(blockedDomains, url.Hostname()) || contains(tempBlockedDomains, url.Hostname())
}

func remove(s []string, e string) []string {
	for i, ele := range s {
		if ele==e && i+1 < len(s){
			s = append(s[:i], s[i+1:]...)
		} else if ele == e {
			s = s[:i]
		}
	}
	return s
}

func updateBlockedList() (error){
	req, err := http.NewRequest("GET", "HTTP://localhost:9090/blocked", nil)
	if err != nil {
		log.Println(err)
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		log.Println(err)
		return err
	}
	defer resp.Body.Close()
	dec := json.NewDecoder(resp.Body)

	// read open bracket
	t, err := dec.Token()
	if err != nil {
		log.Println(err)
		return err
	}
	fmt.Printf("%T: %v\n", t, t)

	// while the array contains values
	for dec.More() {
		var site Website
		// decode an array value (Message)
		err := dec.Decode(&site)
		if err != nil {
			log.Println(err)
			return err
		}
		fmt.Printf("Domain: %v\n", site.Domain)
		if !contains(blockedDomains,site.Domain) {
			blockedDomains = append(blockedDomains, site.Domain)
		}
	}

	// read closing bracket
	t, err = dec.Token()
	if err != nil {
		log.Println(err)
		return err
	}
	fmt.Printf("%T: %v\n", t, t)
	return nil
}

func parseRequest(conn net.Conn)(*http.Request, error){
	connReader := bufio.NewReader(conn)
	req, err := http.ReadRequest(connReader)
	if err != nil {return nil, err}
	return req, nil
}

func detectedTampering(id int, req *http.Request, resp *http.Response, err error) (bool, error) {
	//TODO: do resets get caught correctly here?
	//TODO: how to catch TLS certificate errors?
	netErr, ok := err.(net.Error)
	if ok {
		//Timeout, RST?
		log.Println(id, "Website timed out with network error ", netErr)
		blockedDomains = append(blockedDomains, req.URL.Hostname())
		return true, nil
	}
	_, ok = err.(*net.OpError)
	if ok {
		//Finds ECONNRESET and EPIPE?
		log.Println(id, "Website threw net.OpError ", err)
		blockedDomains = append(blockedDomains, req.URL.Hostname())
		return true, nil
	}
	if err != nil {
		log.Println(id, "Website threw unknown error ", err)
		//Don't add to blockedDomains because error wasn't due to censorship?
		return false, err
	} else {
		//HTTP poisoning: Iran only, code taken from https://github.com/getlantern/detour/blob/master/detect.go
		byteResp, dmpErr := httputil.DumpResponse(resp, true)
		if dmpErr != nil {
			err = errors.New("response couldn't be dumped to byte slice")
			return false, err
		}
		http403 := []byte("HTTP/1.1 403 Forbidden")
		iranIFrame := []byte(`<iframe src="http://10.10.34.34`)
		if bytes.HasPrefix(byteResp, http403) && bytes.Contains(byteResp, iranIFrame) {
			blockedDomains = append(blockedDomains, req.URL.Hostname())
			return true, nil
		}
		//TODO: Other tampering detection should go here
	}
	return false, nil
}

func detectedFailedConn(err error) bool {
	//TODO: What errors will get thrown if a connection is censored? Distinguish them from non-censorship errs.
	//TODO: Should putting things in blocked lists go here?
	if err == nil {
		return false
	}
	return true
}

func doHttpRequest(clientConn net.Conn, req *http.Request, id int) error {
	defer clientConn.Close()
	log.Println(id, ": Performing non-CONNECT HTTP request to ", req.Host)
	//http.Request has a field RequestURI that should be replaced by URL, RequestURI cannot be set for client.Do.
	req.RequestURI = ""
	resp, err := client.Do(req)
	//Possible timeout for DNS lookup, DNS spoof, pretty much everything
	//Is this where google reqs fail in China?
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	tampered, tamperingErr := detectedTampering(id, req, resp, err)
	if tamperingErr != nil {
		log.Println("Error attempting to detect tampering: ", err)
		return err
	}
	if tampered {
		return connectToResource(clientConn, req, id, true)

	}
	logDomains("direct", req.URL.Hostname(), id)
	err = resp.Write(clientConn)
	return err
}


func connectToTapdance(clientConn net.Conn, req *http.Request, id int) (net.Conn, error){
	port := req.URL.Port()
	host := req.URL.Hostname()
	if port == "" && req.TLS == nil {
		port = "80"
	} else if port == "" {
		port = "443"
	}
	remoteConn, err := tapdance.Dial("tcp", host+":"+port)
	return remoteConn, err
}

func getMeekListeningPort() (string) {
	//TODO: Move to meek_adapter
	//TODO: Scrape meek's log file for last instance of "Listening on..."
	//TODO: Change log file name to global, pass in from proxy.go
	return "60984"
}

func connectToMeek(clientConn net.Conn, req *http.Request, id int) (net.Conn, error) {
	proxy_addr := "127.0.0.1:"+getMeekListeningPort()
	socksDialer, err := proxy.SOCKS5("tcp", proxy_addr, nil, proxy.Direct)
	if err != nil {
		fmt.Fprintln(os.Stderr, "can't connect to the proxy:", err)
		return nil, err
	}
	port := req.URL.Port()
	host := req.URL.Hostname()
	if port == "" && req.TLS == nil {
		port = "80"
	} else if port == "" {
		port = "443"
	}
	//Check args on this
	//Returns remoteConn, err
	return socksDialer.Dial("tcp", host+":"+port)
}

func transmitError(clientConn net.Conn, err error){
	defer clientConn.Close()
	_, ok := err.(net.Error)
	if ok {
		clientConn.Write([]byte("HTTP/1.1 504 Gateway Timeout\r\n\r\n"))
		return
	}
	_, ok = err.(*net.OpError)
	if ok {
		clientConn.Write([]byte("HTTP/1.1 504 Gateway Timeout\r\n\r\n"))
		return
	}
	if err != nil {
		//TODO: Tapdance sometimes responds with HTTP errors (503 Service Unavailable), how do I make sure an error is an HTTP response?
		clientConn.Write([]byte(err.Error()))
	}
}

func connectToResource(clientConn net.Conn, req *http.Request, id int, routeToTd bool) error {
	log.Println(id, ": CONNECTing to resource ", req.Host)
	var remoteConn net.Conn
	var err error
	errChan := make(chan error)

	if !routeToTd {
		remoteConn, err = net.DialTimeout("tcp", req.URL.Hostname()+":"+req.URL.Port(), 5*time.Second)
		if detectedFailedConn(err) || err!=nil{
			log.Println("Goroutine", id, "failed to CONNECT to resource directly with error", err)
			tempBlockedDomains = append(tempBlockedDomains, req.URL.Hostname())
			remoteConn, err = connectToTapdance(clientConn, req, id)
			if err != nil {
				tempBlockedDomains = remove(tempBlockedDomains, req.URL.Hostname())
				log.Println(id, ": Cannot connect to ", req.URL.Hostname(), ": ", err)
				logDomains("failed", req.URL.Hostname(), id)
				//TODO: Check this error handling.
				transmitError(clientConn, err)
				return err
			} else {
				logDomains("detour", req.URL.Hostname(), id)
			}
		} else {
			logDomains("direct", req.URL.Hostname(), id)
		}
	} else {
		logDomains("detour", req.URL.Hostname(), id)
		remoteConn, err = connectToTapdance(clientConn, req, id)
		if err != nil {
			//Try again
			//TODO: Should I be retrying to connect here like this?
			remoteConn, err = connectToTapdance(clientConn, req, id)
		}
		if err != nil {
			//Request probably isn't going through, it failed twice.
			tempBlockedDomains = remove(tempBlockedDomains, req.URL.Hostname())
			log.Println(id, ": Cannot connect to Tapdance after two tries: ", err)
			logDomains("failed", req.URL.Hostname(), id)
			transmitError(clientConn, err)
			return err
		}
	}

	defer func() {
		remoteConn.Close()
		clientConn.Close()
		_ = <- errChan
	}()

	if req.Method != "CONNECT" {
		requestDump, err := httputil.DumpRequestOut(req, req.Body != nil)
		fmt.Println(requestDump)
		if err != nil {
			fmt.Println(err)
			//TODO: return err here?
		}
		_, err = remoteConn.Write(requestDump)
		if err != nil {
			fmt.Println(err)
		}
		rBuf := make([]byte, 65536)
		_, err = io.CopyBuffer(clientConn, remoteConn, rBuf)
		if err != nil {
			log.Println("Error transmiting response from HTTP request through Tapdance to client: ", err)
			return err
		}
		return nil
	}

	clientConn.Write([]byte("HTTP/1.1 200 Connection established\r\n\r\n"))

	forwardFromClientToRemote := func() {
		cBuf := make([]byte, 65536)
		n, err := io.CopyBuffer(remoteConn, clientConn, cBuf)
		log.Println(id, ": Client request length: - ", n)
		errChan <- err
	}

	forwardFromRemoteToClient := func() {
		rBuf := make([]byte, 65536)
		//remoteConn never sends EOF?
		n, err := io.CopyBuffer(clientConn, remoteConn, rBuf)
		log.Println(id, ": Remote response length: - ", n)
		errChan <- err
	}

	go forwardFromClientToRemote()
	go forwardFromRemoteToClient()

	err = <-errChan
	if err != nil {
		log.Println(err)
	}
	return err
}

//Assumes clientConn is not nil. TODO: check assumption
func handleConnection(clientConn net.Conn, id int) {
	//Parse the request as HTTP
	req, err := parseRequest(clientConn)
	if err == io.EOF { return }
	if err != nil {
		log.Println ("Error parsing HTTP request: ", err)
		clientConn.Write([]byte("HTTP/1.1 400 Bad request\r\n\r\n"))
		return
	}
	method := req.Method
	reqUrl := req.URL

	//Check to see where request should be routed
	routeToTransport := isBlocked(reqUrl)
	log.Println("Goroutine", id, "is connecting to ", reqUrl)
	if !routeToTransport && method != "CONNECT" {
		err = doHttpRequest(clientConn, req, id)
		if err != nil {
			tempBlockedDomains = append(tempBlockedDomains, req.URL.Hostname())
			log.Println("Goroutine", id, "failed to connect directly with error", err)
			err = connectToResource(clientConn, req, id, true)
		}
	} else {
		err = connectToResource(clientConn, req, id, routeToTransport)
	}
	if err != nil {
		fmt.Println("Goroutine", id, "returned error", err)
	}
}

func (e *Endpoint) handleConnection(clientConn net.Conn, id int, handler func(net.Conn, int)) {
	defer log.Println("Goroutine", id, "is closed. Number of goroutines:", runtime.NumGoroutine())
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
		//Spins until a request comes in
		conn, err := e.listener.Accept()
		if err != nil {
			log.Println("Failed accepting a connection request:", err)
			continue
		}
		go e.handleConnection(conn, id, handler)
		id++
	}
}

func logDomains(logFile string, d string, id int) {
	domainLog, err := os.OpenFile("log/"+logFile+".txt",os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		log.Println("Couldn't open log file "+logFile+".txt")
		return
	}
	defer domainLog.Close()
	_, err = domainLog.WriteString(d+"\r\n")
	if err != nil {
		log.Println("Couldn't write to log file "+logFile+".txt")
		return
	}
	domainLog.Sync()
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	endpt := new(Endpoint)
	log.Println("Starting Metis proxy....")
	if updateBlockedList() != nil {
		log.Println("Error updating blocked list, starting with empty blocked list!")
	}
	endpt.Listen(8080, handleConnection)
}

