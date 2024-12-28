package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
)

var servers = []string{"http://localhost:8080", "http://localhost:8081", "http://localhost:8082"}
var unhealthyServers = make(map[string]bool)
var unhealthyServersMutex sync.RWMutex
var numHealthyServers int32 = int32(len(servers))
var counter uint32

func getServerResponse(server string) (string, error) {
	resp, err := http.Get(server)
	if err != nil {
		return "", fmt.Errorf("error making GET request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("error reading response body: %w", err)
	}

	return string(body), nil
}

func getServer() string {
	for {
		unhealthyServersMutex.Lock()
		if numHealthyServers == 0 {
			unhealthyServersMutex.Unlock()
			log.Fatal("No servers available")
		}
		unhealthyServersMutex.Unlock()
		idx := atomic.AddUint32(&counter, 1)
		server := servers[int(idx)%len(servers)]
		unhealthyServersMutex.RLock()
		isUnhealthy := unhealthyServers[server]
		unhealthyServersMutex.RUnlock()
		if !isUnhealthy {
			return server
		}
	}
}

func runHealthCheck() {
	for {
		for _, server := range servers {
			resp, err := http.Get(server)
			if err != nil || resp.StatusCode != http.StatusOK {
				if resp != nil {
					resp.Body.Close()
				}
				if !unhealthyServers[server] {
					log.Printf("Server %s is unhealthy. Removing from list of available servers \n", server)
					unhealthyServersMutex.Lock()
					atomic.AddInt32(&numHealthyServers, -1)
					unhealthyServers[server] = true
					unhealthyServersMutex.Unlock()
				}
			} else {
				resp.Body.Close()
				if unhealthyServers[server] {
					log.Printf("Server %s is healthy. Adding to list of available servers \n", server)
					unhealthyServersMutex.Lock()
					atomic.AddInt32(&numHealthyServers, 1)
					unhealthyServers[server] = false
					unhealthyServersMutex.Unlock()
				}
			}
		}
	}
}

func main() {

	healthCheckTimeout := os.Args[1]
	timeout, err := strconv.Atoi(healthCheckTimeout)
	if err != nil {
		log.Fatalf("Invalid health check timeout: %s\n", healthCheckTimeout)
	}
	log.Printf("Health check timeout: %d\n", timeout)
	go runHealthCheck()

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Request received from IP: %s\n", r.RemoteAddr)
		server := getServer()
		unhealthyServersMutex.RLock()
		for unhealthyServers[server] {
			server = getServer()
		}
		unhealthyServersMutex.RUnlock()
		log.Printf("Forwarding request to %s\n", server)
		resp, err := getServerResponse(server)
		if err != nil {
			http.Error(w, "Error forwarding request", http.StatusInternalServerError)
			return
		}
		log.Printf("Response from server: %s\n", resp)
	})

	fs := http.FileServer(http.Dir("static/"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))
	log.Fatal(http.ListenAndServe(":80", nil))
}
