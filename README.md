# Metis

Thesis statement: If we build a system that can defeat censorship by choosing intelligently between existing 
circumvention tools to route traffic, it will reduce the bandwidth required to avoid censorship, improve the latency 
experienced by users of these tools, and provide a means of collecting data on censorship across the globe. 

##To install:

```
go build -o client proxy.go  
go build -o svr server/server.go
```

Start the server before starting the client, or the client will throw an error.  
```
./svr  
./client 
``` 

##Set up your browser to use Metis as a proxy:  
**Chrome:** *Settings -> Advanced -> Open proxy settings (under System).*  
On Windows, click the box labeled "LAN settings."
Check the "Use a proxy..." box, and set your proxy address to 127.0.0.1 and the port to 8080.

**Firefox:** *Preferences -> Advanced.* Click "Settings" across from Connection. Select "Manual proxy configuration"
and set the HTTP Proxy box to 127.0.0.1 and the port to 8080.
