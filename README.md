# Metis

Thesis statement: It is possible to increase the speed at which a proxy server, whose intention is to circumvent 
censorship, operates, by routing only those requests that would be censored through the proxy and allowing others to 
connect directly.

The process should look something like:
1) Receive a connection and bytes from a local browser (e.g. "GET" or "CONNECT" stuff)
2) Pass these bytes to some HTTP proxy library parser, that parses them and returns some HTTP request object
3) Host/destination is extracted from the request object, and we determine if the request needs a proxy or not
4) If it needs a proxy, pass the bytes to tapdance/whatever proxy system we're using. If it doesn't need a proxy,
 you pass the bytes to a local library that does the GET or CONNECT for you (aka, goproxy).

Metis goes here: browser -> Metis -> Tapdance client or local HTTP proxy

# TODO

This link might be useful: https://github.com/elazarl/goproxy

Notes on the Tapdance station:
the station runs in an ISP
you shouldn't have to worry too much about what it's doing
it terminates (is the other endpoint) of the HTTP proxy though
so normally, we have browser -> tapdance client
and then tapdance client -> tapdance station -> squid
and what the browser really sees, is that it's just talking to squid
(squid is an HTTP proxy)
Metis goes in between the browser and tapdance client, and decides, for each request, whether to use the tapdance client, or just fetch the request directly
if it's directly though, Metis COULD fetch it "itself" (implementing a local HTTP proxy, essentially), but likely there exists a go library that will do that for you like https://github.com/elazarl/goproxy

browser starts a connection to tapdance client (which starts a connection to tapdance station, (which starts a connection to squid))
then browser sends up that path the request
and receives back down the response
yeah, squid doesn't do any decoy routing (refraction networking)
the only things that do that are the tapdance client and tapdance station
you can think of it like, we provide transport of data between browser and squid
the browser doesn't know it's talking to tapdance, or what any of this stuff is
all it cares is: it connects to *something* that talks HTTP proxy
we encode and decode and transport that something, and ultimately it ends up at a squid instance
that squid instance doesn't know what connected to it (or anything about tapdance or decoy routing/refraction networking)
it just knows it gets a connection, and an HTTP proxy request
and then it fulfills that request, and sends a response
we take that response, encapsulate it back into the tapdance protocol, get it back down to the client, and then the client sends it back to the browser

but basically, the only things you'll see a browser produce is a `GET http://site.com/ HTTP/1.1` for HTTP requests, and a `CONNECT site.com:443 HTTP/1.1` for TLS
https://en.wikipedia.org/wiki/Proxy_server#Implementations_of_proxies


# Notes 10/2:

1) If I get a GET request, close clientConn when? While clientConn is open (while it doesn't throw an error), 
response = http.defaultTransport(request). 
Forward response to client. 
2) If I get a CONNECT request, it might be followed by an SSL handshake. Assuming the http parsing logic is right after
 accept(), stop parsing incoming msgs as HTTP right after you get a CONNECT and send the 200 OK. Switch to byte copying
 from then on, copy bytes from clientConn to remoteConn which you create using net.Dial.
3) Close CONNECT clientConn when?
4) accept() should return a socket sock. 
5) TODO: replace goproxy with sergey's DualStream function from forward_proxy.
6) Basically, the code I had at first is what should happen for GET requests. The code I have now should happen for CONNECTs.
Excpet that I should replace goproxy with DualStream.