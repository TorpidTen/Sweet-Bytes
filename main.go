package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"

	"://github.com"
)

const (
	AuthToken       = "SecureSecretToken2026"
	AllowlistDomain = "https://httpbin.org" // The ONLY destination allowed for security
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type TunnelPayload struct {
	URL    string              `json:"url"`
	Method string              `json:"method"`
	Header map[string][]string `json:"header"`
}

type TunnelResult struct {
	Status    int                 `json:"status"`
	Body      []byte              `json:"body"`
	Header    map[string][]string `json:"header"`
	BytesUsed int64               `json:"bytes_used"`
}

var (
	activeClient *websocket.Conn
	clientMutex  sync.Mutex
	totalBytes   int64
	bytesMutex   sync.Mutex
)

// Endpoint 1: The local background App Daemon connects here to register its secure reverse tunnel
func handleClientRegistration(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token != AuthToken {
		http.Error(w, "Unauthorized token", http.StatusUnauthorized)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("WebSocket upgrade failed:", err)
		return
	}

	clientMutex.Lock()
	if activeClient != nil {
		activeClient.Close()
	}
	activeClient = conn
	clientMutex.Unlock()

	log.Println("[SERVER] Remote application client daemon connected successfully.")
}

// Endpoint 2: The Dashboard uses this API to route a test query safely through the tunnel
func handleProxyRequest(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	
	targetURL := r.URL.Query().Get("url")
	if !strings.HasPrefix(targetURL, AllowlistDomain) {
		http.Error(w, "Forbidden: Target URL is not in the safe allowlist", http.StatusForbidden)
		return
	}

	clientMutex.Lock()
	conn := activeClient
	clientMutex.Unlock()

	if conn == nil {
		http.Error(w, "No active app client daemon connected to the tunnel gateway", http.StatusServiceUnavailable)
		return
	}

	task := TunnelPayload{
		URL:    targetURL,
		Method: "GET",
		Header: r.Header,
	}

	taskBytes, _ := json.Marshal(task)
	err := conn.WriteMessage(websocket.TextMessage, taskBytes)
	if err != nil {
		http.Error(w, "Failed to relay request over the active tunnel", http.StatusInternalServerError)
		return
	}

	_, responseBytes, err := conn.ReadMessage()
	if err != nil {
		http.Error(w, "Failed to read response from the remote client", http.StatusInternalServerError)
		return
	}

	var result TunnelResult
	json.Unmarshal(responseBytes, &result)

	bytesMutex.Lock()
	totalBytes += result.BytesUsed
	bytesMutex.Unlock()

	for key, values := range result.Header {
		for _, val := range values {
			w.Header().Add(key, val)
		}
	}
	w.WriteHeader(result.Status)
	w.Write(result.Body)
}

// Endpoint 3: Reports real-time tracking stats back to the frontend panel
func handleStats(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/json")
	
	clientMutex.Lock()
	status := "Offline"
	if activeClient != nil {
		status = "Online"
	}
	clientMutex.Unlock()

	bytesMutex.Lock()
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":     status,
		"bytes_used": totalBytes,
	})
	bytesMutex.Unlock()
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	http.HandleFunc("/connect-app", handleClientRegistration)
	http.HandleFunc("/proxy", handleProxyRequest)
	http.HandleFunc("/api/stats", handleStats)

	// Serves your index.html visual UI cleanly from the /ui directory
	http.Handle("/", http.FileServer(http.Dir("./ui")))

	log.Printf("[SERVER] Operational on port :%s\n", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
