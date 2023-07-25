package main

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"time"
)

type Server interface {
	// Address returns the addr of server
	Address() string

	// isAlive checks if server is serving or not
	isAlive() bool

	// Serve processes the request
	Serve(res http.ResponseWriter, req *http.Request)
}

type serverNode struct {
	addr  string
	proxy *httputil.ReverseProxy // load balancer to server nodes redirector(proxy)
}

func (s *serverNode) Address() string { return s.addr }
func (s *serverNode) isAlive() bool   { return true }
func (s *serverNode) Serve(res http.ResponseWriter, req *http.Request) {
	s.proxy.ServeHTTP(res, req)
}

func serverAllocate(addr string) *serverNode {
	serveUrl, err := url.Parse(addr)
	handleError(err)

	return &serverNode{
		addr:  addr,
		proxy: httputil.NewSingleHostReverseProxy(serveUrl),
	}
}

type LoadBalancer struct {
	port       string
	roundRobin int
	servers    []Server
}

func LoadBalancerAllocate(port string, servers []Server) *LoadBalancer {
	return &LoadBalancer{
		port:       port,
		roundRobin: 0,
		servers:    servers,
	}
}

func handleError(err error) {
	if err != nil {
		fmt.Println("error: ", err)
		os.Exit(1)
	}
}

// getNextAvailableServer() returns the address of the next available server to send a
// request to, with round robin algorithm
func (lb *LoadBalancer) getNextAvailableServer() Server {
	server := lb.servers[lb.roundRobin%len(lb.servers)]
	for !server.isAlive() {
		lb.roundRobin++
		server = lb.servers[lb.roundRobin%len(lb.servers)]
	}
	lb.roundRobin++

	return server
}

func (lb *LoadBalancer) serveProxy(res http.ResponseWriter, req *http.Request) {
	targetServer := lb.getNextAvailableServer()
	time.Sleep(1000)
	fmt.Println("Forwarding request to server", targetServer.Address())
	targetServer.Serve(res, req)
}

func main() {
	servers := []Server{
		serverAllocate("http://127.0.0.1:8001"),
		// serverAllocate("https://127.0.0.1:8002"),
		// serverAllocate("https://google.com"),
		// serverAllocate("https://bing.com"),
		// serverAllocate("https://yahoo.com"),

	}
	lb := LoadBalancerAllocate("8000", servers)
	handleRequest := func(res http.ResponseWriter, req *http.Request) {
		lb.serveProxy(res, req)
	}

	http.HandleFunc("/", handleRequest)

	fmt.Println("Serving request at localhost:", lb.port)
	http.ListenAndServe(":"+lb.port, nil)
}
